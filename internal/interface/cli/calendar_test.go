package cli

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// monthHere is the first of the month today falls in where the fake
// installation's account keeps its days.
func monthHere(t *testing.T) time.Time {
	t.Helper()

	now := time.Now().In(mustZone(t, "Europe/Berlin"))

	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// onMonth is a screen on the calendar's tab, with the month from first read.
func onMonth(first string) func(screen) bool {
	return func(m screen) bool {
		return settled(m) && m.currentTab().view == viewCalendar && m.cal.month.first == first
	}
}

// calendarOpen moves to the calendar's tab once the day is read - two tabs on
// from the time entries - and waits for this month.
func (w *watched) calendarOpen(first time.Time) string {
	w.t.Helper()

	w.until("the time entries are on screen", onTab(viewTimesheets))
	w.press("right", "right")

	return w.until("the month is read", onMonth(first.Format(time.DateOnly)))
}

// calendarCells is what each day of the month grid on screen says, by the day of
// the month: its number, a • where the day is marked, and the hours booked on it,
// with the spaces between them made single. The grid's lines are those between
// the line of weekday names and the month's total; each day is a column of ten.
func calendarCells(shows string) map[int]string {
	cells := map[int]string{}
	inGrid := false

	for _, line := range strings.Split(shows, "\n") {
		border := strings.Index(line, "│")
		if border < 0 {
			continue
		}

		inner := []rune(strings.TrimSuffix(strings.TrimRight(line[border+len("│"):], " "), "│"))

		switch text := strings.TrimSpace(string(inner)); {
		case strings.HasPrefix(text, "Mon") && strings.Contains(text, "Sun"):
			inGrid = true

			continue
		case !inGrid:
			continue
		case text == "" || strings.HasPrefix(text, "Month total"):
			return cells
		}

		// The frame's padding, two columns.
		if len(inner) < 2 {
			continue
		}

		row := inner[2:]

		for start := 0; start < len(row); start += 10 {
			cell := strings.Fields(string(row[start:min(start+9, len(row))]))
			if len(cell) == 0 {
				continue
			}

			day, err := strconv.Atoi(strings.TrimSuffix(cell[0], "•"))
			if err == nil {
				cells[day] = strings.Join(cell, " ")
			}
		}
	}

	return cells
}

// The calendar is the tab after the week, as on the page.
func TestTheCalendarIsTheTabAfterTheWeek(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.until("the time entries are on screen", onTab(viewTimesheets))

	row := lineWith(shows, "Week", "Calendar", "My account")
	if row == "" || strings.Index(row, "Week") > strings.Index(row, "Calendar") ||
		strings.Index(row, "Calendar") > strings.Index(row, "My account") {
		t.Errorf("the calendar is not between the week and My account:\n%s", shows)
	}

	w.close()
}

// The month is drawn as the page draws its calendar: headed by its name and
// year, the days under the weekdays from Monday, each with the hours booked on it,
// a public holiday and a day away marked and named under the grid, and the
// month's total.
func TestTheCalendarShowsTheMonthAsThePageDoes(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	f.list = listOf(0,
		booking{11, 4, dayOf(first, 1), 2.5},
		booking{12, 1, dayOf(first, 1), 1},
		booking{13, 0, dayOf(first, 9), 0.5},
	)
	f.days = daysOf(map[string]dayMark{
		dayOf(first, 2): {holiday: "germanUnity"},
		dayOf(first, 3): {absence: "vacation"},
		dayOf(first, 4): {absence: "vacation"},
	})

	w := watch(t, server.URL, f.token)
	shows := w.calendarOpen(first)

	if title := first.Format("January 2006"); !strings.Contains(shows, title) {
		t.Errorf("the month is not headed %q:\n%s", title, shows)
	}

	if lineWith(shows, "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun") == "" {
		t.Errorf("the days are not under the weekdays from Monday:\n%s", shows)
	}

	cells := calendarCells(shows)
	last := first.AddDate(0, 1, -1).Day()

	if len(cells) != last {
		t.Fatalf("the grid shows %d days of a month of %d: %v\n%s", len(cells), last, cells, shows)
	}

	for day, want := range map[int]string{2: "2 3.5", 3: "3•", 4: "4•", 5: "5•", 10: "10 0.5", 1: "1"} {
		if cells[day] != want {
			t.Errorf("day %d says %q, want %q:\n%s", day, cells[day], want, shows)
		}
	}

	marks := first.AddDate(0, 0, 2).Format("01-02") + ": German Unity Day · " +
		first.AddDate(0, 0, 3).Format("01-02") + "–" + first.AddDate(0, 0, 4).Format("01-02") + ": Leave"
	if !strings.Contains(prose(shows), marks) {
		t.Errorf("the marked days are not named under the grid as %q:\n%s", marks, shows)
	}

	if !strings.Contains(shows, "Month total: 4.00 h") {
		t.Errorf("the month's total is not said:\n%s", shows)
	}

	w.close()
}

// The calendar moves a month back and on with < and >, reading each month it
// moves to from its first day to its last, and t brings it back to this month,
// which it offers only once it is somewhere else.
func TestTheCalendarMovesMonthByMonth(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	f.list = listOf(0)

	w := watch(t, server.URL, f.token)
	shows := w.calendarOpen(first)

	if strings.Contains(shows, "t Today") {
		t.Errorf("this month offers to go to today:\n%s", shows)
	}

	before := first.AddDate(0, -1, 0)

	w.press("<")
	shows = w.until("the month before is read", onMonth(before.Format(time.DateOnly)))

	if !strings.Contains(shows, "t Today") {
		t.Errorf("the month before does not offer to go back to today:\n%s", shows)
	}

	asked := func(route string) bool {
		for _, sent := range f.sentTo(route) {
			q, _ := sent["?"].(url.Values)
			if q.Get("from") == before.Format(time.DateOnly) && q.Get("to") == first.AddDate(0, 0, -1).Format(time.DateOnly) {
				return true
			}
		}

		return false
	}

	if !asked("GET /timesheets") || !asked("GET /me/days") {
		t.Errorf("the month before was not read from its first day to its last: %v %v",
			f.sentTo("GET /timesheets"), f.sentTo("GET /me/days"))
	}

	w.press(">", ">")
	w.until("the month after is read", onMonth(first.AddDate(0, 1, 0).Format(time.DateOnly)))

	w.press("t")
	w.until("this month is read again", onMonth(first.Format(time.DateOnly)))

	w.close()
}

// Every entry of the month is in its calendar, however many pages the list
// answers them in.
func TestEveryEntryOfTheMonthIsReadHoweverManyPagesItTakes(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	var five []booking
	for i := range 5 {
		five = append(five, booking{uint(30 + i), 4, dayOf(first, 0), 1})
	}

	f.list = listOf(2, five...)

	w := watch(t, server.URL, f.token)
	shows := w.calendarOpen(first)

	if got := calendarCells(shows)[1]; got != "1 5" {
		t.Errorf("the month's five entries on its first came to %q:\n%s", got, shows)
	}

	if !strings.Contains(shows, "Month total: 5.00 h") {
		t.Errorf("the month's total is not every page's:\n%s", shows)
	}

	w.close()
}
