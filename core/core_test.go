package core

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseFrontmatter(t *testing.T) {
	data := "---\nname: grilling\ndescription:  Stress-test a plan with relentless questioning.\nother: x\n---\n\n# body\n"
	name, desc := ParseFrontmatter(data)
	if name != "grilling" {
		t.Errorf("name = %q, want grilling", name)
	}
	if desc != "Stress-test a plan with relentless questioning." {
		t.Errorf("description = %q", desc)
	}
}

func TestParseFrontmatterQuotedAndMissing(t *testing.T) {
	name, desc := ParseFrontmatter("---\nname: \"tdd\"\n---\nbody")
	if name != "tdd" || desc != "" {
		t.Errorf("got %q/%q", name, desc)
	}
	if n, d := ParseFrontmatter("no frontmatter"); n != "" || d != "" {
		t.Errorf("expected empty, got %q/%q", n, d)
	}
}

func TestManifestRoundtrip(t *testing.T) {
	hub := t.TempDir()
	m, err := LoadManifest(hub)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get("grilling"); ok {
		t.Fatal("expected empty manifest")
	}
	m.Set("grilling", SkillMeta{Source: "https://github.com/mattpocock/skills.git", Ref: "main", Synced: "abc123"})
	if err := m.Save(hub); err != nil {
		t.Fatal(err)
	}
	m2, err := LoadManifest(hub)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := m2.Get("grilling")
	if !ok || got.Source != "https://github.com/mattpocock/skills.git" || got.Ref != "main" || got.Synced != "abc123" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	m2.Remove("grilling")
	if _, ok := m2.Get("grilling"); ok {
		t.Fatal("expected removal")
	}
}
