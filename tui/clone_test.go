package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHubCloneInputImportsRepository(t *testing.T) {
	hub, _, m := tuiEnv(t)
	remote := filepath.Join(filepath.Dir(hub), "new-skill")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remote, "SKILL.md"), []byte("# new skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"add", "-A"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = remote
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	m, _ = key(t, m, "3")
	m, _ = key(t, m, "g")
	if !m.cloneActive || !strings.Contains(m.View(), "git clone") {
		t.Fatal("g should open the clone URL input on the Hub page")
	}
	m.cloneInput.SetValue(remote)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || !m.busy || m.cloneActive {
		t.Fatal("enter should close the input and start an asynchronous clone")
	}
	msg := cmd()
	result, ok := msg.(opResultMsg)
	if !ok || result.err != nil {
		t.Fatalf("clone failed: %+v", msg)
	}
	m = apply(t, m, msg)
	if _, ok := m.state.SkillByName("new-skill"); !ok {
		t.Fatal("cloned skill was not added to refreshed state")
	}
	if !strings.Contains(m.status, "cloned: new-skill") {
		t.Fatalf("unexpected status: %q", m.status)
	}
}
