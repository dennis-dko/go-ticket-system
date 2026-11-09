package cli

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// subLeave names the leave's menu, opened from the calendar's own.
const subLeave = "leave"

// leaveYear is a year of the account's leave as /me/vacation answers it: what
// the year gives, what the year before left over, what is taken and planned and
// what is left, and the days a year given from which year on.
type leaveYear struct {
	Year         int           `json:"year"`
	Entitlement  float64       `json:"entitlement"`
	CarriedOver  float64       `json:"carriedOver"`
	Taken        float64       `json:"taken"`
	Planned      float64       `json:"planned"`
	Remaining    float64       `json:"remaining"`
	Entitlements []entitlement `json:"entitlements"`
}

// entitlement is how many days of leave a year gives, from a year on.
type entitlement struct {
	FromYear int     `json:"fromYear"`
	Days     float64 `json:"days"`
}

// leaveRead is a year of leave read on its own, for a card moved to it.
type leaveRead struct {
	year leaveYear
	err  error
}

// readLeave reads a year of the account's leave.
func (s *session) readLeave(ctx context.Context, year int) (leaveYear, error) {
	var leave leaveYear

	err := s.api.call(ctx, http.MethodGet, "/me/vacation?"+url.Values{"year": {strconv.Itoa(year)}}.Encode(), nil, &leave)

	return leave, err
}

// leaveYearShown is the year the leave shows: the one it was moved to, or this
// one in the owner's zone, which is not the machine's for part of the last day of
// it.
func (m screen) leaveYearShown() int {
	if m.cal.leaveShown != 0 {
		return m.cal.leaveShown
	}

	return m.today().Year()
}

// stepYear moves the leave a year back or on, and reads the year it moves to.
func (m screen) stepYear(years int) (tea.Model, tea.Cmd) {
	shown := m.leaveYearShown() + years
	if shown == m.today().Year() {
		shown = 0
	}

	m.cal.leaveShown = shown

	return m, m.readLeaveNow()
}

// readLeaveNow reads the year of leave shown, without the rest of the day.
func (m screen) readLeaveNow() tea.Cmd {
	ctx, s, year := m.ctx, m.s, m.leaveYearShown()

	return func() tea.Msg {
		leave, err := s.readLeave(ctx, year)

		return leaveRead{year: leave, err: err}
	}
}

// tookLeave keeps a year of leave read, where it is still the year shown.
func (m screen) tookLeave(msg leaveRead) screen {
	switch {
	case msg.err != nil:
		m.stale = msg.err.Error()
	case msg.year.Year == m.leaveYearShown():
		m.cal.leave = msg.year
	}

	return m
}

// leaveChoices are the leave's menu: giving days a year from a year on and taking
// such a gift back, for an account that may set its own settings - the right the
// page's form asks for.
func (m screen) leaveChoices() []choice {
	l := m.s.lang

	if !slices.Contains(m.shown.permissions, model.PermSettingsWriteOwn) {
		return nil
	}

	choices := []choice{{"n", "leaveNew", l.text("track.leave.give", "Give days of leave a year")}}

	if len(m.cal.leave.Entitlements) > 0 {
		choices = append(choices, choice{"d", "leaveDelete", l.text("track.leave.delete", "Take back days given a year")})
	}

	return choices
}

// leaveSteps ask how many days of leave a year gives, and from which year on.
func (m screen) leaveSteps(a *answers) []*huh.Group {
	l := m.s.lang

	return []*huh.Group{
		huh.NewGroup(huh.NewInput().Key("leaveDays").Title(l.text("track.ask.leaveDays", "How many days of leave does a year give?")).
			Validate(l.readableLeaveDays).Value(&a.leaveDays)),
		huh.NewGroup(huh.NewInput().Key("leaveYear").Title(l.text("track.ask.leaveYear", "From which year on?")).
			Validate(l.readableYear).Value(&a.leaveYear)),
	}
}

// leaveDays reads days of leave as the page's field takes them: whole or half
// days, from 0 to 366, with a point or a comma.
func leaveDays(raw string) (float64, bool) {
	days, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(raw), ",", "."), 64)

	return days, err == nil && days >= 0 && days <= 366 && math.Mod(days*2, 1) == 0
}

