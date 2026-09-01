package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BubbleBubbleIce/skillsend/core"
	"github.com/BubbleBubbleIce/skillsend/tui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Stamped in by GoReleaser at release time; "dev" for local builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("skillsend %s (commit %s, built %s)\n", version, commit, date)
		return
	}
	hub, err := resolveHub()
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillsend:", err)
		os.Exit(1)
	}
	state, err := core.Scan(hub, DefaultTargets())
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillsend: scan:", err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(tui.New(state, DefaultTargets()), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "skillsend:", err)
		os.Exit(1)
	}
}

// resolveHub returns the configured hub path, running a first-run setup screen
// when the config is missing or points nowhere.
func resolveHub() (string, error) {
	cfg, found, err := LoadConfig()
	if err != nil {
		return "", err
	}
	if found {
		if fi, err := os.Stat(cfg.Hub); err == nil && fi.IsDir() {
			return cfg.Hub, nil
		}
		fmt.Fprintln(os.Stderr, "skillsend: configured hub not found:", cfg.Hub)
	}
	hub, err := runSetup()
	if err != nil {
		return "", err
	}
	cfg.Hub = hub
	if err := SaveConfig(cfg); err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "skillsend: hub saved to config:", hub)
	return hub, nil
}

// runSetup asks for the hub directory once (ADR-0004: hub path is the single
// configurable value, chosen per machine).
func runSetup() (string, error) {
	ti := textinput.New()
	ti.Placeholder = "path to your skills repo (the hub)"
	ti.Prompt = "❯ "
	if home, err := os.UserHomeDir(); err == nil {
		def := filepath.Join(home, "my_dev", "skills")
		if fi, err := os.Stat(def); err == nil && fi.IsDir() {
			ti.SetValue(def)
		}
	}
	ti.Focus()

	m := setupModel{input: ti}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	result, ok := final.(setupModel)
	if !ok || result.hub == "" {
		return "", fmt.Errorf("setup cancelled")
	}
	return result.hub, nil
}

type setupModel struct {
	input textinput.Model
	hub   string
	err   string
}

func (m setupModel) Init() tea.Cmd { return textinput.Blink }

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			p := m.input.Value()
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				abs, err := filepath.Abs(p)
				if err != nil {
					m.err = err.Error()
					return m, nil
				}
				m.hub = abs
				return m, tea.Quit
			}
			m.err = "not a directory: " + p
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m setupModel) View() string {
	out := "⚔ SKILLSEND — first run\n\n"
	out += "Where do your skills live? This folder (a git repo) becomes the\n"
	out += "hub: the single source of truth that gets linked into\n"
	out += "  " + fmt.Sprint(DefaultTargets()) + "\n\n"
	out += m.input.View() + "\n"
	if m.err != "" {
		out += errSetup(m.err)
	}
	out += "\nenter: confirm · ctrl+c: cancel"
	return out
}

func errSetup(s string) string { return "✗ " + s + "\n" }
