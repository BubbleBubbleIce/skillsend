package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	gitCmd(t, dir, "config", "user.name", "test")
}

func commitFile(t *testing.T, dir, rel, content, msg string) string {
	t.Helper()
	writeFile(t, filepath.Join(dir, rel), content)
	gitCmd(t, dir, "add", rel)
	gitCmd(t, dir, "commit", "-m", msg)
	return strings.TrimSpace(gitCmd(t, dir, "rev-parse", "HEAD"))
}

func readSkillFile(t *testing.T, hub, rel, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(hub, rel, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCommitAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "r")
	initRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "1")
	did, err := CommitAll(dir, "first")
	if err != nil || !did {
		t.Fatalf("commit: did=%v err=%v", did, err)
	}
	did, err = CommitAll(dir, "second")
	if err != nil || did {
		t.Fatalf("clean commit should be a no-op: did=%v err=%v", did, err)
	}
}

func TestPullFF(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "origin")
	initRepo(t, origin)
	commitFile(t, origin, "f.txt", "v1", "c1")

	work := filepath.Join(t.TempDir(), "work")
	gitCmd(t, t.TempDir(), "clone", origin, work)
	gitCmd(t, work, "config", "user.email", "t@e.com")
	gitCmd(t, work, "config", "user.name", "t")

	commitFile(t, origin, "f.txt", "v2", "c2")
	if err := PullFF(work); err != nil {
		t.Fatalf("ff pull: %v", err)
	}
	if got := readSkillFile(t, filepath.Dir(work), filepath.Base(work), "f.txt"); got != "v2" {
		t.Fatalf("got %q", got)
	}

	// local commit makes it non-fast-forwardable
	commitFile(t, work, "local.txt", "x", "local")
	commitFile(t, origin, "other.txt", "y", "c3")
	if err := PullFF(work); err == nil {
		t.Fatal("expected divergence error")
	}
}

// fixture builds: upstream repo + hub repo with skill "grilling" copied from upstream.
func fixture(t *testing.T) (upstream, hub string, m *Manifest) {
	base := t.TempDir()
	upstream = filepath.Join(base, "upstream")
	hub = filepath.Join(base, "hub")
	initRepo(t, upstream)
	sha := commitFile(t, upstream, "grilling/SKILL.md", "---\nname: grilling\n---\nv1", "s1")
	commitFile(t, upstream, "grilling/extra.txt", "e", "s1b")

	initRepo(t, hub)
	writeFile(t, filepath.Join(hub, "grilling/SKILL.md"), "---\nname: grilling\n---\nv1")
	writeFile(t, filepath.Join(hub, "grilling/extra.txt"), "e")
	gitCmd(t, hub, "add", "-A")
	gitCmd(t, hub, "commit", "-m", "adopt grilling")

	sig, err := DirSignature(filepath.Join(hub, "grilling"))
	if err != nil {
		t.Fatal(err)
	}
	m = &Manifest{Skills: map[string]SkillMeta{}}
	// upstream is a collection repo: the skill lives in its "grilling/" subdirectory
	m.Set("grilling", SkillMeta{Source: upstream, Path: "grilling", Synced: sha, Tree: sig})
	CacheRootOverride = filepath.Join(base, "cache")
	t.Cleanup(func() { CacheRootOverride = "" })
	return upstream, hub, m
}

func TestUpdateUpstreamSkill(t *testing.T) {
	upstream, hub, m := fixture(t)
	old := m.Skills["grilling"]

	// upstream advances: modify, add, remove
	commitFile(t, upstream, "grilling/SKILL.md", "---\nname: grilling\n---\nv2", "s2")
	commitFile(t, upstream, "grilling/new.txt", "n", "s3")
	gitCmd(t, upstream, "rm", "grilling/extra.txt")
	gitCmd(t, upstream, "commit", "-m", "s4")

	meta, err := UpdateUpstreamSkill(hub, "grilling", old)
	if err != nil {
		t.Fatal(err)
	}
	if got := readSkillFile(t, hub, "grilling", "SKILL.md"); !strings.Contains(got, "v2") {
		t.Fatalf("content not updated: %q", got)
	}
	if _, err := os.Stat(filepath.Join(hub, "grilling", "extra.txt")); !os.IsNotExist(err) {
		t.Fatal("removed upstream file still present")
	}
	if _, err := os.Stat(filepath.Join(hub, "grilling", "new.txt")); err != nil {
		t.Fatal("new upstream file missing")
	}
	if meta.Synced == old.Synced || meta.Tree == old.Tree {
		t.Fatal("expected new synced sha and tree signature")
	}

	// up-to-date: second update is a no-op
	again, err := UpdateUpstreamSkill(hub, "grilling", meta)
	if err != nil {
		t.Fatal(err)
	}
	if again.Synced != meta.Synced {
		t.Fatalf("up-to-date update changed sha: %q -> %q", meta.Synced, again.Synced)
	}
}

