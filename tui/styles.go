// Package tui renders Skillsend's three-view management panel:
// Skills (browse & toggle), Targets (reverse audit), Hub (git sync).
package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	tabStyle       = lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EE6FF8")).Padding(0, 2).Underline(true)
	hubPathStyle   = lipgloss.NewStyle().Faint(true)
	colHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#626262"))
	selectedStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EE6FF8"))
	detailKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00A67D"))
	errStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF5F87"))
	footerStyle    = lipgloss.NewStyle().Faint(true)
	focusRingStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(lipgloss.Color("#7D56F4")).PaddingLeft(1)

	// state glyphs
	onStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D787")) // enabled
	offStyle      = lipgloss.NewStyle().Faint(true)                                      // disabled
	foreignStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86B"))            // foreign
	brokenStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF5F87")) // broken
	conflictStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86B"))
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86B"))
	behindStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FD7FF"))
)
