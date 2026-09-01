package tui

import (
	"fmt"
	"strings"

	"github.com/BubbleBubbleIce/skillsend/core"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateHub(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "u":
		hub, targets := m.hub, m.targets
		m.busy = true
		return m, func() tea.Msg {
			err := core.PullFF(hub)
			st, serr := core.Scan(hub, targets)
			return opResultMsg{status: "hub pull finished", err: firstErr(err, serr), state: st}
		}
	case "p":
		hub, targets := m.hub, m.targets
		m.busy = true
		return m, func() tea.Msg {
			err := core.Push(hub)
			st, serr := core.Scan(hub, targets)
			return opResultMsg{status: "hub push finished", err: firstErr(err, serr), state: st}
		}
	case "c":
		if len(m.state.DirtyAll) == 0 {
			m.status = "nothing to commit"
			return m, nil
		}
		m.commitActive = true
		m.commitInput.SetValue("")
		m.commitInput.Focus()
		return m, textinput.Blink
	case "f":
		hub, targets := m.hub, m.targets
		m.busy = true
		m.checking = true
		return m, func() tea.Msg {
			manifest, err := core.LoadManifest(hub)
			if err != nil {
				return opResultMsg{err: err}
			}
			stale := core.CheckStaleness(hub, manifest)
			st, serr := core.Scan(hub, targets)
			msg := opResultMsg{status: "staleness check finished", err: firstErr(serr, nil), state: st}
			msg.staleness = stale
			return msg
		}
	}
	return m, nil
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func (m Model) viewHub() string {
	var b strings.Builder
	b.WriteString(colHeaderStyle.Render("HUB") + " " + m.hub + "\n")
	if !m.state.RepoOK {
		b.WriteString("\n" + errStyle.Render("hub is not a git repository — update/push/commit unavailable") + "\n")
	}
	b.WriteString("\n" + colHeaderStyle.Render("Uncommitted changes") + fmt.Sprintf(" (%d)\n", len(m.state.DirtyAll)))
	for i, p := range m.state.DirtyAll {
		if i > 40 {
			b.WriteString(footerStyle.Render(fmt.Sprintf("… and %d more", len(m.state.DirtyAll)-i)) + "\n")
			break
		}
		b.WriteString("  " + p + "\n")
	}
	if len(m.state.DirtyAll) == 0 {
		b.WriteString("  " + statusStyle.Render("clean") + "\n")
	}

	if len(m.staleness) > 0 {
		b.WriteString("\n" + colHeaderStyle.Render("Upstream staleness (f)") + "\n")
		for _, st := range m.staleness {
			line := "  " + st.Name + ": "
			switch {
			case st.Err != nil:
				line += errStyle.Render("error: " + st.Err.Error())
			case st.Diverged && !st.UpToDate:
				line += conflictStyle.Render(fmt.Sprintf("behind %d, local changes — update will skip", st.Behind))
			case st.UpToDate:
				line += statusStyle.Render("up to date")
			default:
				line += behindStyle.Render(fmt.Sprintf("behind %d commit(s) — press u", maxInt(st.Behind, 0)))
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n" + footerStyle.Render("u: pull · p: push · c: commit all (message) · f: staleness check"))
	if m.commitActive {
		b.WriteString("\n\n" + m.commitInput.View() + "\n")
	}
	return lipgloss.NewStyle().PaddingLeft(1).Render(b.String())
}
