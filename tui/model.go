package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BubbleBubbleIce/skillsend/core"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// tab indices
const (
	tabSkills = iota
	tabTargets
	tabHub
)

// opResultMsg carries the outcome of a core operation plus a fresh scan.
type opResultMsg struct {
	status    string
	err       error
	state     *core.State
	staleness []core.Staleness
}

type statusMsg string

// Model is the root bubbletea model.
type Model struct {
	hub     string
	targets []string
	state   *core.State
	width   int
	height  int

	tab int

	// skills view
	cursor      int
	focusTarget int // which target column space toggles
	filtering   bool
	filter      textinput.Model
	filterText  string

	// targets view
	tcursor int

	// hub view
	staleness    []core.Staleness
	commitInput  textinput.Model
	commitActive bool

	// upstream edit modal
	upstreamInput  textinput.Model
	upstreamActive bool

	// confirmation modal
	confirm      *confirmRequest
	confirmRight int // right pane cursor in y/n

	status string
	errMsg string
	busy   bool
}

type confirmRequest struct {
	title string
	// action to run on confirmation
	run func(m *Model) tea.Cmd
}

// New builds the root model from a scanned state.
func New(state *core.State, targets []string) Model {
	fi := textinput.New()
	fi.Placeholder = "filter skills…"
	fi.Prompt = "/"
	ci := textinput.New()
	ci.Placeholder = "commit message"
	ci.Prompt = "✓ "
	ui := textinput.New()
	ui.Placeholder = "https://github.com/user/skills.git  (empty to clear)"
	ui.Prompt = "↑ "
	return Model{
		hub:           state.Hub,
		targets:       targets,
		state:         state,
		filter:        fi,
		commitInput:   ci,
		upstreamInput: ui,
		status:        "ready",
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

func refreshCmd(hub string, targets []string) tea.Cmd {
	return func() tea.Msg {
		st, err := core.Scan(hub, targets)
		return opResultMsg{state: st, err: err}
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case opResultMsg:
		m.busy = false
		if msg.staleness != nil {
			m.staleness = msg.staleness
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.errMsg = ""
			if msg.state != nil {
				m.state = msg.state
				m.hub = msg.state.Hub
			}
		}
		if msg.status != "" {
			m.status = msg.status
		}
		return m, nil

	case statusMsg:
		m.status = string(msg)
		return m, nil

	case tea.KeyMsg:
		if m.confirm != nil {
			return m.updateConfirm(msg)
		}
		if m.commitActive {
			return m.updateCommitInput(msg)
		}
		if m.upstreamActive {
			return m.updateUpstreamInput(msg)
		}
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateGlobal(msg)
	}
	return m, nil
}

func (m Model) updateGlobal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.status = "1/2/3 or tab: switch views · j/k move · space toggle · / filter (skills) · o open editor · u update · a adopt · x remove link (targets) · c commit · p push · f staleness (hub) · r rescan · q quit"
		return m, nil
	case "1":
		m.tab = tabSkills
	case "2":
		m.tab = tabTargets
	case "3":
		m.tab = tabHub
	case "tab":
		m.tab = (m.tab + 1) % 3
	case "r":
		m.busy = true
		return m, refreshCmd(m.hub, m.targets)
	}

	switch m.tab {
	case tabSkills:
		return m.updateSkills(msg)
	case tabTargets:
		return m.updateTargets(msg)
	case tabHub:
		return m.updateHub(msg)
	}
	return m, nil
}

// --- skills view ---

func (m *Model) filtered() []core.Skill {
	skills := m.state.Skills
	if m.filterText == "" {
		return skills
	}
	needle := strings.ToLower(m.filterText)
	var out []core.Skill
	for _, s := range skills {
		if strings.Contains(strings.ToLower(s.Label), needle) ||
			strings.Contains(strings.ToLower(s.Name), needle) ||
			strings.Contains(strings.ToLower(s.Description), needle) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) clampCursor() {
	n := len(m.filtered())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) updateSkills(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.cursor++
		m.clampCursor()
	case "k", "up":
		m.cursor--
		m.clampCursor()
	case "h", "left":
		if m.focusTarget > 0 {
			m.focusTarget--
		}
	case "l", "right":
		if m.focusTarget < len(m.targets)-1 {
			m.focusTarget++
		}
	case "/":
		m.filtering = true
		m.filter.SetValue("")
		m.filter.Focus()
		return m, textinput.Blink
	case "esc":
		m.filterText = ""
	case " ":
		return m.toggleFocused()
	case "o":
		return m.openEditor()
	case "e":
		return m.startUpstreamEdit()
	case "u":
		return m.updateAll()
	}
	return m, nil
}

