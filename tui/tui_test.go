package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rookie-oops/skillsend/core"
)

func tuiEnv(t *testing.T) (hub string, targets []string, m Model) {
	base := t.TempDir()
	hub = filepath.Join(base, "skills")
	if err := os.MkdirAll(hub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeT := func(rel, content string) {
		p := filepath.Join(hub, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeT("grilling/SKILL.md", "---\nname: grilling\ndescription: Stress-test plans.\n---\n# grilling")
	writeT("tdd/SKILL.md", "---\nname: tdd\ndescription: Red green refactor.\n---\n# tdd")

	agents := filepath.Join(base, "agents-skills")
	claude := filepath.Join(base, "claude-skills")
	for _, d := range []string{agents, claude} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	targets = []string{agents, claude}

	st, err := core.Scan(hub, targets)
	if err != nil {
		t.Fatal(err)
	}
	m = New(st, targets)
	m.width, m.height = 100, 30
	return hub, targets, m
}

func key(t *testing.T, m Model, s string) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return next.(Model), cmd
}

func apply(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestViewRendersAllTabs(t *testing.T) {
	hub, targets, m := tuiEnv(t)

	v := m.View()
	for _, want := range []string{"SKILLSEND", "Skills", "Targets", "Hub", "grilling", "tdd", "Stress-test plans."} {
		if !strings.Contains(v, want) {
			t.Errorf("skills view missing %q", want)
		}
	}
	_ = hub
	_ = targets
}

func TestTabSwitchAndViews(t *testing.T) {
	_, _, m := tuiEnv(t)
	m, _ = key(t, m, "2")
	if v := m.View(); !strings.Contains(v, "TARGET ENTRIES") {
		t.Error("targets view not rendered")
	}
	m, _ = key(t, m, "3")
	if v := m.View(); !strings.Contains(v, "HUB") {
		t.Error("hub view not rendered")
	}
	m, _ = key(t, m, "1")
	if v := m.View(); !strings.Contains(v, "SKILL") {
		t.Error("skills view not rendered back")
	}
}

func TestToggleThroughUpdateCreatesLink(t *testing.T) {
	hub, targets, m := tuiEnv(t)
	agents, claude := targets[0], targets[1]

	// cursor on first skill (grilling), focus first target (agents): space → enable
	m2, cmd := key(t, m, " ")
	if cmd == nil {
		t.Fatal("expected toggle cmd")
	}
	msg := cmd()
	res, ok := msg.(opResultMsg)
	if !ok || res.err != nil {
		t.Fatalf("toggle failed: %+v", msg)
	}
	m2 = apply(t, m2, msg)
	if _, err := os.Lstat(filepath.Join(agents, "grilling")); err != nil {
		t.Fatalf("link not created in agents: %v", err)
	}
	if !m2.state.Skills[0].Enabled[agents] {
		t.Fatal("state not refreshed as enabled")
	}

	// focus second target (l), space → enable in claude too
	m2, cmd = key(t, m2, "l")
	m2, cmd = key(t, m2, " ")
	if cmd == nil {
		t.Fatal("expected toggle cmd")
	}
	msg2 := cmd()
	if res, ok := msg2.(opResultMsg); !ok || res.err != nil {
		t.Fatalf("second toggle failed: %+v", msg2)
	}
	m2 = apply(t, m2, msg2)
	if _, err := os.Lstat(filepath.Join(claude, "grilling")); err != nil {
		t.Fatalf("link not created in claude: %v", err)
	}

	// space again on claude (still focused) → disable
	m2, cmd = key(t, m2, " ")
	msg3 := cmd()
	if res, ok := msg3.(opResultMsg); !ok || res.err != nil {
		t.Fatalf("disable failed: %+v", msg3)
	}
	m2 = apply(t, m2, msg3)
	if _, err := os.Lstat(filepath.Join(claude, "grilling")); !os.IsNotExist(err) {
		t.Fatal("claude link not removed")
	}
	if _, err := os.Lstat(filepath.Join(agents, "grilling")); err != nil {
		t.Fatal("agents link must remain")
	}
	_ = hub
}

func TestFilterNarrowsList(t *testing.T) {
	_, _, m := tuiEnv(t)
	m, _ = key(t, m, "/")
	m.filter.SetValue("tdd")
	m.filterText = "tdd"
	m = apply(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	v := m.View()
	if strings.Contains(v, "Stress-test plans.") {
		t.Error("grilling should be filtered out")
	}
	if !strings.Contains(v, "Red green refactor.") {
		t.Error("tdd should remain visible")
	}
}

func TestConfirmModalGuardsRemoval(t *testing.T) {
	_, targets, m := tuiEnv(t)
	agents := targets[0]
	// foreign link in agents
	if err := os.Symlink("/", filepath.Join(agents, "elsewhere")); err != nil {
		t.Fatal(err)
	}
	st, err := core.Scan(m.hub, targets)
	if err != nil {
		t.Fatal(err)
	}
	m = New(st, targets)
	m.width, m.height = 100, 30
	m, _ = key(t, m, "2")

	// cursor to the foreign entry: entries sorted by name → "elsewhere" first
	m, cmd := key(t, m, "x")
	if m.confirm == nil {
		t.Fatal("expected confirmation modal")
	}
	if cmd != nil {
		t.Fatal("no action before confirmation")
	}
	// n cancels
	m, _ = key(t, m, "n")
	if m.confirm != nil {
		t.Fatal("modal should be dismissed")
	}
	if _, err := os.Lstat(filepath.Join(agents, "elsewhere")); err != nil {
		t.Fatal("foreign link must survive cancel")
	}
	// x then y removes
	m, _ = key(t, m, "x")
	m2, cmd := key(t, m, "y")
	if cmd == nil {
		t.Fatal("expected removal cmd after y")
	}
	msg := cmd()
	if res, ok := msg.(opResultMsg); !ok || res.err != nil {
		t.Fatalf("removal failed: %+v", msg)
	}
	m2 = apply(t, m2, msg)
	if _, err := os.Lstat(filepath.Join(agents, "elsewhere")); !os.IsNotExist(err) {
		t.Fatal("foreign link not removed after confirm")
	}
	_ = m2
}
