package cli

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// weekCell is the cell of the week grid the cursor is on: its row by the row's
// key, the row's place when the cursor was last put there - which a row that is
// gone once the week is read again leaves the cursor at - and its day, Monday
// first.
type weekCell struct {
	row string
	at  int
	day int
}

// cellPicked is the cell hours are entered into, as it was when it was picked:
// its row's project - nil for the hours without one - and name, its day, the
// one entry behind it where there is one, and what it held.
type cellPicked struct {
	project *uint
	name    string
	date    string
	entry   uint
	before  float64
}

// weekChoices are the week's menu: entering hours into a cell, adding a row and
// copying last week, for an account that may book its own time - and for one
// that may not, the way to the entries behind a cell of several, which the page
// links to for anybody who sees the grid. Nothing while the week moved to is
// still being read, no cell in a grid with none to pick, and no row where every
// row there could be is there.
func (m screen) weekChoices() []choice {
	l := m.s.lang

	if m.week.monday != m.weekMonday() {
		return nil
	}

	rows := m.weekRows()

	var choices []choice

	if m.anyCell(rows) {
		called := l.text("track.week.show", "Show a cell's entries")
		if m.mayWrite() {
			called = l.text("track.week.cell", "Enter hours")
		}

		choices = append(choices, choice{"e", "cell", called})
	}

	if !m.mayWrite() {
		return choices
	}

	if len(m.addable(rows)) > 0 {
		choices = append(choices, choice{"a", "addRow", l.text("track.week.addRow", "Add a row")})
	}

	return append(choices, choice{"c", "copyWeek", l.text("week.copyLast", "Copy last week")})
}

// cellPickable reports whether a cell can be picked: one of several entries
// always, since it leads to them, and any other in a row that takes hours, for
// an account that may book them - as the page draws the one a link and the other
// a field, and a closed project's cells as figures.
func (m screen) cellPickable(r weekRow, day int) bool {
	return len(r.on(addDays(m.week.monday, day))) > 1 || (m.mayWrite() && r.open)
}

// anyCell reports whether the grid has a cell to pick.
func (m screen) anyCell(rows []weekRow) bool {
	for _, r := range rows {
		if _, ok := m.nearestCell(r, 0); ok {
			return true
		}
	}

	return false
}

// nearestCell is the day of a row's cell to pick nearest to day, earlier first
// where two are as near.
func (m screen) nearestCell(r weekRow, day int) (int, bool) {
	for by := range 7 {
		for _, d := range []int{day - by, day + by} {
			if d >= 0 && d < 7 && m.cellPickable(r, d) {
				return d, true
			}
		}
	}

	return 0, false
}

// startCell puts the cursor on a cell to pick: today's in the first row that has
// one where the grid shows this week, Monday's in another week, or the nearest
// day to that the row has a cell to pick on.
func (m screen) startCell() (tea.Model, tea.Cmd) {
	day := 0
	if m.week.monday == mondayOf(m.today()) {
		day = (int(m.today().Weekday()) + 6) % 7
	}

	for at, r := range m.weekRows() {
		if d, ok := m.nearestCell(r, day); ok {
			m.picking, m.news, m.failure = "cell", "", ""
			m.cell = weekCell{row: r.key, at: at, day: d}

			return m, nil
		}
	}

	return m, nil
}

// cellAt is the row the cursor is in: the row it was put in, or where that is
// gone, the one now standing where it stood. Less than zero in a grid with no
// rows.
func (m screen) cellAt(rows []weekRow) int {
	for at, r := range rows {
		if r.key == m.cell.row {
			return at
		}
	}

	return min(m.cell.at, len(rows)-1)
}

// cellKey answers a key while a cell is being picked: the arrows move to the
// next cell to pick in their direction, enter takes the one the cursor is on -
// not while a change is still on its way, since the grid shows the week as it
// was before it - and escape puts the picking away.
func (m screen) cellKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	rows := m.weekRows()

	at := m.cellAt(rows)
	if at < 0 {
		m.picking = ""

		return m, nil
	}

	switch k.String() {
	case "esc":
		m.picking = ""
	case "enter":
		if !m.busy && m.week.monday == m.weekMonday() && m.cellPickable(rows[at], m.cell.day) {
			return m.pickedCell(rows[at])
		}
	case "left", "right":
		by := 1
		if k.String() == "left" {
			by = -1
		}

		for d := m.cell.day + by; d >= 0 && d < 7; d += by {
			if m.cellPickable(rows[at], d) {
				m.cell = weekCell{row: rows[at].key, at: at, day: d}

				break
			}
		}
	case "up", "k", "down", "j":
		by := 1
		if k.String() == "up" || k.String() == "k" {
			by = -1
		}

		for next := at + by; next >= 0 && next < len(rows); next += by {
			if d, ok := m.nearestCell(rows[next], m.cell.day); ok {
				m.cell = weekCell{row: rows[next].key, at: next, day: d}

				break
			}
		}
	}

	return m, nil
}

