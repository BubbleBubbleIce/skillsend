package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BubbleBubbleIce/skillsend/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// stalenessByName indexes the last staleness check by skill name.
func (m *Model) stalenessByName() map[string]core.Staleness {
	out := map[string]core.Staleness{}
	for _, st := range m.staleness {
		out[st.Name] = st
	}
	return out
}

func (m *Model) renderSkillList(width int) string {
	skills := m.filtered()
	if len(skills) > 0 {
		if m.cursor >= len(skills) {
			m.cursor = len(skills) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
	stale := m.stalenessByName()

	// fixed header: "SKILL" + one column per target
	var head strings.Builder
	head.WriteString(colHeaderStyle.Render("SKILL") + strings.Repeat(" ", 4))
	for i, t := range m.targets {
		name := filepath.Base(filepath.Dir(t))
		if m.focusTarget == i {
			head.WriteString(selectedStyle.Render("["+name+"]") + " ")
		} else {
			head.WriteString(colHeaderStyle.Render(" "+name+" ") + " ")
		}
	}

	// one line per skill
	var lines []string
	for i, sk := range skills {
		cursor := "  "
		if i == m.cursor {
			cursor = selectedStyle.Render("❯ ")
		}
		line := fmt.Sprintf("%s%-24s", cursor, truncate(sk.Label, 24))
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
		if st, ok := stale[sk.Name]; ok && !st.Diverged && st.Behind > 0 {
			line += " " + behindStyle.Render(fmt.Sprintf("↑%d", st.Behind))
		} else if sk.Upstream.Source != "" {
			line += " " + colHeaderStyle.Render("↑")
		}
		lines = append(lines, line)
	}

	// Body is clipped to h-6; the header is fixed, the rest scrolls with the
	// cursor pinned to the bottom edge of the viewport.
	scrollH := m.height - 7
	if scrollH < 1 {
		scrollH = 1
	}
	start := 0
	if len(skills) > 0 && m.cursor >= scrollH {
		start = m.cursor - scrollH + 1
	}
	end := start + scrollH
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	b.WriteString(head.String() + "\n")
	for _, l := range lines[start:end] {
		b.WriteString(l + "\n")
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
	b.WriteString(selectedStyle.Render(sk.Label) + "\n\n")
	kv("path", sk.Rel)
	if sk.Label != sk.Name {
		kv("dir", sk.Name)
	}
	if sk.Description != "" {
		kv("desc", truncate(sk.Description, maxInt(width-14, 20)))
	}
	if sk.Upstream.Source != "" {
		kv("upstream", sk.Upstream.Source)
		if sk.Upstream.Synced != "" {
			kv("synced", core.ShortSha(sk.Upstream.Synced))
		}
		if st, ok := m.stalenessByName()[sk.Name]; ok {
			switch {
			case st.Err != nil:
				kv("staleness", errStyle.Render("check failed"))
			case st.Diverged:
				kv("staleness", conflictStyle.Render("local changes — u will skip"))
			case st.UpToDate:
				kv("staleness", statusStyle.Render("up to date"))
			default:
				kv("staleness", behindStyle.Render(fmt.Sprintf("behind %d — press u", st.Behind)))
			}
		} else {
			kv("staleness", footerStyle.Render("press f in Hub view"))
		}
	} else {
		kv("upstream", "— (own skill)")
	}
	if len(sk.Dirty) > 0 {
		kv("dirty", dirtyStyle.Render(fmt.Sprintf("%d file(s) uncommitted", len(sk.Dirty))))
	}

	// SKILL.md body preview
	if body := m.previewBody(sk, 12); body != "" {
		b.WriteString("\n" + colHeaderStyle.Render("SKILL.md") + "\n")
		for _, l := range strings.Split(body, "\n") {
			b.WriteString("  " + footerStyle.Render(truncate(l, maxInt(width-6, 20))) + "\n")
		}
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

// previewBody returns the first n lines of the skill's SKILL.md body.
func (m *Model) previewBody(sk core.Skill, n int) string {
	_, _, body := core.ReadSkillMD(filepath.Join(m.hub, filepath.FromSlash(sk.Rel)))
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