// readableLeaveDays refuses days of leave leaveDays cannot read.
func (l language) readableLeaveDays(raw string) error {
	if _, ok := leaveDays(raw); !ok {
		return errors.New(l.text("track.leave.badDays", "Write the days as whole or half days, from 0 to 366."))
	}

	return nil
}

// readableYear refuses a year the page's field would not take: 1900 to 9999.
func (l language) readableYear(raw string) error {
	if year, err := strconv.Atoi(strings.TrimSpace(raw)); err != nil || year < 1900 || year > 9999 {
		return errors.New(l.text("track.leave.badYear", "Write a year from 1900 to 9999."))
	}

	return nil
}

// saveLeave gives days of leave a year from a year on, as the page's form does.
func (m screen) saveLeave() tea.Cmd {
	ctx, s, a := m.ctx, m.s, *m.asked

	days, _ := leaveDays(a.leaveDays)
	year, _ := strconv.Atoi(strings.TrimSpace(a.leaveYear))
	body := map[string]any{"fromYear": year, "days": days}

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodPut, "/me/vacation", body, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("vacation.saved", "Leave saved")}
	}
}

// leaveDeleteStep asks before days given a year are taken back, naming them as
// the page's delete button does.
func (m screen) leaveDeleteStep(a *answers) *huh.Group {
	l := m.s.lang
	named := l.text("vacation.from", "from {0}", strconv.Itoa(m.cal.chosenLeave.FromYear))

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("confirm.deleteTitle", "Delete for good?")).
		Description(named + " " + l.text("confirm.deleteText", "will be deleted. This cannot be undone.")).
		Affirmative(l.text("track.form.deleteYes", "Delete")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// deleteLeave takes back the days given from the year picked, once the answer
// was yes.
func (m screen) deleteLeave() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, path := m.ctx, m.s, "/me/vacation/"+strconv.Itoa(m.cal.chosenLeave.FromYear)

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodDelete, path, nil, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("vacation.withdrawn", "Entitlement taken back")}
	}
}

// leaveCardView draws a year's leave as the page's card does: headed by the year,
// what is left of it - or that nothing is given for it yet - what makes it up,
// and the days a year given from which year on. While one of those is picked the
// cursor stands before it.
func (m screen) leaveCardView(look palette, width int) string {
	l, leave := m.s.lang, m.cal.leave

	if leave.Year != m.leaveYearShown() {
		return look.muted.Render(l.text("track.leave.reading", "Reading the leave …"))
	}

	lines := []string{look.title.Render(l.text("vacation.title", "Leave {0}", strconv.Itoa(leave.Year)))}

	// Given once an entitlement reaches back to the year, as the page decides it.
	if given := len(leave.Entitlements) > 0 && leave.Year >= leave.Entitlements[0].FromYear; given {
		lines = append(lines, l.text("vacation.left", "Days left: {0}", l.filled(leave.Remaining)),
			look.muted.Render(l.text("vacation.detail", "Given: {0} · carried over: {1} · taken: {2} · planned: {3}",
				l.filled(leave.Entitlement), l.filled(leave.CarriedOver), l.filled(leave.Taken), l.filled(leave.Planned))))
	} else {
		lines = append(lines, l.text("vacation.none",
			"No leave given for this year yet: say below how many days a year give."))
	}

	if len(leave.Entitlements) == 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("vacation.noEntitlements", "No days of leave given yet."))), "\n")
	}

	table := grid{
		headings: []string{l.text("vacation.fromYear", "From the year"), l.text("vacation.days", "Days of leave a year")},
		right:    []bool{false, true},
	}

	for _, given := range leave.Entitlements {
		table.rows = append(table.rows, []string{strconv.Itoa(given.FromYear), l.number(given.Days)})
	}

	inner := width - 2
	lines = append(lines, "  "+table.heading(look, inner))

	for i, line := range table.lines(inner) {
		cursor := " "
		if m.picking == "leaveDelete" && i == m.row {
			cursor = look.title.Render(">")
		}

		lines = append(lines, cursor+" "+ansi.Truncate(line, inner, "…"))
	}

	return strings.Join(lines, "\n")
}
