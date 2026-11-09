package cli

import (
	"image/color"
	"math"

	"charm.land/lipgloss/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// tones are the web interface's colours, light or dark, so that the terminal
// and the browser look like one application: app.css holds the same ones. ask
// is its teal, the charts' second hue there, which says what the screen wants
// to know, and bg the page's background, which the year's tiles mix the accent
// into.
type tones struct {
	accent, onAccent, muted, danger, green, line, ask, bg color.Color
}

// tonesFor is the colours for a dark terminal or a light one.
func tonesFor(dark bool) tones {
	pick := lipgloss.LightDark(dark)

	return tones{
		accent:   pick(lipgloss.Color("#2f6feb"), lipgloss.Color("#5b8dfa")),
		onAccent: pick(lipgloss.Color("#ffffff"), lipgloss.Color("#0d1117")),
		muted:    pick(lipgloss.Color("#626d78"), lipgloss.Color("#98a2ad")),
		danger:   pick(lipgloss.Color("#c0392b"), lipgloss.Color("#e06c5c")),
		green:    lipgloss.Color("#2e9e5b"),
		line:     pick(lipgloss.Color("#dfe3e8"), lipgloss.Color("#2c333b")),
		ask:      pick(lipgloss.Color("#12a19a"), lipgloss.Color("#3fc9c0")),
		bg:       pick(lipgloss.Color("#f5f6f8"), lipgloss.Color("#14171a")),
	}
}

// mixed is share of a and the rest of b, channel by channel, as app.css's
// color-mix in srgb mixes them.
func mixed(a, b color.Color, share float64) color.Color {
	r1, g1, b1, _ := a.RGBA()
	r2, g2, b2, _ := b.RGBA()

	channel := func(x, y uint32) uint8 {
		return uint8(math.Round((float64(x)*share + float64(y)*(1-share)) / 0x101))
	}

	return color.RGBA{R: channel(r1, r2), G: channel(g1, g2), B: channel(b1, b2), A: 0xff}
}

// palette is the styles the screen draws with. tab and tabOn are the page's
// tabs, the one on screen filled with the accent as app.css fills it. levels are
// the year's tiles from a day with nothing booked to one with the most: the
// quiet colour, then the accent mixed into the background in the shares app.css
// mixes it.
type palette struct {
	title, strong, muted, ok, failed, key, ask lipgloss.Style
	tab, tabOn                                 lipgloss.Style
	levels                                     [6]lipgloss.Style

	frame, idle, running color.Color
}

// paletteFor is the palette for a dark terminal or a light one.
func paletteFor(dark bool) palette {
	c := tonesFor(dark)

	levels := [6]lipgloss.Style{lipgloss.NewStyle().Foreground(c.muted)}
	for i, share := range []float64{0.30, 0.48, 0.64, 0.82, 1} {
		levels[i+1] = lipgloss.NewStyle().Foreground(mixed(c.accent, c.bg, share))
	}

	return palette{
		levels:  levels,
		title:   lipgloss.NewStyle().Bold(true).Foreground(c.accent),
		strong:  lipgloss.NewStyle().Bold(true),
		muted:   lipgloss.NewStyle().Foreground(c.muted),
		ok:      lipgloss.NewStyle().Foreground(c.green),
		failed:  lipgloss.NewStyle().Foreground(c.danger),
		key:     lipgloss.NewStyle().Bold(true).Foreground(c.accent),
		ask:     lipgloss.NewStyle().Foreground(c.ask),
		tab:     lipgloss.NewStyle().Foreground(c.muted),
		tabOn:   lipgloss.NewStyle().Foreground(c.onAccent).Background(c.accent),
		frame:   c.accent,
		idle:    c.muted,
		running: c.green,
	}
}

// darkness says whether the screen draws for a dark background: what the
// terminal says its background is, and the appearance the account chose where
// the terminal does not say.
//
// The terminal's answer wins because the screen cannot change the background,
// only draw on it: the colours for a dark page are unreadable on a light
// terminal, whatever somebody chose for the browser. The account's choice is the
// best guess left where the terminal cannot be asked, and a terminal that cannot
// is drawn for light as before when the account chose nothing.
func (m screen) darkness() bool {
	if m.measured {
		return m.dark
	}

	return m.shown.theme == model.ThemeDark
}