func TestUpdateUpstreamSkillRefusesDivergence(t *testing.T) {
	_, hub, m := fixture(t)
	base := filepath.Dir(hub)

	// re-point manifest at a fresh upstream whose content matches the hub copy
	up2 := filepath.Join(base, "up2")
	initRepo(t, up2)
	sha := commitFile(t, up2, "grilling/SKILL.md", "---\nv1", "a")
	sig, err := DirSignature(filepath.Join(hub, "grilling"))
	if err != nil {
		t.Fatal(err)
	}
	// make local content match up2 exactly, then record baseline
	writeFile(t, filepath.Join(hub, "grilling/SKILL.md"), "---\nv1")
	gitCmd(t, hub, "add", "-A")
	gitCmd(t, hub, "commit", "-m", "match upstream")
	sig, err = DirSignature(filepath.Join(hub, "grilling"))
	if err != nil {
		t.Fatal(err)
	}
	m.Skills["grilling"] = SkillMeta{Source: up2, Synced: sha, Tree: sig}

	// committed local change
	commitFile(t, hub, "grilling/SKILL.md", "---\nlocal edit", "my change")
	if _, err := UpdateUpstreamSkill(hub, "grilling", m.Skills["grilling"]); !errors.Is(err, ErrDiverged) {
		t.Fatalf("want ErrDiverged (committed), got %v", err)
	}
	// upstream advances too; still refused
	commitFile(t, up2, "grilling/SKILL.md", "---\nv2", "b")
	if _, err := UpdateUpstreamSkill(hub, "grilling", m.Skills["grilling"]); !errors.Is(err, ErrDiverged) {
		t.Fatalf("want ErrDiverged (committed), got %v", err)
	}

	// uncommitted local change is also refused
	writeFile(t, filepath.Join(hub, "grilling", "SKILL.md"), "---\nuncommitted")
	if _, err := UpdateUpstreamSkill(hub, "grilling", m.Skills["grilling"]); !errors.Is(err, ErrDiverged) {
		t.Fatalf("want ErrDiverged (uncommitted), got %v", err)
	}
}

func TestUpdateUpstreamSkillNoBaseline(t *testing.T) {
	hub := filepath.Join(t.TempDir(), "hub")
	initRepo(t, hub)
	writeFile(t, filepath.Join(hub, "s/SKILL.md"), "x")
	if _, err := UpdateUpstreamSkill(hub, "s", SkillMeta{Source: "https://example.com/x.git"}); !errors.Is(err, ErrNoBaseline) {
		t.Fatalf("want ErrNoBaseline, got %v", err)
	}
}

func TestUpdateUpstreamSkillEstablishesBaseline(t *testing.T) {
	base := t.TempDir()
	up2 := filepath.Join(base, "up2")
	hub := filepath.Join(base, "hub")
	initRepo(t, up2)
	commitFile(t, up2, "grilling/SKILL.md", "---\nname: grilling\n---\nv1", "a")

	initRepo(t, hub)
	writeFile(t, filepath.Join(hub, "grilling/SKILL.md"), "---\nname: grilling\n---\nv1")
	gitCmd(t, hub, "add", "-A")
	gitCmd(t, hub, "commit", "-m", "same content")
	CacheRootOverride = filepath.Join(base, "cache")
	t.Cleanup(func() { CacheRootOverride = "" })

	// `e`-style upstream record: Tree known, Synced unknown
	sig, err := DirSignature(filepath.Join(hub, "grilling"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := UpdateUpstreamSkill(hub, "grilling", SkillMeta{Source: up2, Path: "grilling", Tree: sig})
	if err != nil {
		t.Fatalf("baseline establishment failed: %v", err)
	}
	if meta.Synced == "" {
		t.Fatal("expected synced sha to be established")
	}

	// matching content means up-to-date afterwards
	again, err := UpdateUpstreamSkill(hub, "grilling", meta)
	if err != nil || again.Synced != meta.Synced {
		t.Fatalf("post-baseline update: %+v %v", again, err)
	}

	// differing content is refused and baseline stays unknown
	writeFile(t, filepath.Join(hub, "grilling", "SKILL.md"), "---\nname: grilling\n---\nLOCAL EDIT")
	_, err = UpdateUpstreamSkill(hub, "grilling", SkillMeta{Source: up2, Path: "grilling", Tree: sig})
	if !errors.Is(err, ErrDiverged) {
		t.Fatalf("want ErrDiverged, got %v", err)
	}
}

func TestCheckStaleness(t *testing.T) {
	_, hub, m := fixture(t)
	sts := CheckStaleness(hub, m)
	if len(sts) != 1 {
		t.Fatalf("want 1 staleness, got %d", len(sts))
	}
	st := sts[0]
	if st.Err != nil {
		t.Fatalf("staleness error: %v", st.Err)
	}
	if st.UpToDate || st.Diverged {
		t.Fatalf("expected clean+behind, got %+v", st)
	}
	if st.Behind != 1 {
		t.Fatalf("behind = %d, want 1", st.Behind)
	}

	// local divergence blocks the update but staleness still reports
	commitFile(t, hub, "grilling/SKILL.md", "---\nlocal", "edit")
	sts = CheckStaleness(hub, m)
	if !sts[0].Diverged {
		t.Fatalf("expected diverged flag, got %+v", sts[0])
	}
}

func TestPorcelainPaths(t *testing.T) {
	lines := []string{
		" M modified.txt",
		"?? untracked.txt",
		"R  old -> new",
		`R  "old name" -> "new name"`,
		` M "tab\there.txt"`,
		` M "caf\303\251.txt"`,
		"",
	}
	got := porcelainPaths(lines)
	want := []string{"modified.txt", "untracked.txt", "new", "new name", "tab\there.txt", "café.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestUnquoteGitPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain.txt", "plain.txt"}, // unquoted passthrough
		{`"space name.txt"`, "space name.txt"},
		{`"tab\there"`, "tab\there"},
		{`"new\nline"`, "new\nline"},
		{`"quote\"mark"`, `quote"mark`},
		{`"caf\303\251"`, "café"},
		{`"back\\slash"`, `back\slash`},
	}
	for _, c := range cases {
		if got := unquoteGitPath(c.in); got != c.want {
			t.Errorf("unquoteGitPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
