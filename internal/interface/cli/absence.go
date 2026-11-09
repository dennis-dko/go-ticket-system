package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// awayRead is the year's days away read on their own, for a calendar moved to.
type awayRead struct {
	days []absence
	err  error
}

// readAwayNow reads the year's days away, without the rest of the day.
func (m screen) readAwayNow() tea.Cmd {
	ctx, s := m.ctx, m.s

	return func() tea.Msg {
		days, err := s.readAway(ctx)

		return awayRead{days: days, err: err}
	}
}

// tookAway keeps the days away read on their own.
func (m screen) tookAway(msg awayRead) screen {
	if msg.err != nil {
		m.stale = msg.err.Error()

		return m
	}

	m.cal.away = msg.days

	return m
}

// subAway names the days away's menu, opened from the calendar's own.
const subAway = "away"

// absence is a day away and why, as /me/absences answers it.
type absence struct {
	Day  string `json:"day"`
	Kind string `json:"kind"`
}

// absenceKinds are the reasons a day away can be recorded for, in the order the
// page's form offers them.
var absenceKinds = []string{"vacation", "halfVacation", "sick", "flexDay", "localHoliday"}

// readAway reads the days away of the year today falls in, which is what the
// page's card lists.
func (s *session) readAway(ctx context.Context) ([]absence, error) {
	var listed struct {
		Items []absence `json:"items"`
	}

	err := s.api.call(ctx, http.MethodGet, "/me/absences", nil, &listed)

	return listed.Items, err
}

// awayChoices are the days away's menu: recording days over a range, and taking
// back one of them.
func (m screen) awayChoices() []choice {
	l := m.s.lang
	choices := []choice{{"n", "awayNew", l.text("track.away.record", "Record days away")}}

	if len(m.cal.away) > 0 {
		choices = append(choices, choice{"d", "awayDelete", l.text("track.away.delete", "Take back a day away")})
	}

	return choices
}

// awaySteps ask for the first and the last day away and why, as the page's form
// asks.
func (m screen) awaySteps(a *answers) []*huh.Group {
	l := m.s.lang

	day := func(key, question string, into *string) *huh.Group {
		return huh.NewGroup(huh.NewInput().Key(key).Title(question).
			Placeholder(l.text("track.form.dayHint", "YYYY-MM-DD")).Validate(l.readableDay).Value(into))
	}

	kinds := make([]huh.Option[string], len(absenceKinds))
	for i, kind := range absenceKinds {
		kinds[i] = huh.NewOption(l.absenceName(kind), kind)
	}

	return []*huh.Group{
		day("awayFrom", l.text("track.ask.awayFrom", "From which day are you away?"), &a.awayFrom),
		day("awayTo", l.text("track.ask.awayTo", "Up to which day?"), &a.awayTo),
		huh.NewGroup(huh.NewSelect[string]().Key("awayKind").Title(l.text("track.ask.awayKind", "Why?")).
			Options(kinds...).Value(&a.awayKind)),
	}
}

// saveAway records the days away of a range, for one reason, and says how many
// the installation recorded - only the days the week asks something of are, so a
// fortnight of leave is its ten working days.
func (m screen) saveAway() tea.Cmd {
	ctx, s, a := m.ctx, m.s, *m.asked
	body := map[string]string{"from": strings.TrimSpace(a.awayFrom), "to": strings.TrimSpace(a.awayTo), "kind": a.awayKind}

	return func() tea.Msg {
		var recorded struct {
			TotalCount int `json:"totalCount"`
		}

		if err := s.api.call(ctx, http.MethodPost, "/me/absences", body, &recorded); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("absence.recorded", "Days recorded: {0}", strconv.Itoa(recorded.TotalCount))}
	}
}

// awayDeleteStep asks before a day away is taken back, naming it by its day and
// why, as the page's delete button does.
func (m screen) awayDeleteStep(a *answers) *huh.Group {
	l, away := m.s.lang, m.cal.chosenAway

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("confirm.deleteTitle", "Delete for good?")).
		Description(l.date(away.Day) + ", " + l.absenceName(away.Kind) + " " +
			l.text("confirm.deleteText", "will be deleted. This cannot be undone.")).
		Affirmative(l.text("track.form.deleteYes", "Delete")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// deleteAway takes back the day away picked, once the answer was yes.
func (m screen) deleteAway() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, path := m.ctx, m.s, "/me/absences/"+url.PathEscape(m.cal.chosenAway.Day)

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodDelete, path, nil, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("absence.removed", "Absence taken back")}
	}
}

// awayCardView draws the days away as the page's card does: what it records,
// and each day and why. While one is picked the cursor stands before it.
func (m screen) awayCardView(look palette, width int) string {
	l := m.s.lang

	lines := []string{look.title.Render(l.text("absence.title", "Days away")), lipgloss.NewStyle().Width(width).Render(look.muted.Render(l.text(
		"absence.hint", "Leave, illness and days taken against overtime. Only the days your week asks something of are recorded, so a fortnight of leave is its ten working days.")))}

	if len(m.cal.away) == 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("absence.empty", "No absence recorded this year."))), "\n")
	}

	table := grid{headings: []string{l.text("field.day", "Day"), l.text("field.kind", "Reason")}}
	for _, away := range m.cal.away {
		table.rows = append(table.rows, []string{l.date(away.Day), l.absenceName(away.Kind)})
	}

	inner := width - 2
	lines = append(lines, "  "+table.heading(look, inner))

	for i, line := range table.lines(inner) {
		cursor := " "
		if m.picking == "awayDelete" && i == m.row {
			cursor = look.title.Render(">")
		}

		lines = append(lines, cursor+" "+line)
	}

	return strings.Join(lines, "\n")
}
