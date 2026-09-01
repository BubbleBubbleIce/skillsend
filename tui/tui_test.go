package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/BubbleBubbleIce/skillsend/core"
	tea "github.com/charmbracelet/bubbletea"
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
	_, _, m := tuiEnv(t)

	v := m.View()
	for _, want := range []string{"SKILLSEND", "Skills", "Targets", "Hub", "grilling", "tdd", "Stress-test plans."} {
		if !strings.Contains(v, want) {
			t.Errorf("skills view missing %q", want)
		}
	}
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
	_, targets, m := tuiEnv(t)
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
}

func TestTargetsListScrollsToCursor(t *testing.T) {
	_, targets, m := tuiEnv(t)
	agents := targets[0]
	// 40 foreign dirs so the target list overflows the terminal height.
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("s%02d", i)
		if err := os.MkdirAll(filepath.Join(agents, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := core.Scan(m.hub, targets)
	if err != nil {
		t.Fatal(err)
	}
	m = New(st, targets)
	m.width, m.height = 100, 12 // listH=6, scrollH=5
	m, _ = key(t, m, "2")       // switch to Targets

	// move the cursor to the bottom of the list
	for i := 0; i < 100; i++ {
		m, _ = key(t, m, "j")
	}
	v := m.View()
	if !strings.Contains(v, "s39") {
		t.Fatalf("last entry should be visible; view:\n%s", v)
	}
	if strings.Contains(v, "s00") {
		t.Fatalf("first entry should have scrolled out of view; view:\n%s", v)
	}
}

func TestSkillsListScrollsToCursor(t *testing.T) {
	hub, targets, m := tuiEnv(t)
	// 40 extra skills so the skills list overflows the terminal height.
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("s%02d", i)
		p := filepath.Join(hub, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := core.Scan(hub, targets)
	if err != nil {
		t.Fatal(err)
	}
	m = New(st, targets)
	m.width, m.height = 100, 12 // scrollH=5

	// skills sort by name: grilling, s00..s39, tdd → cursor lands on tdd
	for i := 0; i < 100; i++ {
		m, _ = key(t, m, "j")
	}
	v := m.View()
	if !strings.Contains(v, "tdd") {
		t.Fatalf("last skill should be visible; view:\n%s", v)
	}
	if strings.Contains(v, "grilling") {
		t.Fatalf("first skill should have scrolled out of view; view:\n%s", v)
	}
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

func TestFlavorCycleRepaints(t *testing.T) {
	_, _, m := tuiEnv(t)
	start := Flavor()
	defer SetFlavor(start)

	seen := map[string]bool{}
	for i := 0; i < len(flavorOrder)+1; i++ {
		name := Flavor()
		v := m.View()
		if !strings.Contains(v, "SKILLSEND") {
			t.Fatalf("%s: view lost the title", name)
		}
		// every flavor must render distinct accent colors, otherwise the
		// cycle silently paints nothing
		seen[fmt.Sprint(selectedStyle.GetForeground())] = true
		if i < len(flavorOrder) {
			if got := CycleFlavor(); got == name {
				t.Fatalf("cycle did not advance past %q", name)
			}
		}
	}
	if len(seen) != len(flavorOrder) {
		t.Errorf("expected %d distinct accent colors across flavors, got %d", len(flavorOrder), len(seen))
	}
	if Flavor() != start {
		t.Errorf("cycle did not wrap back to %q, got %q", start, Flavor())
	}
}

func TestSetFlavorIgnoresUnknownNames(t *testing.T) {
	_, _, m := tuiEnv(t)
	start := Flavor()
	defer SetFlavor(start)

	SetFlavor("not-a-flavor")
	if Flavor() != start {
		t.Errorf("unknown flavor changed the palette to %q", Flavor())
	}
	if v := m.View(); !strings.Contains(v, "SKILLSEND") {
		t.Error("view broken after rejected flavor")
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
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	_, _, m := tuiEnv(t)
	m, _ = key(t, m, "?")
	if v := m.View(); !strings.Contains(v, "HELP") {
		t.Fatal("help overlay should render")
	}
	// any key closes the overlay without executing that key's action
	m, _ = key(t, m, "j")
	if v := m.View(); strings.Contains(v, "HELP") {
		t.Fatal("help overlay should close on any key")
	}
}

func TestTruncateKeepsUTF8(t *testing.T) {
	got := truncate("资料库技能", 3)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate split a rune, produced invalid UTF-8: %q", got)
	}
	if got != "资料…" {
		t.Fatalf("truncate = %q, want %q", got, "资料…")
	}
	// no truncation needed: unchanged
	if truncate("grilling", 24) != "grilling" {
		t.Fatal("short string must pass through untouched")
	}
}

func TestHubViewUnknownBehind(t *testing.T) {
	_, _, m := tuiEnv(t)
	m.staleness = []core.Staleness{{Name: "grilling", Behind: -1}}
	v := m.viewHub()
	if !strings.Contains(v, "behind unknown") {
		t.Fatalf("unknown behind should render as 'behind unknown', got:\n%s", v)
	}
	if strings.Contains(v, "behind 0") {
		t.Fatalf("unknown behind must not render as 'behind 0':\n%s", v)
	}
}
