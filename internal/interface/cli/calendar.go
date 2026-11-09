package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// calendarState is the calendar's tab: the month as it was last read, and the
// first of the month it was moved to - empty for this month, which it then
// follows.
type calendarState struct {
	month calendarCard
	shown string

	// day is the day open under the month, and cursor the one moved to while one
	// is picked.
	day, cursor string

	// picked is the day of the working time picked to be corrected or taken back.
	picked workDay

	// leave is the year of leave as it was last read, leaveShown the year it was
	// moved to - zero for this year - and chosenLeave the days given a year picked
	// to be taken back.
	leave       leaveYear
	leaveShown  int
	chosenLeave entitlement
}

// calendarCard is the month as it was read: its first day, as it travels, every
// entry booked in it, what each of its days is besides, and its working time.
type calendarCard struct {
	first   string
	entries []entry
	days    []weekDay
	work    workMonth
}

// monthRead is a month read on its own, for a calendar moved to it.
type monthRead struct {
	card calendarCard
	err  error
}

// readMonth reads the month from its first day to its last, and its working time
// with it, which follows the calendar's month as the page's card does.
func (s *session) readMonth(ctx context.Context, first string) (calendarCard, error) {
	entries, days, err := s.readDays(ctx, first, lastOfMonth(first))
	if err != nil {
		return calendarCard{}, err
	}

	work, err := s.readWork(ctx, first)
	if err != nil {
		return calendarCard{}, err
	}

	return calendarCard{first: first, entries: entries, days: days, work: work}, nil
}

// firstOfMonth is the first of the month a day falls in, as it travels.
func firstOfMonth(day time.Time) string {
	return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
}

// lastOfMonth is the last day of the month from its first.
func lastOfMonth(first string) string {
	at, err := time.Parse(time.DateOnly, first)
	if err != nil {
		return first
	}

	return at.AddDate(0, 1, -1).Format(time.DateOnly)
}

// calendarMonth is the first of the month the calendar shows: the one it was
// moved to, or this one.
func (m screen) calendarMonth() string {
	if m.cal.shown != "" {
		return m.cal.shown
	}

	return firstOfMonth(m.today())
}

// stepMonth moves the calendar months back or on, or to this month where months
// is zero, and reads the month it moves to.
func (m screen) stepMonth(months int) (tea.Model, tea.Cmd) {
	shown := ""

	if months != 0 {
		if at, err := time.Parse(time.DateOnly, m.calendarMonth()); err == nil {
			shown = at.AddDate(0, months, 0).Format(time.DateOnly)
		}
	}

	// This month is followed rather than named, wherever the calendar came back
	// to it from.
	if shown == firstOfMonth(m.today()) {
		shown = ""
	}

	// A day open under the month goes with it.
	m.cal.shown, m.cal.day = shown, ""

	return m, m.readMonthNow()
}

// readMonthNow reads the month the calendar shows, without the rest of the day.
func (m screen) readMonthNow() tea.Cmd {
	ctx, s, first := m.ctx, m.s, m.calendarMonth()

	return func() tea.Msg {
		card, err := s.readMonth(ctx, first)

		return monthRead{card: card, err: err}
	}
}

// tookMonth keeps a month read on its own, where it is still the month shown.
func (m screen) tookMonth(msg monthRead) screen {
	switch {
	case msg.err != nil:
		m.stale = msg.err.Error()
	case msg.card.first == m.calendarMonth():
		m.cal.month = msg.card
	}

	return m
}

// calendarBody is what the calendar shows above its menu: the month, once the
// month moved to has been read.
func (m screen) calendarBody(look palette, width int) []string {
	if m.cal.month.first != m.calendarMonth() {
		return []string{look.muted.Render(m.s.lang.text("track.cal.reading", "Reading the month …"))}
	}

	switch {
	case m.sub == subWork:
		return []string{m.workCardView(look, width)}
	case m.sub == subLeave:
		return []string{m.leaveCardView(look, width)}
	case m.cal.day != "":
		return []string{m.monthView(look, width), m.dayCardView(look, width)}
	}

	return []string{m.monthView(look, width)}
}

// calendarCell is how wide a day of the month is drawn: its number, its mark and
// its hours. With a space between two, a week is seventy columns, which a
// terminal eighty wide holds inside the frame.
const calendarCell = 9

