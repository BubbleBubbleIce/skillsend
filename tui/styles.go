// Package tui renders Skillsend's three-view management panel:
// Skills (browse & toggle), Targets (reverse audit), Hub (git sync).
//
// Theming is Catppuccin (https://github.com/catppuccin/catppuccin).
// The flavor is picked from the terminal background — Mocha on dark
// terminals, Latte on light ones — can be pinned with the SKILLSEND_FLAVOR
// env var (latte|frappe|macchiato|mocha), and cycled at runtime with "t".
package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// flavor names, in the order the "t" key cycles through them.
const (
	flavorLatte     = "latte"
	flavorFrappe    = "frappe"
	flavorMacchiato = "macchiato"
	flavorMocha     = "mocha"
)

var flavorOrder = []string{flavorMocha, flavorMacchiato, flavorFrappe, flavorLatte}

// palette holds the 26 official Catppuccin colors of one flavor.
type palette struct {
	name string
	dark bool

	// accents
	rosewater, flamingo, pink, mauve, red, maroon, peach, yellow string
	green, teal, sky, sapphire, blue, lavender                   string

	// neutrals
	text, subtext1, subtext0, overlay2, overlay1, overlay0 string
	surface2, surface1, surface0, base, mantle, crust      string
}

// muted is the color of chrome that should read as quiet but stay legible:
// column headers, detail keys. Dark backgrounds need a lighter gray than
// light ones, otherwise the same overlay value flips to unreadable.
func (p palette) muted() string {
	if p.dark {
		return p.overlay2
	}
	return p.subtext0
}

// dim is for text that should recede: footers, disabled rows, the hub path.
func (p palette) dim() string {
	if p.dark {
		return p.overlay0
	}
	return p.overlay2
}

var flavors = map[string]palette{
	flavorLatte: {
		name: flavorLatte, dark: false,
		rosewater: "#DC8A78", flamingo: "#DD7878", pink: "#EA76CB", mauve: "#8839EF",
		red: "#D20F39", maroon: "#E64553", peach: "#FE640B", yellow: "#DF8E1D",
		green: "#40A02B", teal: "#179299", sky: "#04A5E5", sapphire: "#209FB5",
		blue: "#1E66F5", lavender: "#7287FD",
		text: "#4C4F69", subtext1: "#5C5F77", subtext0: "#6C6F85", overlay2: "#7C7F93",
		overlay1: "#8C8FA1", overlay0: "#9CA0B0", surface2: "#ACB0BE", surface1: "#BCC0CC",
		surface0: "#CCD0DA", base: "#EFF1F5", mantle: "#E6E9EF", crust: "#DCE0E8",
	},
	flavorFrappe: {
		name: flavorFrappe, dark: true,
		rosewater: "#F2D5CF", flamingo: "#EEBEBE", pink: "#F4B8E4", mauve: "#CA9EE6",
		red: "#E78284", maroon: "#EA999C", peach: "#EF9F76", yellow: "#E5C890",
		green: "#A6D189", teal: "#81C8BE", sky: "#99D1DB", sapphire: "#85C1DC",
		blue: "#8CAAEE", lavender: "#BABBF1",
		text: "#C6D0F5", subtext1: "#B5BFE2", subtext0: "#A5ADCE", overlay2: "#949CBB",
		overlay1: "#838BA7", overlay0: "#737994", surface2: "#626880", surface1: "#51576D",
		surface0: "#414559", base: "#303446", mantle: "#292C3C", crust: "#232634",
	},
	flavorMacchiato: {
		name: flavorMacchiato, dark: true,
		rosewater: "#F4DBD6", flamingo: "#F0C6C6", pink: "#F5BDE6", mauve: "#C6A0F6",
		red: "#ED8796", maroon: "#EE99A0", peach: "#F5A97F", yellow: "#EED49F",
		green: "#A6DA95", teal: "#8BD5CA", sky: "#91D7E3", sapphire: "#7DC4E4",
		blue: "#8AADF4", lavender: "#B7BDF8",
		text: "#CAD3F5", subtext1: "#B8C0E0", subtext0: "#A5ADCB", overlay2: "#939AB7",
		overlay1: "#8087A2", overlay0: "#6E738D", surface2: "#5B6078", surface1: "#494D64",
		surface0: "#363A4F", base: "#24273A", mantle: "#1E2030", crust: "#181926",
	},
	flavorMocha: {
		name: flavorMocha, dark: true,
		rosewater: "#F5E0DC", flamingo: "#F2CDCD", pink: "#F5C2E7", mauve: "#CBA6F7",
		red: "#F38BA8", maroon: "#EBA0AC", peach: "#FAB387", yellow: "#F9E2AF",
		green: "#A6E3A1", teal: "#94E2D5", sky: "#89DCEB", sapphire: "#74C7EC",
		blue: "#89B4FA", lavender: "#B4BEFE",
		text: "#CDD6F4", subtext1: "#BAC2DE", subtext0: "#A6ADC8", overlay2: "#9399B2",
		overlay1: "#7F849C", overlay0: "#6C7086", surface2: "#585B70", surface1: "#45475A",
		surface0: "#313244", base: "#1E1E2E", mantle: "#181825", crust: "#11111B",
	},
}

