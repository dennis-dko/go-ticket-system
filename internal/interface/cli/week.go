package cli

import (
	"context"
	"net/http"
	"net/url"
	"time"

	tea "charm.land/bubbletea/v2"
)

// weekCard is the week grid as it was read: the Monday of the week it shows, as
// it travels, every entry booked in that week, and what each of its days asks.
type weekCard struct {
	monday  string
	entries []entry
	days    []weekDay
}

// weekDay is one day as /me/days answers it: what it asks, why it may ask less -
// a day away or a public holiday - and whether it counts towards the balance.
type weekDay struct {
	Date              string  `json:"date"`
	Target            float64 `json:"target"`
	Absence           string  `json:"absence"`
	Holiday           string  `json:"holiday"`
	HolidaySubstitute bool    `json:"holidaySubstitute"`
	Counts            bool    `json:"counts"`
}

// weekRead is a week read on its own, for a grid moved to it.
type weekRead struct {
	card weekCard
	err  error
}

// readWeek reads the week from a Monday on.
func (s *session) readWeek(ctx context.Context, monday string) (weekCard, error) {
	entries, days, err := s.readDays(ctx, monday, addDays(monday, 6))
	if err != nil {
		return weekCard{}, err
	}

	return weekCard{monday: monday, entries: entries, days: days}, nil
}

// readDays reads the days from one day to another as the page's week and
// calendar read them: every entry booked on them, however many pages the list
// answers them in, and what each day asks. The page reads the list to its end
// for the reason the calendar learnt it - a month of a hundred and one entries
// totalled from its first page lost its first days.
func (s *session) readDays(ctx context.Context, from, to string) ([]entry, []weekDay, error) {
	var entries []entry

	for {
		page, err := s.entryList(ctx, entryFilter{from: from, to: to}, len(entries), listMost)
		if err != nil {
			return nil, nil, err
		}

		entries = append(entries, page.Items...)

		if len(page.Items) == 0 || len(entries) >= page.TotalCount {
			break
		}
	}

	var answer struct {
		Days []weekDay `json:"days"`
	}

	q := url.Values{"from": {from}, "to": {to}}
	if err := s.api.call(ctx, http.MethodGet, "/me/days?"+q.Encode(), nil, &answer); err != nil {
		return nil, nil, err
	}

	return entries, answer.Days, nil
}

// today is the owner's day by this machine's clock, in the zone they keep their
// days in.
func (m screen) today() time.Time {
	now, zone := m.now, m.shown.zone

	if now.IsZero() {
		now = time.Now()
	}

	if zone == nil {
		zone = time.Local
	}

	return now.In(zone)
}

// mondayOf is the Monday of the week a day falls in, as it travels.
func mondayOf(day time.Time) string {
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7).Format(time.DateOnly)
}

// addDays is a day as it travels, moved by days on the calendar.
func addDays(day string, days int) string {
	at, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return day
	}

	return at.AddDate(0, 0, days).Format(time.DateOnly)
}

// weekMonday is the Monday of the week the grid shows: the one it was moved to,
// or this week's - which is the week of a screen left open over a weekend once
// the new one has begun, as long as nobody moved it.
func (m screen) weekMonday() string {
	if m.weekShown != "" {
		return m.weekShown
	}

	return mondayOf(m.today())
}

// stepWeek moves the grid weeks back or on, or to this week where weeks is
// zero, and reads the week it moves to.
func (m screen) stepWeek(weeks int) (tea.Model, tea.Cmd) {
	shown := ""
	if weeks != 0 {
		shown = addDays(m.weekMonday(), 7*weeks)
	}

	// This week is followed rather than named, wherever the grid came back to it
	// from.
	if shown == mondayOf(m.today()) {
		shown = ""
	}

	m.weekShown = shown

	return m, m.readWeekNow()
}

// readWeekNow reads the week the grid shows, without the rest of the day.
func (m screen) readWeekNow() tea.Cmd {
	ctx, s, monday := m.ctx, m.s, m.weekMonday()

	return func() tea.Msg {
		card, err := s.readWeek(ctx, monday)

		return weekRead{card: card, err: err}
	}
}

// tookWeek keeps a week read on its own, where it is still the week shown: one
// read for a week the grid has moved on from belongs to a grid that is gone.
func (m screen) tookWeek(msg weekRead) screen {
	switch {
	case msg.err != nil:
		m.stale = msg.err.Error()
	case msg.card.monday == m.weekMonday():
		m.week = msg.card
	}

	return m
}

// moveKey answers the keys that move the week or the calendar on the tab on
// screen: < back, > on, and t to the week or month today is in, once the tab
// shows another. ok is false for any other key, and on any other tab.
func (m screen) moveKey(key string) (tea.Model, tea.Cmd, bool) {
	step, away := m.stepWeek, m.weekShown != ""

	switch m.currentTab().view {
	case viewWeek:
	case viewCalendar:
		step, away = m.stepMonth, m.cal.shown != ""

		// The leave's card has arrows of its own, for its years.
		if m.sub == subLeave {
			step, away = m.stepYear, false
		}
	default:
		return m, nil, false
	}

	var (
		next tea.Model
		cmd  tea.Cmd
	)

	switch {
	case key == "<":
		next, cmd = step(-1)
	case key == ">":
		next, cmd = step(1)
	case key == "t" && away:
		next, cmd = step(0)
	default:
		return m, nil, false
	}

	return next, cmd, true
}

// weekBody is what the week shows above its menu: its grid, once the week moved
// to has been read.
func (m screen) weekBody(look palette, width int) []string {
	if m.week.monday != m.weekMonday() {
		return []string{look.muted.Render(m.s.lang.text("track.week.reading", "Reading the week …"))}
	}

	return []string{m.weekCardView(look, width)}
}
