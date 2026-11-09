package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// timesheetChoices are the time entries' menu: the changes the stopwatch can
// take as it stands. A change it cannot take - stopping one that is not running
// - is not offered, and its key does nothing.
func (m screen) timesheetChoices() []choice {
	l := m.s.lang

	if m.shown.clock.Running {
		return []choice{
			{"w", "switch", l.text("track.form.switch", "Switch to another project")},
			{"x", "stop", l.text("track.form.stop", "Stop the stopwatch")},
			{"b", "book", l.text("track.form.book", "Book time by hand")},
		}
	}

	return []choice{
		{"s", "start", l.text("track.form.start", "Start the stopwatch")},
		{"b", "book", l.text("track.form.book", "Book time by hand")},
	}
}

// timesheetBody is what the time entries show above their menu: the stopwatch,
// and once the day is read what today has booked.
func (m screen) timesheetBody(look palette, width int) []string {
	shown := []string{m.stopwatch(look, width)}

	if m.loaded {
		shown = append(shown, m.todayList(look, width))
	}

	return shown
}

// stopwatch is what the stopwatch runs on, since when, and for how long.
func (m screen) stopwatch(look palette, width int) string {
	l := m.s.lang

	switch {
	case !m.loaded && m.reading:
		return look.muted.Render(l.text("track.reading", "Reading {0} …", m.host))
	case !m.loaded:
		return look.muted.Render(l.text("track.nothingRead", "Nothing read from {0} yet.", m.host))
	}

	clock := m.shown.clock
	if !clock.Running || clock.StartedAt == nil {
		return lipgloss.NewStyle().Foreground(look.idle).Render("○  ") + l.text("track.notRunning", "The stopwatch is not running.")
	}

	on := l.nameOf(m.shown.projects, clock.ProjectID)
	if clock.Description != nil && *clock.Description != "" {
		on += look.muted.Render(" · " + *clock.Description)
	}

	left := lipgloss.NewStyle().Foreground(look.running).Render("●  ") + look.strong.Render(clockFace(m.elapsed())) + "   " + on
	right := look.muted.Render(l.text("track.since", "since {0}", clock.StartedAt.In(m.shown.zone).Format("15:04")))

	return spread(width, left, right)
}

// todayList is what today has booked, entry by entry, under the day and its
// total.
func (m screen) todayList(look palette, width int) string {
	l := m.s.lang
	entries := m.shown.today
	day := l.weekday(m.readAt.In(m.shown.zone))

	heading := spread(width, look.strong.Render(day), look.strong.Render(l.hours(total(entries))))

	if len(entries) == 0 {
		return heading + "\n" + look.muted.Render(l.text("track.nothingYet", "Nothing booked yet."))
	}

	names := make([]string, len(entries))
	column := 0

	for i, e := range entries {
		names[i] = l.nameOf(m.shown.projects, e.ProjectID)
		column = max(column, lipgloss.Width(names[i]))
	}

	column = min(column, width/2)
	rows := []string{heading}
	hoursWidth := hoursColumn(l.hoursOf(entries)...)

	for i, e := range entries {
		name := ansi.Truncate(names[i], column, "…")
		row := fmt.Sprintf("   %*s  %s", hoursWidth, l.hours(e.DurationHours),
			name+strings.Repeat(" ", column-lipgloss.Width(name)))

		if e.Description != nil && *e.Description != "" {
			row += "  " + look.muted.Render(*e.Description)
		}

		rows = append(rows, ansi.Truncate(row, width, "…"))
	}

	return strings.Join(rows, "\n")
}
