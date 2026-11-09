package cli

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// grid is a table as the screen draws the page's tables: headings over rows of
// cells, each column as wide as its widest cell, figures set to the right as the
// page sets them, and the last column taking what is left. A line is cut at the
// width rather than wrapped, so a row is always one line. Cells may carry their
// own colours.
type grid struct {
	headings []string

	// right says which columns hold figures, and most how wide a column may be at
	// the most - a share of the width, as width/most, zero for no bound.
	right []bool
	most  []int

	rows [][]string
}

// widths is how wide each column but the last is drawn on a line of width.
func (g grid) widths(width int) []int {
	widths := make([]int, len(g.headings))

	for i, heading := range g.headings {
		widths[i] = lipgloss.Width(heading)

		for _, row := range g.rows {
			widths[i] = max(widths[i], lipgloss.Width(row[i]))
		}

		if i < len(g.most) && g.most[i] > 0 {
			widths[i] = min(widths[i], width/g.most[i])
		}
	}

	return widths
}

// line sets one row's cells in their columns, cut at the width.
func (g grid) line(widths []int, cells []string, width int) string {
	var b strings.Builder

	for i, cell := range cells {
		if i == len(cells)-1 {
			// The last column takes what is left, unless it holds figures, which
			// line up at the right whatever column they are in.
			if i < len(g.right) && g.right[i] {
				cell = strings.Repeat(" ", max(widths[i]-lipgloss.Width(cell), 0)) + cell
			}

			b.WriteString(cell)

			break
		}

		cell = ansi.Truncate(cell, widths[i], "…")
		gap := strings.Repeat(" ", max(widths[i]-lipgloss.Width(cell), 0))

		if i < len(g.right) && g.right[i] {
			b.WriteString(gap + cell)
		} else {
			b.WriteString(cell + gap)
		}

		b.WriteString("  ")
	}

	return ansi.Truncate(b.String(), width, "…")
}

// heading is the line of headings, in the quiet colour.
func (g grid) heading(look palette, width int) string {
	return look.muted.Render(g.line(g.widths(width), g.headings, width))
}

// lines are the rows, each a line of width.
func (g grid) lines(width int) []string {
	widths := g.widths(width)
	lines := make([]string, len(g.rows))

	for i, row := range g.rows {
		lines[i] = g.line(widths, row, width)
	}

	return lines
}