// monthView draws the month as the page draws its calendar: headed by its name
// and year, its days under the weekdays from Monday, each with the hours booked
// on it, today in the accent. A public holiday or a day away is marked in its
// cell and named under the grid, since a cell has no room for the page's words,
// and the month's total is said last.
func (m screen) monthView(look palette, width int) string {
	l, month := m.s.lang, m.cal.month

	first, err := time.Parse(time.DateOnly, month.first)
	if err != nil {
		return ""
	}

	lines := []string{look.title.Render(l.monthName(first) + " " + strconv.Itoa(first.Year()))}

	var head []string
	for _, name := range l.weekdayNames() {
		head = append(head, fitLeft(name, calendarCell))
	}

	lines = append(lines, look.muted.Render(strings.TrimRight(strings.Join(head, " "), " ")))

	booked, total := map[string]float64{}, 0.0
	for _, e := range month.entries {
		booked[e.Date] += e.DurationHours
		total += e.DurationHours
	}

	today := m.today().Format(time.DateOnly)
	last := first.AddDate(0, 1, -1).Day()

	// Weeks start on Monday, so a month beginning on a Sunday leaves six days
	// before it.
	week := make([]string, (int(first.Weekday())+6)%7)
	for i := range week {
		week[i] = strings.Repeat(" ", calendarCell)
	}

	for d := 1; d <= last; d++ {
		date := first.AddDate(0, 0, d-1).Format(time.DateOnly)
		day := month.dayOn(date)

		mark, hours := " ", ""
		if day.Holiday != "" || day.Absence != "" {
			mark = "•"
		}

		if booked[date] > 0 {
			hours = l.share(booked[date])
		}

		// The hours beside their own day rather than against the next one, and a
		// day nothing is booked on quiet, as the page draws only those with entries
		// as something to press. The day being picked is filled with the accent,
		// and today is written in it.
		cell := fmt.Sprintf("%2d%s %-5s", d, mark, hours)

		switch {
		case m.picking == "showDay" && date == m.cal.cursor:
			cell = look.tabOn.Render(cell)
		case date == today:
			cell = look.title.Render(cell)
		case hours == "":
			cell = look.muted.Render(cell)
		}

		if week = append(week, cell); len(week) == 7 || d == last {
			lines = append(lines, ansi.Truncate(strings.TrimRight(strings.Join(week, " "), " "), width, "…"))
			week = nil
		}
	}

	if marks := m.monthMarks(first, last); marks != "" {
		lines = append(lines, lipgloss.NewStyle().Width(width).Render(look.muted.Render(marks)))
	}

	return strings.Join(append(lines, l.text("cal.monthTotal", "Month total")+": "+l.hours(total)), "\n")
}

// monthMarks names the month's public holidays and days away under its grid, a
// run of days that are the same thing - a fortnight of leave - as one range.
func (m screen) monthMarks(first time.Time, last int) string {
	l, month := m.s.lang, m.cal.month

	type run struct {
		from, to time.Time
		said     string
	}

	var runs []run

	for d := range last {
		at := first.AddDate(0, 0, d)
		day := month.dayOn(at.Format(time.DateOnly))

		var said []string

		if day.Holiday != "" {
			said = append(said, l.holidayOf(day))
		}

		if day.Absence != "" {
			said = append(said, l.absenceName(day.Absence))
		}

		if len(said) == 0 {
			continue
		}

		what := strings.Join(said, ", ")

		if n := len(runs); n > 0 && runs[n-1].said == what && runs[n-1].to.AddDate(0, 0, 1).Equal(at) {
			runs[n-1].to = at

			continue
		}

		runs = append(runs, run{from: at, to: at, said: what})
	}

	named := make([]string, len(runs))

	for i, r := range runs {
		days := l.dayAndMonth(r.from)
		if !r.to.Equal(r.from) {
			days += "–" + l.dayAndMonth(r.to)
		}

		named[i] = days + ": " + r.said
	}

	return strings.Join(named, " · ")
}

// dayOn is what the month's days say about a day, empty for one they do not name.
func (c calendarCard) dayOn(date string) weekDay {
	return weekCard{days: c.days}.dayOn(date)
}

// monthName is what the calendar calls a month, as the page's calendar calls it.
func (l language) monthName(at time.Time) string {
	names := strings.Split(l.text("cal.months",
		"January,February,March,April,May,June,July,August,September,October,November,December"), ",")

	if len(names) != 12 {
		return at.Month().String()
	}

	return names[at.Month()-1]
}

// subWork names the working time's menu, opened from the calendar's own.
const subWork = "work"