// pickedCell does what enter on a cell does: a cell of several entries leads to
// them - the time entries, narrowed to its day and its project, as the page's
// link does - and any other is asked how many hours it holds.
func (m screen) pickedCell(r weekRow) (tea.Model, tea.Cmd) {
	date := addDays(m.week.monday, m.cell.day)
	on := r.on(date)

	if len(on) > 1 {
		m.view, m.sub, m.picking = viewTimesheets, subEntries, ""

		return m.narrow(entryFilter{from: date, to: date, project: r.key})
	}

	picked := cellPicked{project: r.project, name: r.name, date: date}
	asked := &answers{cell: picked}

	if len(on) == 1 {
		asked.cell.entry, asked.cell.before = on[0].ID, on[0].DurationHours
		asked.hours = m.s.lang.share(on[0].DurationHours)
	}

	m.picking, m.news, m.failure = "", "", ""

	return m.openWith("cell", asked)
}

// cellSteps ask how many hours a cell holds, starting from what it holds now,
// and - only where it was emptied and an entry is behind it - whether that entry
// is taken back, in the page's words.
func (m screen) cellSteps(a *answers) []*huh.Group {
	l, c := m.s.lang, a.cell

	about := c.name
	if at, err := time.Parse(time.DateOnly, c.date); err == nil {
		about += " · " + l.weekday(at)
	}

	if c.entry != 0 {
		about += " · " + l.text("track.week.emptyBack", "Emptied, the cell takes its entry back.")
	}

	hours := huh.NewInput().Key("hours").Title(l.text("track.ask.hours", "How many hours?")).Description(about).
		Placeholder(l.text("track.form.hoursHint", "1.5, 1,5 or 1:30")).
		Validate(func(raw string) error {
			if strings.TrimSpace(raw) == "" {
				return nil
			}

			return l.readableHours(raw)
		}).Value(&a.hours)

	back := huh.NewConfirm().Key("confirm").Title(l.text("week.removeTitle", "Take this entry back?")).
		Description(l.text("week.removeText", "The {0} booked on {1} are deleted.", l.hours(c.before), l.date(c.date))).
		Affirmative(l.text("track.form.deleteYes", "Delete")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed)

	return []*huh.Group{
		huh.NewGroup(hours),
		huh.NewGroup(back).WithHideFunc(func() bool { return c.entry == 0 || strings.TrimSpace(a.hours) != "" }),
	}
}

// saveCell makes a cell's change as the page makes it: a number in an empty cell
// books it on the row's project and the column's day, a changed number corrects
// the entry behind it - its hours alone - and an emptied cell takes the entry
// back once the answer was yes. The number it held already changes nothing.
func (m screen) saveCell() tea.Cmd {
	ctx, s, a := m.ctx, m.s, *m.asked
	c, typed := a.cell, strings.TrimSpace(a.hours)

	if typed == "" {
		if c.entry == 0 || !a.confirmed {
			return nil
		}

		return func() tea.Msg {
			if err := s.api.call(ctx, http.MethodDelete, fmt.Sprintf("/timesheets/%d", c.entry), nil, nil); err != nil {
				return done{err: err}
			}

			return done{said: s.lang.text("week.removed", "Entry taken back")}
		}
	}

	hours, err := s.lang.parseHours(typed)

	switch {
	case err != nil:
		return func() tea.Msg { return done{err: err} }
	case hours == c.before:
		return nil
	}

	method, path, body := http.MethodPost, "/timesheets", map[string]any{"date": c.date, "durationHours": hours}

	switch {
	case c.entry != 0:
		method, path, body = http.MethodPut, fmt.Sprintf("/timesheets/%d", c.entry), map[string]any{"durationHours": hours}
	case c.project != nil:
		body["projectId"] = *c.project
	}

	return func() tea.Msg {
		if err := s.api.call(ctx, method, path, body, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("msg.booked", "Time booked")}
	}
}
