package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func makeCloneRemote(t *testing.T, base, name string, files map[string]string) string {
	t.Helper()
	repo := filepath.Join(base, name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"add", "-A"},
		{"commit", "-m", "initial"},
	} {
		if _, err := runGit(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestCloneIntoHubImportsRootSkillAndProvenance(t *testing.T) {
	base := t.TempDir()
	hub := filepath.Join(base, "hub")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	remote := makeCloneRemote(t, base, "root-skill", map[string]string{
		"SKILL.md": "---\nname: root-skill\n---\n",
	})

	names, err := CloneIntoHub(hub, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "root-skill" {
		t.Fatalf("skills = %v, want [root-skill]", names)
	}
	if _, err := os.Stat(filepath.Join(hub, "root-skill", "SKILL.md")); err != nil {
		t.Fatalf("skill was not imported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hub, "root-skill", ".git")); !os.IsNotExist(err) {
		t.Fatalf("nested .git must be stripped, got %v", err)
	}
	m, err := LoadManifest(hub)
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := m.Get("root-skill")
	if !ok || meta.Source != remote || meta.Path != "" || meta.Synced == "" || meta.Tree == "" {
		t.Fatalf("unexpected provenance: %+v, present=%v", meta, ok)
	}

	oldCache := CacheRootOverride
	CacheRootOverride = filepath.Join(base, "cache")
	defer func() { CacheRootOverride = oldCache }()
	if err := os.WriteFile(filepath.Join(remote, "SKILL.md"), []byte("# updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "update"}} {
		if _, err := runGit(remote, args...); err != nil {
			t.Fatal(err)
		}
	}
	updated, err := UpdateUpstreamSkill(hub, "root-skill", meta)
	if err != nil {
		t.Fatalf("update after clone import: %v", err)
	}
	if updated.Synced == meta.Synced {
		t.Fatal("upstream update did not advance the synced commit")
	}
	content, err := os.ReadFile(filepath.Join(hub, "root-skill", "SKILL.md"))
	if err != nil || string(content) != "# updated\n" {
		t.Fatalf("updated content = %q, err=%v", content, err)
	}
}

func TestCloneIntoHubImportsCollectionSkills(t *testing.T) {
	base := t.TempDir()
	hub := filepath.Join(base, "hub")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	remote := makeCloneRemote(t, base, "collection", map[string]string{
		"alpha/SKILL.md": "# alpha\n",
		"beta/SKILL.md":  "# beta\n",
		"README.md":      "collection\n",
	})

	names, err := CloneIntoHub(hub, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("skills = %v, want [alpha beta]", names)
	}
	m, err := LoadManifest(hub)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		meta, ok := m.Get(name)
		if !ok || meta.Path != name {
			t.Fatalf("%s provenance = %+v, present=%v", name, meta, ok)
		}
	}
}

func TestCloneIntoHubRejectsUnsupportedOrConflictingRepo(t *testing.T) {
	base := t.TempDir()
	hub := filepath.Join(base, "hub")
	if err := os.MkdirAll(filepath.Join(hub, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	unsupported := makeCloneRemote(t, base, "not-a-skill", map[string]string{"README.md": "no skill\n"})
	if _, err := CloneIntoHub(hub, unsupported); err == nil {
		t.Fatal("repository without a supported SKILL.md should fail")
	}
	conflict := makeCloneRemote(t, base, "taken", map[string]string{"SKILL.md": "# taken\n"})
	if _, err := CloneIntoHub(hub, conflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}
