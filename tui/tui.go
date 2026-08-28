package tui

import (
	"strings"
)

// View renders the whole panel: header (title + tabs + hub path), the active
// view, and the footer (status / error / busy indicator), plus any modal.
func (m Model) View() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚔ SKILLSEND"))
	b.WriteString("  ")
	tabs := []string{"1 Skills", "2 Targets", "3 Hub"}
	for i, t := range tabs {
		if i == m.tab {
			b.WriteString(activeTabStyle.Render(t))
		} else {
			b.WriteString(tabStyle.Render(t))
		}
	}
	b.WriteString("  " + hubPathStyle.Render(m.hub))
	if m.busy {
		b.WriteString("  " + behindStyle.Render("…"))
	}
	b.WriteString("\n\n")

	var body string
	switch m.tab {
	case tabSkills:
		body = m.viewSkills()
	case tabTargets:
		body = m.viewTargets()
	case tabHub:
		body = m.viewHub()
	}
	b.WriteString(clipLines(body, h-6))

	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString(errStyle.Render("✗ " + truncate(m.errMsg, maxInt(w-4, 10))))
	} else {
		b.WriteString(statusStyle.Render("● " + truncate(m.status, maxInt(w-4, 10))))
	}
	b.WriteString("\n" + footerStyle.Render("? help · q quit"))

	out := b.String()
	if m.confirm != nil {
		out += "\n\n" + conflictStyle.Render("⚠ "+m.confirm.title) + "\n" +
			selectedStyle.Render("  y: yes") + "  " + footerStyle.Render("n/esc: no")
	}
	return out
}

// clipLines cuts a multi-line block to at most n lines and width w.
func clipLines(block string, n int) string {
	if n < 1 {
		return ""
	}
	lines := strings.Split(block, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for i, l := range lines {
		lines[i] = truncate(l, 240)
	}
	return strings.Join(lines, "\n")
}