func (m Model) toggleFocused() (tea.Model, tea.Cmd) {
	skills := m.filtered()
	if len(skills) == 0 || m.focusTarget >= len(m.targets) {
		return m, nil
	}
	sk := skills[m.cursor]
	target := m.targets[m.focusTarget]
	enabled := sk.Enabled[target]
	hub, targets := m.hub, m.targets
	m.busy = true
	return m, func() tea.Msg {
		var err error
		status := ""
		if enabled {
			err = core.Disable(hub, target, sk.Name)
			status = fmt.Sprintf("disabled %s in %s", sk.Name, filepath.Base(target))
		} else {
			err = core.Enable(hub, target, sk.Rel)
			status = fmt.Sprintf("enabled %s in %s", sk.Name, filepath.Base(target))
		}
		if err != nil {
			return opResultMsg{err: err}
		}
		st, serr := core.Scan(hub, targets)
		return opResultMsg{status: status, err: serr, state: st}
	}
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	skills := m.filtered()
	if len(skills) == 0 {
		return m, nil
	}
	dir := filepath.Join(m.hub, filepath.FromSlash(skills[m.cursor].Rel))
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	var cmd *exec.Cmd
	if editor != "" {
		cmd = exec.Command("sh", "-c", editor+" "+escapeArg(dir))
	} else {
		cmd = exec.Command("open", dir) // reveal in Finder on macOS
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return opResultMsg{err: err}
		}
		return refreshCmd(m.hub, m.targets)()
	})
}

func escapeArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (m Model) startUpstreamEdit() (tea.Model, tea.Cmd) {
	skills := m.filtered()
	if len(skills) == 0 {
		return m, nil
	}
	sk := skills[m.cursor]
	m.upstreamActive = true
	m.upstreamInput.SetValue(sk.Upstream.Source)
	m.upstreamInput.Focus()
	return m, textinput.Blink
}

func (m Model) updateUpstreamInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.upstreamActive = false
		m.upstreamInput.Blur()
		return m, nil
	case "enter":
		skills := m.filtered()
		if len(skills) == 0 {
			return m, nil
		}
		sk := skills[m.cursor]
		m.upstreamActive = false
		m.upstreamInput.Blur()
		newURL := strings.TrimSpace(m.upstreamInput.Value())
		hub, targets := m.hub, m.targets
		m.busy = true
		return m, func() tea.Msg {
			manifest, err := core.LoadManifest(hub)
			if err != nil {
				return opResultMsg{err: err}
			}
			if newURL == "" {
				manifest.Remove(sk.Name)
			} else {
				old, _ := manifest.Get(sk.Name)
				old.Source = newURL
				// A new URL implies an unknown upstream layout and ref: drop any
				// stale Path/Ref/Synced and re-baseline on the current content so
				// the next update either matches the new upstream or is skipped.
				old.Path = ""
				old.Ref = ""
				old.Synced = ""
				if sig, serr := core.DirSignature(filepath.Join(hub, filepath.FromSlash(sk.Rel))); serr == nil {
					old.Tree = sig
				} else {
					old.Tree = ""
				}
				manifest.Set(sk.Name, old)
			}
			if err := manifest.Save(hub); err != nil {
				return opResultMsg{err: err}
			}
			st, serr := core.Scan(hub, targets)
			if newURL == "" {
				return opResultMsg{status: "upstream cleared for " + sk.Name, err: serr, state: st}
			}
			return opResultMsg{status: "upstream set for " + sk.Name, err: serr, state: st}
		}
	}
	var cmd tea.Cmd
	m.upstreamInput, cmd = m.upstreamInput.Update(msg)
	return m, cmd
}

// updateAll runs hub pull + every upstream skill update in one background job.
func (m Model) updateAll() (tea.Model, tea.Cmd) {
	hub, targets := m.hub, m.targets
	m.busy = true
	return m, func() tea.Msg {
		var report []string
		var firstErr error

		if core.IsRepo(hub) {
			if err := core.PullFF(hub); err != nil {
				report = append(report, "hub pull: "+err.Error())
				firstErr = err
			} else {
				report = append(report, "hub pulled")
			}
		}
		manifest, err := core.LoadManifest(hub)
		if err != nil {
			return opResultMsg{err: err}
		}
		for _, name := range sortedKeys(manifest.Skills) {
			meta := manifest.Skills[name]
			rel, ok := core.FindSkillRel(hub, name)
			if !ok {
				continue
			}
			newMeta, err := core.UpdateUpstreamSkill(hub, rel, meta)
			switch {
			case errors.Is(err, core.ErrDiverged):
				report = append(report, name+": skipped (local changes)")
			case errors.Is(err, core.ErrNoBaseline):
				report = append(report, name+": skipped (no baseline)")
			case err != nil:
				report = append(report, name+": "+err.Error())
				if firstErr == nil {
					firstErr = err
				}
			case newMeta.Synced == meta.Synced:
				report = append(report, name+": up to date")
			default:
				manifest.Set(name, newMeta)
				report = append(report, name+": updated")
			}
		}
		if err := manifest.Save(hub); err != nil && firstErr == nil {
			firstErr = err
		}
		st, serr := core.Scan(hub, targets)
		if serr != nil && firstErr == nil {
			firstErr = serr
		}
		status := strings.Join(report, " · ")
		if status == "" {
			status = "nothing to update"
		}
		return opResultMsg{status: status, err: firstErr, state: st}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
