package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// env builds a hub with a top-level skill, a nested skill, and three targets
// populated exactly like the user's real machine: direct links, a chained
// link (claude → agents → hub), foreign links/dirs, and a broken link.
func env(t *testing.T) (hub string, targets []string) {
	base := t.TempDir()
	hub = filepath.Join(base, "skills")
	writeFile(t, filepath.Join(hub, "grilling/SKILL.md"), "---\nname: grilling\ndescription: grill\n---\nbody")
	writeFile(t, filepath.Join(hub, "tdd/SKILL.md"), "---\nname: tdd\ndescription: tdd\n---\nbody")
	writeFile(t, filepath.Join(hub, "ericadskill/draft/SKILL.md"), "---\nname: draft\ndescription: draft intro\n---\nbody")

	agents := filepath.Join(base, "agents-skills")
	claude := filepath.Join(base, "claude-skills")
	other := filepath.Join(base, "other-skills")
	for _, d := range []string{agents, claude, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// direct link in agents
	mustSymlink(t, filepath.Join(hub, "grilling"), filepath.Join(agents, "grilling"))
	// chained link: claude → agents → hub
	mustSymlink(t, filepath.Join(agents, "grilling"), filepath.Join(claude, "grilling"))
	mustSymlink(t, filepath.Join(hub, "ericadskill/draft"), filepath.Join(agents, "draft"))
	// nested skill linked by leaf name
	mustSymlink(t, filepath.Join(hub, "tdd"), filepath.Join(claude, "tdd"))
	// foreign link pointing elsewhere
	mustSymlink(t, base, filepath.Join(agents, "somewhere-else"))
	// broken link
	mustSymlink(t, filepath.Join(base, "gone"), filepath.Join(claude, "dangling"))
	// foreign real dir colliding with a hub skill name
	if err := os.MkdirAll(filepath.Join(other, "grilling"), 0o755); err != nil {
		t.Fatal(err)
	}

	return canonicalize(hub), []string{agents, claude, other}
}

func mustSymlink(t *testing.T, to, from string) {
	t.Helper()
	if err := os.Symlink(to, from); err != nil {
		t.Fatal(err)
	}
}

func TestScan(t *testing.T) {
	hub, targets := env(t)
	st, err := Scan(hub, targets)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Skills) != 3 {
		t.Fatalf("want 3 skills, got %d: %+v", len(st.Skills), st.Skills)
	}

	byName := map[string]Skill{}
	for _, sk := range st.Skills {
		byName[sk.Name] = sk
	}
	if byName["draft"].Rel != "ericadskill/draft" {
		t.Fatalf("nested rel = %q", byName["draft"].Rel)
	}
	if byName["grilling"].Description != "grill" {
		t.Fatalf("description = %q", byName["grilling"].Description)
	}

	// chained link (claude → agents → hub) must be recognized as enabled
	agents, claude, other := targets[0], targets[1], targets[2]
	if !byName["grilling"].Enabled[claude] {
		t.Error("chained link not recognized as enabled in claude target")
	}
	if !byName["grilling"].Enabled[agents] {
		t.Error("direct link not recognized as enabled in agents target")
	}
	if !byName["draft"].Enabled[agents] || byName["draft"].Enabled[claude] {
		t.Error("draft enabled map wrong")
	}
	if !byName["tdd"].Enabled[claude] || byName["tdd"].Enabled[agents] {
		t.Error("tdd enabled map wrong (linked into claude only)")
	}

	// target entry classification
	var claudeTarget, agentsTarget, otherTarget *Target
	for i := range st.Targets {
		switch st.Targets[i].Path {
		case claude:
			claudeTarget = &st.Targets[i]
		case agents:
			agentsTarget = &st.Targets[i]
		case other:
			otherTarget = &st.Targets[i]
		}
	}
	get := func(tg *Target, name string) TargetEntry {
		for _, e := range tg.Entries {
			if e.Name == name {
				return e
			}
		}
		t.Fatalf("entry %q not found", name)
		return TargetEntry{}
	}
	if e := get(claudeTarget, "grilling"); e.Kind != KindHubLink || e.HubSkill != "grilling" {
		t.Errorf("claude grilling = %+v", e)
	}
	if e := get(claudeTarget, "dangling"); e.Kind != KindBroken {
		t.Errorf("dangling = %+v", e)
	}
	if e := get(agentsTarget, "somewhere-else"); e.Kind != KindForeignLink {
		t.Errorf("somewhere-else = %+v", e)
	}
	if e := get(otherTarget, "grilling"); e.Kind != KindForeignDir || !e.Conflicts {
		t.Errorf("conflicting foreign dir = %+v", e)
	}
}

