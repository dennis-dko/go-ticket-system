package cli

import (
	"charm.land/lipgloss/v2"
)

// timesheetChoices are the time entries' menu: the changes the stopwatch can
// take as it stands, and the menu of the entries card. A change it cannot take -
// stopping one that is not running - is not offered, and its key does nothing.
func (m screen) timesheetChoices() []choice {
	l := m.s.lang

	entries := choice{"e", "menu:" + subEntries, l.text("ts.entries", "Entries") + " …"}

	if m.shown.clock.Running {
		return []choice{
			{"w", "switch", l.text("track.form.switch", "Switch to another project")},
			{"x", "stop", l.text("track.form.stop", "Stop the stopwatch")},
			{"b", "book", l.text("track.form.book", "Book time by hand")},
			entries,
		}
	}

	return []choice{
		{"s", "start", l.text("track.form.start", "Start the stopwatch")},
		{"b", "book", l.text("track.form.book", "Book time by hand")},
		entries,
	}
}

// timesheetBody is what the time entries show above their menu: the stopwatch,
// and once the day is read the entries card with rows of its entries.
func (m screen) timesheetBody(look palette, width, rows int) []string {
	shown := []string{m.stopwatch(look, width)}

	if m.loaded {
		shown = append(shown, m.entryCardView(look, width, rows))
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
