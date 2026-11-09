package cli

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// subYear names the year's menu, opened from the overtime's own.
const subYear = "year"

// yearMonths is how many rows the year's months take as figures.
const yearMonths = 12

// yearGlyphs are the tiles' shades, from a day with nothing booked to one with
// more than eight hours. A terminal has four shades of ink, so the last two
// steps share the full block and are told apart by their colour alone - which
// is how the page tells all six apart.
var yearGlyphs = [6]string{"·", "░", "▒", "▓", "█", "█"}

// yearDays is a year of days as /me/days answers them, and the year before it,
// which the page's tiles compare it with.
type yearDays struct {
	year         int
	days, before []weekDay
}

// yearRead is a year of days read on its own, for the year the tiles show.
type yearRead struct {
	days yearDays
	err  error
}

// readYear reads a year of days and the year before it, as the page's tiles
// read them: each a whole year, the empty days included.
func (s *session) readYear(ctx context.Context, year int) (yearDays, error) {
	read := yearDays{year: year}

	for i, into := range []*[]weekDay{&read.days, &read.before} {
		y := strconv.Itoa(year - i)

		days, err := s.daysFrom(ctx, y+"-01-01", y+"-12-31")
		if err != nil {
			return yearDays{}, err
		}

		*into = days
	}

	return read, nil
}

// tilesYear is the year the tiles show: the one moved to, or this one.
func (m screen) tilesYear() int {
	if m.ot.yearShown != 0 {
		return m.ot.yearShown
	}

	return m.today().Year()
}

// stepTiles moves the tiles a year back or on, and reads the year they move to.
func (m screen) stepTiles(years int) (tea.Model, tea.Cmd) {
	shown := m.tilesYear() + years
	if shown == m.today().Year() {
		shown = 0
	}

	m.ot.yearShown = shown

	return m, m.readYearNow()
}

// readYearNow reads the year the tiles show, without the rest of the day.
func (m screen) readYearNow() tea.Cmd {
	ctx, s, year := m.ctx, m.s, m.tilesYear()

	return func() tea.Msg {
		days, err := s.readYear(ctx, year)

		return yearRead{days: days, err: err}
	}
}

// tookYear keeps a year of days read, where it is still the year the tiles show.
func (m screen) tookYear(msg yearRead) screen {
	switch {
	case msg.err != nil:
		m.stale = msg.err.Error()
	case msg.days.year == m.tilesYear():
		m.ot.year = msg.days
	}

	return m
}

// yearLevel is how dark a day's tile is, as the page's yearLevel has it: by the
// hours booked on it, two hours a step.
func yearLevel(hours float64) int {
	if hours <= 0 {
		return 0
	}

	return min(5, int(math.Ceil(hours/2)))
}

// yearBody is what the year's menu shows above it: the year's card, once the
// year has been read.
func (m screen) yearBody(look palette, width, rows int) []string {
	if m.ot.year.year != m.tilesYear() {
		return []string{look.muted.Render(m.s.lang.text("track.year.reading", "Reading the year …"))}
	}

	return []string{m.yearCardView(look, width, rows)}
}

// yearCardView draws the year as the page's card draws it: headed by the year
// and what was booked in it against the year before, the tiles under the
// months' names, the legend, and the months as figures - as many as the window
// has room for, from the one at the top, saying which.
func (m screen) yearCardView(look palette, width, rows int) string {
	l := m.s.lang

	lines := []string{
		look.title.Render(l.text("year.title", "The year {0}", strconv.Itoa(m.ot.year.year))),
		lipgloss.NewStyle().Width(width).Render(look.muted.Render(m.yearSummary())),
	}

	table := m.yearTable()

	lines = append(lines, m.yearTiles(look)...)
	lines = append(lines, m.yearLegend(look), table.heading(look, width))

	months := table.lines(width)
	rows = min(rows, len(months))
	top := max(min(m.ot.yearTop, len(months)-rows), 0)

	lines = append(lines, months[top:top+rows]...)

	if rows < len(months) {
		lines = append(lines, look.muted.Render(spread(width, "", l.place(top, rows, len(months)))))
	}

	return strings.Join(lines, "\n")
}

// yearSummary is what the page says over its tiles: the hours booked and on how
// many days, and the hours of the year before. A year that is running is set
// against the year before up to the same day, since a year half done against a
// whole one would always say less; the days with hours are the whole year's, as
// the page counts them.
func (m screen) yearSummary() string {
	l, year := m.s.lang, m.ot.year

	upTo := "12-31"
	if today := m.today(); today.Year() == year.year {
		upTo = today.Format("01-02")
	}

	bookedUpTo := func(days []weekDay) float64 {
		total := 0.0

		for _, day := range days {
			if len(day.Date) == len(time.DateOnly) && day.Date[5:] <= upTo {
				total += day.Booked
			}
		}

		return total
	}

	withHours := 0

	for _, day := range year.days {
		if day.Booked > 0 {
			withHours++
		}
	}

	return l.text("year.booked", "Booked: {0} · days with hours: {1}", l.hours(bookedUpTo(year.days)), strconv.Itoa(withHours)) +
		" · " + l.text("year.compare", "{0} to the same day: {1}", strconv.Itoa(year.year-1), l.hours(bookedUpTo(year.before)))
}

