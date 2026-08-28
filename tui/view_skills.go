package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rookie-oops/skillsend/core"
)

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter.Blur()
		m.filterText = ""
		m.clampCursor()
		return m, nil
	case "enter":
		m.filtering = false
		m.filter.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filterText = m.filter.Value()
	m.clampCursor()
	return m, cmd
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n", "esc", "q":
		m.confirm = nil
		m.status = "cancelled"
		return m, nil
	case "y", "enter":
		req := m.confirm
		m.confirm = nil
		m.busy = true
		return m, req.run(&m)
	}
	return m, nil
}

func (m Model) updateCommitInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.commitActive = false
		m.commitInput.Blur()
		return m, nil
	case "enter":
		msgText := strings.TrimSpace(m.commitInput.Value())
		m.commitActive = false
		m.commitInput.Blur()
		if msgText == "" {
			m.status = "commit cancelled (empty message)"
			return m, nil
		}
		hub, targets := m.hub, m.targets
		m.busy = true
		return m, func() tea.Msg {
			did, err := core.CommitAll(hub, msgText)
			if err != nil {
				return opResultMsg{err: err}
			}
			status := "nothing to commit"
			if did {
				status = "committed: " + msgText
			}
			st, serr := core.Scan(hub, targets)
			return opResultMsg{status: status, err: serr, state: st}
		}
	}
	var cmd tea.Cmd
	m.commitInput, cmd = m.commitInput.Update(msg)
	return m, cmd
}

func (m Model) viewSkills() string {
	listW := m.width * 45 / 100
	if listW < 30 {
		listW = 30
	}
	list := m.renderSkillList(listW)
	detail := m.renderSkillDetail(m.width - listW - 4)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", detail)
}

func (m *Model) renderSkillList(width int) string {
	skills := m.filtered()
	var b strings.Builder
	b.WriteString(colHeaderStyle.Render("SKILL") + strings.Repeat(" ", 4))
	for i, t := range m.targets {
		name := filepath.Base(filepath.Dir(t))
		if m.focusTarget == i {
			b.WriteString(selectedStyle.Render("[" + name + "]") + " ")
		} else {
			b.WriteString(colHeaderStyle.Render(" " + name + " ") + " ")
		}
	}
	b.WriteString("\n")

	for i, sk := range skills {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = selectedStyle.Render("❯ ")
		}
		line := fmt.Sprintf("%s%-24s", cursor, truncate(sk.Name, 24))
		for _, t := range m.targets {
			if sk.Enabled[t] {
				line += onStyle.Render("●") + "  "
			} else {
				line += offStyle.Render("○") + "  "
			}
		}
		if len(sk.Dirty) > 0 {
			line += dirtyStyle.Render("⚠")
		}
		if sk.Upstream.Source != "" {
			line += " " + behindStyle.Render("↑")
		}
		b.WriteString(style.Render(line) + "\n")
	}
	if m.filtering {
		b.WriteString("\n" + m.filter.View() + "\n")
	} else if m.filterText != "" {
		b.WriteString("\n" + footerStyle.Render("filter: "+m.filterText+" (esc clears)") + "\n")
	}
	return focusRingStyle.Render(b.String())
}

func (m *Model) renderSkillDetail(width int) string {
	skills := m.filtered()
	if len(skills) == 0 {
		return detailKeyStyle.Render("no skills match")
	}
	sk := skills[m.cursor]
	var b strings.Builder
	kv := func(k, v string) {
		b.WriteString(detailKeyStyle.Render(fmt.Sprintf("%-12s", k)) + v + "\n")
	}
	b.WriteString(selectedStyle.Render(sk.Name) + "\n\n")
	kv("path", sk.Rel)
	if sk.Description != "" {
		kv("desc", truncate(sk.Description, maxInt(width-14, 20)))
	}
	if sk.Upstream.Source != "" {
		kv("upstream", sk.Upstream.Source)
		if sk.Upstream.Synced != "" {
			kv("synced", head7T(sk.Upstream.Synced))
		}
	} else {
		kv("upstream", "— (own skill)")
	}
	if len(sk.Dirty) > 0 {
		kv("dirty", dirtyStyle.Render(fmt.Sprintf("%d file(s) uncommitted", len(sk.Dirty))))
	}
	b.WriteString("\n")
	for i, t := range m.targets {
		state := offStyle.Render("○ disabled")
		if sk.Enabled[t] {
			state = onStyle.Render("● enabled")
		}
		marker := " "
		if m.focusTarget == i {
			marker = selectedStyle.Render("❯")
		}
		b.WriteString(fmt.Sprintf("%s %s %s\n", marker, filepath.Base(filepath.Dir(t)), state))
	}
	b.WriteString("\n" + footerStyle.Render("space: toggle focused target · h/l: move focus · o: open editor"))
	if sk.Upstream.Source != "" {
		b.WriteString(" · e: edit upstream")
	}
	b.WriteString(" · u: update all")
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func head7T(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