func TestScanMissingTarget(t *testing.T) {
	hub, _ := env(t)
	st, err := Scan(hub, []string{filepath.Join(t.TempDir(), "nope")})
	if err != nil {
		t.Fatal(err)
	}
	if !st.Targets[0].Missing || len(st.Targets[0].Entries) != 0 {
		t.Fatalf("missing target mishandled: %+v", st.Targets[0])
	}
}

func TestEnableDisable(t *testing.T) {
	hub, targets := env(t)
	agents, claude := targets[0], targets[1]

	// enable tdd into claude
	if err := Enable(hub, claude, "tdd"); err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(filepath.Join(claude, "tdd"))
	if resolved != filepath.Join(hub, "tdd") {
		t.Fatalf("link points to %q", resolved)
	}
	// idempotent
	if err := Enable(hub, claude, "tdd"); err != nil {
		t.Fatalf("idempotent enable: %v", err)
	}

	// disable the chained grilling link in claude: removes only that segment
	if err := Disable(hub, claude, "grilling"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(claude, "grilling")); !os.IsNotExist(err) {
		t.Fatal("claude link still present")
	}
	if _, err := os.Lstat(filepath.Join(agents, "grilling")); err != nil {
		t.Fatal("agents link must be untouched")
	}

	// disable refuses foreign links
	if err := Disable(hub, agents, "somewhere-else"); !errors.Is(err, ErrOutsideHub) {
		t.Fatalf("want ErrOutsideHub, got %v", err)
	}
	// disable refuses real dirs
	if err := Disable(hub, targets[2], "grilling"); !errors.Is(err, ErrNotLink) {
		t.Fatalf("want ErrNotLink, got %v", err)
	}
	// enable refuses occupied names (real dir)
	if err := Enable(hub, targets[2], "grilling"); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	// enable over a foreign link is also a conflict
	if err := Enable(hub, agents, "somewhere-else"); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict for foreign link, got %v", err)
	}
}

func TestRemoveLink(t *testing.T) {
	_, targets := env(t)
	agents, claude := targets[0], targets[1]

	if err := RemoveLink(agents, "somewhere-else"); err != nil {
		t.Fatalf("remove foreign link: %v", err)
	}
	if err := RemoveLink(claude, "dangling"); err != nil {
		t.Fatalf("remove broken link: %v", err)
	}
	if err := RemoveLink(targets[2], "grilling"); !errors.Is(err, ErrNotLink) {
		t.Fatalf("real dir must be refused, got %v", err)
	}
}

