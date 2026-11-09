package cli

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// weekRow is one row of the week grid: a project something is booked on in the
// week, or the hours booked on none. key is the project's id as it travels, or
// "none"; open says whether the row takes hours, which a closed project's does
// not.
type weekRow struct {
	key     string
	name    string
	project *uint
	open    bool
	entries []entry
}

// rowKey is the key of the row an entry on a project is in.
func rowKey(project *uint) string {
	if project == nil {
		return "none"
	}

	return strconv.FormatUint(uint64(*project), 10)
}

// on is the row's entries on a day.
func (r weekRow) on(date string) []entry {
	var on []entry

	for _, e := range r.entries {
		if e.Date == date {
			on = append(on, e)
		}
	}

	return on
}

// weekRows are the grid's rows in the page's order: every project something is
// booked on in the week, by name, and the hours without a project last.
func (m screen) weekRows() []weekRow {
	byKey := map[string]*weekRow{}

	var keys []string

	for _, e := range m.week.entries {
		key := rowKey(e.ProjectID)

		if _, ok := byKey[key]; !ok {
			byKey[key], keys = m.newRow(key, e.ProjectID), append(keys, key)
		}

		byKey[key].entries = append(byKey[key].entries, e)
	}

	rows := make([]weekRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, *byKey[key])
	}

	slices.SortStableFunc(rows, func(a, b weekRow) int {
		switch {
		case a.key == b.key:
			return 0
		case a.key == "none":
			return 1
		case b.key == "none":
			return -1
		}

		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	return rows
}

// newRow is an empty row for a project, or for the hours without one, named as
// the page names it and open where it takes hours: the hours without a project
// always, and a project's while it is active.
func (m screen) newRow(key string, project *uint) *weekRow {
	if project == nil {
		return &weekRow{key: key, name: m.s.lang.text("ts.noProject", "No project"), open: true}
	}

	row := &weekRow{key: key, name: m.s.lang.nameOf(m.shown.projects, project), project: project}

	for _, p := range m.shown.projects {
		if p.ID == *project {
			row.open = p.Status == "active"
		}
	}

	return row
}

// dayOn is what the week's days say about a day, empty for one they do not name.
func (w weekCard) dayOn(date string) weekDay {
	for _, day := range w.days {
		if day.Date == date {
			return day
		}
	}

	return weekDay{Date: date}
}

// weekCardView draws the week as the page draws its grid: its title, the days as
// columns under their names and dates, a row for each project and one for the
// hours without a project, each day's sum and the row's total - a sum of several
// entries marked, since no one entry is behind it - and under them what each day
// came to, what it asked and what was left over, the last only on a day that
// counts. The days that are a public holiday or a day away are named under it.
func (m screen) weekCardView(look palette, width int) string {
	l := m.s.lang

	monday, _ := time.Parse(time.DateOnly, m.week.monday)
	names := l.weekdayNames()

	var dates, short [7]string
	for d := range dates {
		dates[d], short[d] = addDays(m.week.monday, d), l.dayAndMonth(monday.AddDate(0, 0, d))
	}

	_, number := monday.ISOWeek()
	lines := []string{look.title.Render(l.text("week.title", "Week {0} · {1} – {2}",
		strconv.Itoa(number), l.date(dates[0]), l.date(dates[6])))}

	rows := m.weekRows()
	body := make([][]string, len(rows))
	several := false

	for i, r := range rows {
		body[i] = make([]string, 8)
		total := 0.0

		for d, date := range dates {
			on := r.on(date)
			sum := hoursIn(on)
			total += sum

			switch len(on) {
			case 0:
				body[i][d] = "–"
			case 1:
				body[i][d] = l.share(sum)
			default:
				body[i][d], several = l.share(sum)+"*", true
			}
		}

		body[i][7] = l.share(total)
	}

	foot := m.weekFoot(dates)
	labels := []string{l.text("week.booked", "Booked"), l.text("week.target", "Target"), l.text("week.balance", "Balance")}

	columns := weekColumns{gutter: 2, day: 1, total: 1}
	columns.fit(names[:], short[:], body, foot)
	columns.fitTotal(l.text("week.sum", "Total"), body, foot)

	longest := lipgloss.Width(l.text("field.project", "Project"))
	for _, name := range append(slices.Clone(labels), rowNames(rows)...) {
		longest = max(longest, lipgloss.Width(name))
	}

	columns.name = max(min(longest, width-columns.fixed()), 4)

	lines = append(lines,
		look.muted.Render(columns.line("  ", l.text("field.project", "Project"), names[:], l.text("week.sum", "Total"))),
		look.muted.Render(columns.line("  ", "", short[:], "")))

	picking, cursor := m.picking == "cell", m.cellAt(rows)

	for i, r := range rows {
		name := r.name
		if r.key == "none" {
			name = look.muted.Render(name)
		}

		// While a cell is picked, the cursor's row has a > before it and its cell
		// is filled with the accent, and a cell that cannot be picked is quiet.
		gutter, days := "  ", slices.Clone(body[i][:7])

		for d := range days {
			switch {
			case picking && i == cursor && d == m.cell.day:
				days[d] = look.tabOn.Render(fitRight(days[d], columns.day))
			case picking && !m.cellPickable(r, d):
				days[d] = look.muted.Render(days[d])
			}
		}

		if picking && i == cursor {
			gutter = look.title.Render(">") + " "
		}

		lines = append(lines, columns.line(gutter, name, days, body[i][7]))
	}

	if len(rows) == 0 {
		lines = append(lines, "  "+look.muted.Render(l.text("week.empty", "Nothing booked this week yet.")))
	}

	lines = append(lines, look.muted.Render(strings.Repeat("─", min(columns.width(), width))))

	for i, label := range labels {
		lines = append(lines, columns.line("  ", label, foot[i][:7], foot[i][7]))
	}

	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}

	wrapped := lipgloss.NewStyle().Width(width)

	if marks := m.weekMarks(dates, names, short); marks != "" {
		lines = append(lines, wrapped.Render(look.muted.Render(marks)))
	}

	if several {
		lines = append(lines, wrapped.Render(look.muted.Render(
			l.text("track.week.several", "* several entries – choose the cell to see them"))))
	}

	return strings.Join(lines, "\n")
}

