package cli

import (
	"errors"
	"net/http"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// The stopwatch card's corrections, as the page offers them under its clock:
// the start corrected, an end given for a clock stopped later than the work,
// and the measured time discarded. A day and a time are the owner's wall clock,
// sent without an offset as the page sends them, because the installation reads
// such a moment in the owner's own zone.

// forgotten reports whether the running stopwatch has run longer than one entry
// can hold - almost always one somebody forgot. The installation books it only
// up to an end, and refuses both a stop now and a switch.
func (m screen) forgotten() bool {
	return m.shown.clock.Running && m.elapsed() > model.HoursPerDay*time.Hour
}

// clockChoices are the stopwatch card's menu, while it runs.
func (m screen) clockChoices() []choice {
	l := m.s.lang

	if !m.shown.clock.Running {
		return nil
	}

	return []choice{
		{"c", "adjust", l.text("track.clock.adjust", "Correct the start")},
		{"e", "end", l.text("track.clock.end", "Stopped earlier")},
		{"d", "discard", l.text("timer.discard", "Discard")},
	}
}

// clockAnswers fill the day and the time a correction asks for: the start where
// the start is corrected, and for an end the day the work most likely ended on -
// the day a forgotten clock was started, today for one stopped a little late -
// with the time left for whoever knows it, as the page leaves it.
func (m screen) clockAnswers(doing string) *answers {
	a := &answers{}
	zone := m.shown.zone

	switch {
	case doing == "adjust" && m.shown.clock.StartedAt != nil:
		at := m.shown.clock.StartedAt.In(zone)
		a.clockDay, a.clockTime = at.Format(time.DateOnly), at.Format("15:04")
	case doing == "end" && m.forgotten() && m.shown.clock.StartedAt != nil:
		a.clockDay = m.shown.clock.StartedAt.In(zone).Format(time.DateOnly)
	default:
		a.clockDay = m.now.In(zone).Format(time.DateOnly)
	}

	return a
}

// clockSteps ask for a day and a time on the owner's clock: when the work
// started, or when it ended - saying why where the clock was forgotten.
func (m screen) clockSteps(doing string, a *answers) []*huh.Group {
	l := m.s.lang

	question := l.text("timer.adjustTitle", "When did the work start?")
	if doing == "end" {
		question = l.text("timer.endTitle", "When did the work end?")
	}

	day := huh.NewInput().Key("clockDay").Title(question).
		Placeholder(l.text("track.form.dayHint", "YYYY-MM-DD")).Validate(l.readableDay).Value(&a.clockDay)

	if doing == "end" && m.forgotten() {
		day = day.Description(l.text("timer.endForgotten", "The clock has been running for longer than one entry can hold - probably since it was forgotten. Say when the work ended and it is booked up to then, on the day it started."))
	}

	return []*huh.Group{
		huh.NewGroup(day),
		huh.NewGroup(huh.NewInput().Key("clockTime").Title(l.text("track.ask.time", "At what time?")).
			Placeholder("HH:MM").Validate(l.readableTime).Value(&a.clockTime)),
	}
}

// discardStep asks before the measured time is thrown away, in the page's words.
func (m screen) discardStep(a *answers) *huh.Group {
	l := m.s.lang

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("timer.discardTitle", "Discard the measured time?")).
		Description(l.text("timer.discardText", "The clock is stopped and nothing is recorded. This cannot be undone.")).
		Affirmative(l.text("timer.discard", "Discard")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// clockChange is the correction an answered form asks for.
func (m screen) clockChange() tea.Cmd {
	ctx, s, a, projects := m.ctx, m.s, *m.asked, m.shown.projects
	moment := strings.TrimSpace(a.clockDay) + "T" + strings.TrimSpace(a.clockTime)

	switch m.doing {
	case "adjust":
		return func() tea.Msg {
			if err := s.api.call(ctx, http.MethodPut, "/me/timer", map[string]string{"startedAt": moment}, nil); err != nil {
				return done{err: err}
			}

			return done{said: s.lang.text("timer.adjusted", "Start corrected")}
		}
	case "end":
		return func() tea.Msg {
			said, err := s.stopClock(ctx, projects, moment)

			return done{said: said, err: err}
		}
	case "discard":
		if !a.confirmed {
			return nil
		}

		return func() tea.Msg {
			if err := s.api.call(ctx, http.MethodDelete, "/me/timer", nil, nil); err != nil {
				return done{err: err}
			}

			return done{said: s.lang.text("timer.discarded", "The clock was discarded")}
		}
	}

	return nil
}

// readableTime refuses a time of day written other than as HH:MM.
func (l language) readableTime(raw string) error {
	if _, err := time.Parse("15:04", strings.TrimSpace(raw)); err != nil {
		return errors.New(l.text("track.form.timeHint", "write the time as HH:MM, such as 08:45"))
	}

	return nil
}