// yearTiles draws the year as the page lays out its tiles - a column a week,
// Monday first, so that every row is one weekday - under the months' names, each
// over the week its first day falls in. The rows are named by their weekday,
// which the page leaves to the tiles' own titles. A tile is shaded by its hours,
// a public holiday and a day away are marked rather than shaded, as the page
// outlines them, and today is filled with the accent.
func (m screen) yearTiles(look palette) []string {
	l, year := m.s.lang, m.ot.year

	first := time.Date(year.year, 1, 1, 0, 0, 0, 0, time.UTC)
	lead := (int(first.Weekday()) + 6) % 7
	columns := (lead + first.AddDate(1, 0, -1).YearDay() + 6) / 7
	today := m.today().Format(time.DateOnly)

	var tiles [7][]string
	for row := range tiles {
		tiles[row] = slices.Repeat([]string{" "}, columns)
	}

	for _, day := range year.days {
		at, err := time.Parse(time.DateOnly, day.Date)
		if err != nil || at.Year() != year.year {
			continue
		}

		place := lead + at.YearDay() - 1
		tiles[place%7][place/7] = yearTile(look, day, today)
	}

	names := l.weekdayNames()

	named := 0
	for _, name := range names {
		named = max(named, lipgloss.Width(name))
	}

	labels := []rune(strings.Repeat(" ", columns+3))

	for month := time.January; month <= time.December; month++ {
		start := time.Date(year.year, month, 1, 0, 0, 0, 0, time.UTC)
		name := []rune(l.monthName(start))
		copy(labels[(lead+start.YearDay()-1)/7:], name[:min(3, len(name))])
	}

	lines := []string{look.muted.Render(strings.Repeat(" ", named+1) + strings.TrimRight(string(labels), " "))}

	for row, name := range names {
		lines = append(lines, look.muted.Render(fitLeft(name, named))+" "+strings.Join(tiles[row], ""))
	}

	return lines
}

// yearTile draws one day's tile: its shade, or its mark where it is a public
// holiday or a day away - in the colour of its hours where there are any, as the
// page draws the outline over the shade - and today filled with the accent.
func yearTile(look palette, day weekDay, today string) string {
	level := yearLevel(day.Booked)
	glyph, style := yearGlyphs[level], look.levels[level]

	if day.Holiday != "" || day.Absence != "" {
		glyph = "◇"
		if day.Holiday != "" {
			glyph = "◆"
		}

		if level == 0 {
			style = lipgloss.NewStyle()
		}
	}

	if day.Date == today {
		style = look.tabOn
	}

	return style.Render(glyph)
}

// yearLegend says what the tiles mean, as the page's legend under them does:
// the shades from less to more, and the marks of a public holiday and a day away.
func (m screen) yearLegend(look palette) string {
	l := m.s.lang

	var shades strings.Builder
	for level, glyph := range yearGlyphs {
		shades.WriteString(look.levels[level].Render(glyph))
	}

	return look.muted.Render(l.text("year.less", "Less")+" ") + shades.String() +
		look.muted.Render(" "+l.text("year.more", "More")+"   ") + "◆" +
		look.muted.Render(" "+l.text("year.holiday", "Public holiday")+"   ") + "◇" +
		look.muted.Render(" "+l.text("year.away", "Away"))
}

// yearTable is the page's table under the tiles: each month with its hours in
// the year and in the year before - what the tiles say, as figures.
func (m screen) yearTable() grid {
	l, year := m.s.lang, m.ot.year

	perMonth := func(days []weekDay) [yearMonths]float64 {
		var sums [yearMonths]float64

		for _, day := range days {
			if at, err := time.Parse(time.DateOnly, day.Date); err == nil {
				sums[at.Month()-1] += day.Booked
			}
		}

		return sums
	}

	shown, before := perMonth(year.days), perMonth(year.before)

	table := grid{
		headings: []string{l.text("year.month", "Month"), strconv.Itoa(year.year), strconv.Itoa(year.year - 1)},
		right:    []bool{false, true, true},
	}

	for month := range yearMonths {
		name := l.monthName(time.Date(year.year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC))
		table.rows = append(table.rows, []string{name, l.hours(shown[month]), l.hours(before[month])})
	}

	return table
}
