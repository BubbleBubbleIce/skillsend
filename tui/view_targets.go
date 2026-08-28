package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rookie-oops/skillsend/core"
)

// flatEntry is one row of the targets view: a target directory entry with its
// owning target index.
type flatEntry struct {
	targetIdx int
	target    string
	entry     core.TargetEntry
}

func (m *Model) flatEntries() []flatEntry {
	var out []flatEntry
	for ti, t := range m.state.Targets {
		for _, e := range t.Entries {
			out = append(out, flatEntry{targetIdx: ti, target: t.Path, entry: e})
		}
	}
	return out
}

func (m *Model) clampTargetCursor() {
	n := len(m.flatEntries())
	if n == 0 {
		m.tcursor = 0
		return
	}
	if m.tcursor >= n {
		m.tcursor = n - 1
	}
	if m.tcursor < 0 {
		m.tcursor = 0
	}
}

func (m Model) updateTargets(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := m.flatEntries()
	switch msg.String() {
	case "j", "down":
		m.tcursor++
		m.clampTargetCursor()
	case "k", "up":
		m.tcursor--
		m.clampTargetCursor()
	case " ":
		if len(entries) == 0 {
			return m, nil
		}
		fe := entries[m.tcursor]
		if fe.entry.Kind != core.KindHubLink {
			return m, nil
		}
		hub, targets := m.hub, m.targets
		leaf := fe.entry.Name
		target := fe.target
		m.busy = true
		return m, func() tea.Msg {
			err := core.Disable(hub, target, leaf)
			if err != nil {
				return opResultMsg{err: err}
			}
			st, serr := core.Scan(hub, targets)
			return opResultMsg{status: "disabled " + leaf + " in " + filepath.Base(target), err: serr, state: st}
		}
	case "a": // adopt foreign dir
		if len(entries) == 0 {
			return m, nil
		}
		fe := entries[m.tcursor]
		if fe.entry.Kind != core.KindForeignDir {
			return m, nil
		}
		name := fe.entry.Name
		hub, targets := m.hub, m.targets
		m.confirm = &confirmRequest{
			title: fmt.Sprintf("Adopt %q into hub? (moves the directory, links it back in place)", name),
			run: func(mm *Model) tea.Cmd {
				return func() tea.Msg {
					manifest, err := core.LoadManifest(hub)
					if err != nil {
						return opResultMsg{err: err}
					}
					meta, err := core.Adopt(filepath.Join(fe.target, name), hub, manifest)
					if err != nil {
						return opResultMsg{err: err}
					}
					status := "adopted " + name
					if meta.Source != "" {
						status += " (upstream recorded)"
					}
					st, serr := core.Scan(hub, targets)
					return opResultMsg{status: status, err: serr, state: st}
				}
			},
		}
		return m, nil
	case "x": // remove foreign/broken link
		if len(entries) == 0 {
			return m, nil
		}
		fe := entries[m.tcursor]
		if fe.entry.Kind != core.KindForeignLink && fe.entry.Kind != core.KindBroken {
			return m, nil
		}
		leaf, target := fe.entry.Name, fe.target
		hub, targets := m.hub, m.targets
		m.confirm = &confirmRequest{
			title: fmt.Sprintf("Remove link %q from %s? (the link only — never a directory)", leaf, filepath.Base(target)),
			run: func(mm *Model) tea.Cmd {
				return func() tea.Msg {
					err := core.RemoveLink(target, leaf)
					if err != nil {
						return opResultMsg{err: err}
					}
					st, serr := core.Scan(hub, targets)
					return opResultMsg{status: "removed link " + leaf, err: serr, state: st}
				}
			},
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewTargets() string {
	listW := m.width * 55 / 100
	if listW < 34 {
		listW = 34
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderTargetList(listW), "  ", m.renderTargetDetail(m.width-listW-4))
}

func (m *Model) renderTargetList(width int) string {
	entries := m.flatEntries()
	var b strings.Builder
	b.WriteString(colHeaderStyle.Render("TARGET ENTRIES") + "\n")
	lastTarget := ""
	for i, fe := range entries {
		targetName := filepath.Base(filepath.Dir(fe.target))
		if fe.target != lastTarget {
			b.WriteString("\n" + colHeaderStyle.Render("── "+targetName+" "+footerStyle.Render("("+filepath.Base(fe.target)+")")) + "\n")
			lastTarget = fe.target
		}
		cursor := "  "
		if i == m.tcursor {
			cursor = selectedStyle.Render("❯ ")
		}
		glyph, styled := glyphFor(fe.entry.Kind)
		name := fmt.Sprintf("%s%s %-28s", cursor, glyph, truncate(fe.entry.Name, 28))
		if fe.entry.Conflicts {
			name += " " + conflictStyle.Render("⚠ conflict")
		}
		_ = styled
		b.WriteString(name + "\n")
	}
	if len(entries) == 0 {
		b.WriteString(footerStyle.Render("(empty)") + "\n")
	}
	return focusRingStyle.Render(b.String())
}

func glyphFor(k core.EntryKind) (string, interface{}) {
	switch k {
	case core.KindHubLink:
		return onStyle.Render("●"), nil
	case core.KindForeignLink:
		return foreignStyle.Render("◌"), nil
	case core.KindBroken:
		return brokenStyle.Render("✕"), nil
	case core.KindForeignDir:
		return foreignStyle.Render("▣"), nil
	}
	return "?", nil
}

func (m *Model) renderTargetDetail(width int) string {
	entries := m.flatEntries()
	if len(entries) == 0 {
		return detailKeyStyle.Render("no entries")
	}
	fe := entries[m.tcursor]
	var b strings.Builder
	kv := func(k, v string) {
		b.WriteString(detailKeyStyle.Render(fmt.Sprintf("%-10s", k)) + v + "\n")
	}
	b.WriteString(selectedStyle.Render(fe.entry.Name) + "\n\n")
	kv("kind", fe.entry.Kind.String())
	kv("target", fe.target)
	if fe.entry.Kind == core.KindHubLink {
		kv("skill", fe.entry.HubSkill)
		kv("resolves", fe.entry.Resolved)
		kv("action", onStyle.Render("space: disable"))
	} else if fe.entry.Kind == core.KindForeignLink {
		kv("resolves", fe.entry.Resolved)
		kv("action", foreignStyle.Render("x: remove link (confirm)"))
	} else if fe.entry.Kind == core.KindBroken {
		kv("action", brokenStyle.Render("x: remove dead link (confirm)"))
	} else if fe.entry.Kind == core.KindForeignDir {
		if fe.entry.Conflicts {
			kv("note", conflictStyle.Render("name collides with a hub skill"))
		}
		kv("action", foreignStyle.Render("a: adopt into hub (confirm)"))
	}
	b.WriteString("\n" + footerStyle.Render("a: adopt · x: remove link · space: disable hub link"))
	return b.String()
}