// activeFlavor is the flavor currently materialized into the style vars.
var activeFlavor string

var (
	titleStyle     lipgloss.Style
	tabStyle       lipgloss.Style
	activeTabStyle lipgloss.Style
	hubPathStyle   lipgloss.Style
	colHeaderStyle lipgloss.Style
	selectedStyle  lipgloss.Style
	detailKeyStyle lipgloss.Style
	statusStyle    lipgloss.Style
	errStyle       lipgloss.Style
	footerStyle    lipgloss.Style
	focusRingStyle lipgloss.Style

	// state glyphs
	onStyle       lipgloss.Style // enabled
	offStyle      lipgloss.Style // disabled
	foreignStyle  lipgloss.Style // foreign
	brokenStyle   lipgloss.Style // broken
	conflictStyle lipgloss.Style
	dirtyStyle    lipgloss.Style
	behindStyle   lipgloss.Style
)

func init() {
	activeFlavor = detectFlavor()
	buildStyles(flavors[activeFlavor])
}

// detectFlavor honors SKILLSEND_FLAVOR, then falls back to the terminal's
// reported background: Mocha reads well on dark, Latte on light.
func detectFlavor() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("SKILLSEND_FLAVOR"))); v != "" {
		if _, ok := flavors[v]; ok {
			return v
		}
	}
	if lipgloss.HasDarkBackground() {
		return flavorMocha
	}
	return flavorLatte
}

// CycleFlavor advances to the next flavor and returns its name.
func CycleFlavor() string {
	i := 0
	for k, f := range flavorOrder {
		if f == activeFlavor {
			i = k
			break
		}
	}
	next := flavorOrder[(i+1)%len(flavorOrder)]
	SetFlavor(next)
	return next
}

// SetFlavor switches the palette and re-renders every style in place.
func SetFlavor(name string) {
	p, ok := flavors[name]
	if !ok {
		return
	}
	activeFlavor = name
	buildStyles(p)
}

// Flavor reports the active flavor name.
func Flavor() string { return activeFlavor }

// buildStyles maps one palette onto the semantic roles the views use.
// Every background is left transparent so the terminal's own colors show
// through — a TUI that paints its own base would fight the user's theme.
func buildStyles(p palette) {
	c := func(s string) lipgloss.Color { return lipgloss.Color(s) }

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(c(p.crust)).
		Background(c(p.mauve)).
		Padding(0, 1)

	tabStyle = lipgloss.NewStyle().
		Foreground(c(p.muted())).
		Padding(0, 2)
	activeTabStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(c(p.mauve)).
		Padding(0, 2).
		Underline(true)

	hubPathStyle = lipgloss.NewStyle().Foreground(c(p.dim()))
	colHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.muted()))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.mauve))
	detailKeyStyle = lipgloss.NewStyle().Foreground(c(p.muted()))
	statusStyle = lipgloss.NewStyle().Foreground(c(p.green))
	errStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.red))
	footerStyle = lipgloss.NewStyle().Foreground(c(p.dim()))
	focusRingStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(c(p.mauve)).
		PaddingLeft(1)

	onStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.green))
	offStyle = lipgloss.NewStyle().Foreground(c(p.dim()))
	foreignStyle = lipgloss.NewStyle().Foreground(c(p.yellow))
	brokenStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.red))
	conflictStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.peach))
	dirtyStyle = lipgloss.NewStyle().Foreground(c(p.peach))
	behindStyle = lipgloss.NewStyle().Foreground(c(p.sky))
}

// themeInput paints a text input with the active palette. Bubbles ships its
// own grays, which would sit awkwardly next to Catppuccin everywhere else.
func themeInput(ti *textinput.Model) {
	p := flavors[activeFlavor]
	c := func(s string) lipgloss.Color { return lipgloss.Color(s) }
	ti.PromptStyle = lipgloss.NewStyle().Foreground(c(p.peach))
	ti.TextStyle = lipgloss.NewStyle().Foreground(c(p.text))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(c(p.dim()))
	ti.CompletionStyle = lipgloss.NewStyle().Foreground(c(p.dim()))
}