func TestAdopt(t *testing.T) {
	base := t.TempDir()
	hub := filepath.Join(base, "skills")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	hub = canonicalize(hub)
	m, err := LoadManifest(hub)
	if err != nil {
		t.Fatal(err)
	}

	// a foreign skill that is its own git repo with an origin remote
	originURL := "https://github.com/example/skills.git"
	src := filepath.Join(base, "agents-skills", "myskill")
	writeFile(t, filepath.Join(src, "SKILL.md"), "---\nname: myskill\n---\nhi")
	gitCmd(t, src, "init", "-b", "main")
	gitCmd(t, src, "config", "user.email", "t@e.com")
	gitCmd(t, src, "config", "user.name", "t")
	gitCmd(t, src, "config", "remote.origin.url", originURL)
	gitCmd(t, src, "add", "-A")
	gitCmd(t, src, "commit", "-m", "init")
	head := strings.TrimSpace(gitCmd(t, src, "rev-parse", "HEAD"))

	meta, err := Adopt(src, hub, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) && meta.Source == "" {
		t.Fatal("source dir should have been moved away (or replaced by link)")
	}
	// original location replaced by a link into the hub
	fi, err := os.Lstat(src)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("origin should now be a symlink: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(src)
	if resolved != filepath.Join(hub, "myskill") {
		t.Fatalf("link resolves to %q", resolved)
	}
	// provenance recorded
	if meta.Source != originURL || meta.Synced != head || meta.Path != "" {
		t.Fatalf("meta = %+v", meta)
	}
	got, ok := m.Get("myskill")
	if !ok || got.Source != originURL {
		t.Fatalf("manifest = %+v", m.Skills)
	}

	// collision refused
	dupSkill := filepath.Join(base, "dup", "myskill")
	writeFile(t, filepath.Join(dupSkill, "SKILL.md"), "dup")
	if _, err := Adopt(dupSkill, hub, m); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	// adopting a symlink refused
	if _, err := Adopt(src, hub, m); !errors.Is(err, ErrNotLink) {
		t.Fatalf("symlink adopt should fail with ErrNotLink, got %v", err)
	}
}

func TestAdoptFromCollectionRepo(t *testing.T) {
	base := t.TempDir()
	hub := filepath.Join(base, "skills")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	hub = canonicalize(hub)
	m, err := LoadManifest(hub)
	if err != nil {
		t.Fatal(err)
	}

	// a collection repo with the skill nested one level down
	coll := filepath.Join(base, "collection")
	originURL := "git@github.com:example/collection.git"
	writeFile(t, filepath.Join(coll, "skills/myskill/SKILL.md"), "---\nname: myskill\n---\nhi")
	gitCmd(t, coll, "init", "-b", "main")
	gitCmd(t, coll, "config", "user.email", "t@e.com")
	gitCmd(t, coll, "config", "user.name", "t")
	gitCmd(t, coll, "config", "remote.origin.url", originURL)
	gitCmd(t, coll, "add", "-A")
	gitCmd(t, coll, "commit", "-m", "init")
	head := strings.TrimSpace(gitCmd(t, coll, "rev-parse", "HEAD"))

	src := filepath.Join(coll, "skills", "myskill")
	meta, err := Adopt(src, hub, m)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Source != originURL || meta.Path != "skills/myskill" || meta.Synced != head {
		t.Fatalf("meta = %+v", meta)
	}
	if fi, err := os.Lstat(filepath.Join(coll, "skills", "myskill")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("original location should now be a symlink: %v", err)
	}
}

func TestScanPreservesTargetPaths(t *testing.T) {
	hub, targets := env(t)
	orig := append([]string(nil), targets...)

	st, err := Scan(hub, targets)
	if err != nil {
		t.Fatal(err)
	}
	// Scan must not mutate its argument, and Target.Path / Enabled keys must
	// stay in the caller's path space (the TUI holds those same original paths,
	// so a symlinked /var → /private/var mismatch would silently break toggles).
	for i := range targets {
		if targets[i] != orig[i] {
			t.Fatalf("Scan mutated its argument: %q -> %q", orig[i], targets[i])
		}
		if st.Targets[i].Path != orig[i] {
			t.Fatalf("Target.Path = %q, want %q (original caller path)", st.Targets[i].Path, orig[i])
		}
	}

	byName := map[string]Skill{}
	for _, sk := range st.Skills {
		byName[sk.Name] = sk
	}
	if !byName["grilling"].Enabled[orig[1]] {
		t.Fatal("grilling not enabled in claude target when looked up by the original path")
	}
}
