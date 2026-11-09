package cli

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// calendarChoices are the calendar's menu: with a day open, the changes the
// page's day card offers on its entries and closing it; and showing a day, in a
// month with something booked in it, as the page offers only such days to press.
// Nothing while the month moved to is still being read.
func (m screen) calendarChoices() []choice {
	l := m.s.lang

	if m.cal.month.first != m.calendarMonth() {
		return nil
	}

	var choices []choice

	if m.cal.day != "" {
		entries := m.dayEntries()

		if m.mayWrite() && len(entries) > 0 {
			choices = append(choices,
				choice{"e", "edit", l.text("track.list.edit", "Correct an entry")},
				choice{"d", "delete", l.text("track.list.delete", "Delete an entry")})
		}

		if slices.ContainsFunc(entries, entry.revised) {
			choices = append(choices, choice{"h", "history", l.text("track.list.history", "Earlier versions of an entry")})
		}

		choices = append(choices, choice{"c", "closeDay", l.text("track.cal.close", "Close the day")})
	}

	if len(m.bookedDays()) > 0 {
		choices = append(choices, choice{"s", "showDay", l.text("track.cal.show", "Show a day")})
	}

	return append(choices, choice{"z", "menu:" + subWork, l.text("work.title", "Working time") + " …"},
		choice{"u", "menu:" + subLeave, l.text("track.leave.title", "Leave") + " …"})
}

// bookedDays are the days of the month shown that something is booked on.
func (m screen) bookedDays() map[string]bool {
	booked := map[string]bool{}
	for _, e := range m.cal.month.entries {
		booked[e.Date] = true
	}

	return booked
}

// dayEntries are the entries of the day open under the month.
func (m screen) dayEntries() []entry {
	var on []entry

	for _, e := range m.cal.month.entries {
		if e.Date == m.cal.day {
			on = append(on, e)
		}
	}

	return on
}

// startDay puts the cursor on a day to show: today where something is booked on
// it, and otherwise the month's first day that has something.
func (m screen) startDay() (tea.Model, tea.Cmd) {
	booked := m.bookedDays()

	days := make([]string, 0, len(booked))
	for day := range booked {
		days = append(days, day)
	}

	if len(days) == 0 {
		return m, nil
	}

	slices.Sort(days)

	m.picking, m.news, m.failure, m.cal.cursor = "showDay", "", "", days[0]

	if today := m.today().Format(time.DateOnly); booked[today] {
		m.cal.cursor = today
	}

	return m, nil
}

// dayKey answers a key while a day is being picked: the arrows move a day or a
// week, no further than the month's ends, enter opens a day something is booked
// on - one with nothing on it has nothing to show - and escape puts the picking
// away.
func (m screen) dayKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	at, err := time.Parse(time.DateOnly, m.cal.cursor)
	first, firstErr := time.Parse(time.DateOnly, m.cal.month.first)

	if err != nil || firstErr != nil {
		m.picking = ""

		return m, nil
	}

	by := 0

	switch k.String() {
	case "esc":
		m.picking = ""

		return m, nil
	case "enter":
		if m.bookedDays()[m.cal.cursor] {
			m.picking, m.cal.day = "", m.cal.cursor
		}

		return m, nil
	case "left":
		by = -1
	case "right":
		by = 1
	case "up", "k":
		by = -7
	case "down", "j":
		by = 7
	}

	next := at.AddDate(0, 0, by)
	last := first.AddDate(0, 1, -1)

	switch {
	case next.Before(first):
		next = first
	case next.After(last):
		next = last
	}

	m.cal.cursor = next.Format(time.DateOnly)

	return m, nil
}

// dayCardView draws the open day as the page's day card does: headed by its date,
// its entries' project, hours and description under the page's columns, and who
// booked or changed one where that was somebody else. While an entry is picked
// the cursor stands before it, and one that cannot be picked is drawn quietly.
func (m screen) dayCardView(look palette, width int) string {
	l, entries := m.s.lang, m.dayEntries()
	lines := []string{look.title.Render(l.date(m.cal.day))}

	if len(entries) == 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("ot.empty", "No bookings in this period."))), "\n")
	}

	table := grid{
		headings: []string{l.text("field.project", "Project"), l.text("field.hours", "Hours"),
			l.text("field.description", "Description")},
		right: []bool{false, true},
		most:  []int{3},
	}

	for _, e := range entries {
		table.rows = append(table.rows, m.entryCells(look, e)[1:])
	}

	inner := width - 2
	lines = append(lines, "  "+table.heading(look, inner))

	for i, line := range table.lines(inner) {
		cursor := " "

		if m.picking != "" && i == m.row {
			cursor = look.title.Render(">")
		}

		if m.picking != "" && !m.pickable(entries[i]) {
			line = look.muted.Render(ansi.Strip(line))
		}

		lines = append(lines, cursor+" "+line)
	}

	return strings.Join(lines, "\n")
}