// weekFoot is what the week's days came to, asked and left over, each with its
// total: the hours booked, the hours asked, and the balance of each day that
// counts towards it - a dash for one that does not, as the balance has none for
// it either.
func (m screen) weekFoot(dates [7]string) [3][]string {
	l := m.s.lang

	var (
		booked, asked, left float64
		foot                [3][]string
	)

	for _, date := range dates {
		day, sum := m.week.dayOn(date), 0.0

		for _, e := range m.week.entries {
			if e.Date == date {
				sum += e.DurationHours
			}
		}

		booked, asked = booked+sum, asked+day.Target
		foot[0], foot[1] = append(foot[0], l.share(sum)), append(foot[1], l.share(day.Target))

		if !day.Counts {
			foot[2] = append(foot[2], "–")

			continue
		}

		left += sum - day.Target
		foot[2] = append(foot[2], l.signed(sum-day.Target))
	}

	foot[0], foot[1], foot[2] = append(foot[0], l.share(booked)), append(foot[1], l.share(asked)), append(foot[2], l.signed(left))

	return foot
}

// weekMarks names the days of the week that are a public holiday or a day away,
// each by its name and date, as the page tags the day's column.
func (m screen) weekMarks(dates [7]string, names [7]string, short [7]string) string {
	l := m.s.lang

	var marks []string

	for d, date := range dates {
		day := m.week.dayOn(date)

		var said []string

		if day.Holiday != "" {
			said = append(said, l.holidayOf(day))
		}

		if day.Absence != "" {
			said = append(said, l.absenceName(day.Absence))
		}

		if len(said) > 0 {
			marks = append(marks, names[d]+" "+short[d]+": "+strings.Join(said, ", "))
		}
	}

	return strings.Join(marks, " · ")
}

// rowNames are the rows' names.
func rowNames(rows []weekRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.name
	}

	return names
}

// hoursIn is what entries add up to.
func hoursIn(entries []entry) float64 {
	sum := 0.0
	for _, e := range entries {
		sum += e.DurationHours
	}

	return sum
}

// weekColumns are how wide the week grid's columns are drawn: the cursor's
// gutter, the rows' names, each day and the total. A day is as wide as the
// widest thing in any day's column, so the days line up; a column sits a space
// from the one before it, which is what leaves the names room on a terminal
// eighty wide.
type weekColumns struct {
	gutter, name, day, total int
}

// fit widens the days to the widest of their names, dates and figures.
func (c *weekColumns) fit(names, short []string, body [][]string, foot [3][]string) {
	for d := range 7 {
		c.day = max(c.day, lipgloss.Width(names[d]), lipgloss.Width(short[d]))

		for _, row := range body {
			c.day = max(c.day, lipgloss.Width(row[d]))
		}

		for _, row := range foot {
			c.day = max(c.day, lipgloss.Width(row[d]))
		}
	}
}

// fitTotal widens the total to the widest of its heading and figures.
func (c *weekColumns) fitTotal(heading string, body [][]string, foot [3][]string) {
	c.total = max(c.total, lipgloss.Width(heading))

	for _, row := range body {
		c.total = max(c.total, lipgloss.Width(row[7]))
	}

	for _, row := range foot {
		c.total = max(c.total, lipgloss.Width(row[7]))
	}
}

// fixed is how wide everything but the names is.
func (c weekColumns) fixed() int {
	return c.gutter + 7*(1+c.day) + 1 + c.total
}

// width is how wide a line of the grid is.
func (c weekColumns) width() int {
	return c.fixed() + c.name
}

// line sets a row in the columns: the gutter, the name at the left, and each
// day and the total at the right of theirs, as the page sets figures.
func (c weekColumns) line(gutter, name string, days []string, total string) string {
	var b strings.Builder

	b.WriteString(gutter)
	b.WriteString(fitLeft(name, c.name))

	for _, day := range days {
		b.WriteString(" " + fitRight(day, c.day))
	}

	b.WriteString(" " + fitRight(total, c.total))

	return strings.TrimRight(b.String(), " ")
}

// fitLeft sets text at the left of a column of width, cut short where it is
// wider.
func fitLeft(text string, width int) string {
	text = ansi.Truncate(text, width, "…")

	return text + strings.Repeat(" ", max(width-lipgloss.Width(text), 0))
}

// fitRight sets text at the right of a column of width, cut short where it is
// wider.
func fitRight(text string, width int) string {
	text = ansi.Truncate(text, width, "…")

	return strings.Repeat(" ", max(width-lipgloss.Width(text), 0)) + text
}

// signed writes a balance with its sign, as the page's week writes one: ±0 for
// none, and a minus that is a minus rather than a hyphen.
func (l language) signed(v float64) string {
	switch {
	case math.Abs(v) < 0.005:
		return "±0"
	case v > 0:
		return "+" + l.share(v)
	}

	return "−" + l.share(-v)
}
