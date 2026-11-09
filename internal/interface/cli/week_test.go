package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// booking is an entry the fake installation's list answers with: its id, its
// project - zero for none - its day and its hours.
type booking struct {
	id      uint
	project uint
	day     string
	hours   float64
}

// listOf answers the entries' list with the bookings its query finds - from and
// to, on a project or on none - a page at a time as the query asks, and no more
// than most to a page where most is not zero.
func listOf(most int, bookings ...booking) func(url.Values) string {
	return func(q url.Values) string {
		found := []map[string]any{}

		for _, b := range bookings {
			project := q.Get("projectId")

			switch {
			case q.Get("from") != "" && b.day < q.Get("from"), q.Get("to") != "" && b.day > q.Get("to"),
				project == "none" && b.project != 0,
				project != "" && project != "none" && project != strconv.FormatUint(uint64(b.project), 10):
				continue
			}

			item := map[string]any{"id": b.id, "date": b.day, "durationHours": b.hours, "projectId": nil}
			if b.project != 0 {
				item["projectId"] = b.project
			}

			found = append(found, item)
		}

		offset, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("limit"))

		if limit == 0 {
			limit = 100
		}

		if most > 0 {
			limit = min(limit, most)
		}

		page := append([]map[string]any{}, found[min(offset, len(found)):min(offset+limit, len(found))]...)
		raw, _ := json.Marshal(map[string]any{"items": page, "totalCount": len(found)})

		return string(raw)
	}
}

// dayMark is what /me/days says about a day besides its date.
type dayMark struct {
	target, booked   float64
	holiday, absence string
	counts           bool
}

// daysOf answers /me/days for the range its query asks: eight hours asked of a
// day from Monday to Friday and none of a weekend, where marks says nothing else
// about it, nothing booked, and no day counting towards the balance.
func daysOf(marks map[string]dayMark) func(url.Values) string {
	return func(q url.Values) string {
		from, _ := time.Parse(time.DateOnly, q.Get("from"))
		to, _ := time.Parse(time.DateOnly, q.Get("to"))

		days := []map[string]any{}

		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			date := d.Format(time.DateOnly)
			day := map[string]any{"date": date, "booked": 0, "target": 8.0, "counts": false}

			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
				day["target"] = 0.0
			}

			if mark, ok := marks[date]; ok {
				day["target"], day["counts"], day["booked"] = mark.target, mark.counts, mark.booked
				day["holiday"], day["absence"] = mark.holiday, mark.absence
			}

			days = append(days, day)
		}

		raw, _ := json.Marshal(map[string]any{"days": days})

		return string(raw)
	}
}

// mondayHere is the Monday of the week today falls in where the fake
// installation's account keeps its days.
func mondayHere(t *testing.T) time.Time {
	t.Helper()

	now := time.Now().In(mustZone(t, "Europe/Berlin"))

	return time.Date(now.Year(), now.Month(), now.Day()-(int(now.Weekday())+6)%7, 0, 0, 0, 0, time.UTC)
}

// dayOf is the day so many days after a Monday, as it travels.
func dayOf(monday time.Time, days int) string {
	return monday.AddDate(0, 0, days).Format(time.DateOnly)
}

// weekOn is a screen on the week's tab, with the grid read for the week from
// monday.
func weekOn(monday string) func(screen) bool {
	return func(m screen) bool {
		return settled(m) && m.currentTab().view == viewWeek && m.week.monday == monday
	}
}

// weekOpen moves to the week's tab once the day is read, and waits for this
// week's grid.
func (w *watched) weekOpen(monday time.Time) string {
	w.t.Helper()

	w.until("the time entries are on screen", onTab(viewTimesheets))
	w.press("right")

	return w.until("the week is read", weekOn(dayOf(monday, 0)))
}

// cellsOf is what the line of the week grid a name starts says after the name,
// a figure at a time; nil where no line starts with it.
func cellsOf(shows, name string) []string {
	for _, line := range strings.Split(shows, "\n") {
		inner := strings.TrimLeft(strings.Trim(line, "│ "), "> ")

		if strings.HasPrefix(inner, name+" ") {
			return strings.Fields(strings.TrimPrefix(inner, name))
		}
	}

	return nil
}

// The week is the tab after the time entries, as on the page.
func TestTheWeekIsTheTabAfterTheTimeEntries(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.until("the time entries are on screen", onTab(viewTimesheets))

	row := lineWith(shows, "Time entries", "Week", "My account")
	if row == "" || strings.Index(row, "Time entries") > strings.Index(row, "Week") ||
		strings.Index(row, "Week") > strings.Index(row, "My account") {
		t.Errorf("the week is not between the time entries and My account:\n%s", shows)
	}

	w.close()
}

// The week is drawn as the page's grid: the projects booked on in it as rows by
// name and the hours without a project last, each day's sum in its column - one
// of several entries marked, since it is not one entry's - the row's total, and
// under them what each day came to, asked and left over, the last only where a
// day counts. A public holiday and a day away are named with their day.
func TestTheWeekShowsItsEntriesAsThePagesGridDoes(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)

	f.list = listOf(0,
		booking{11, 4, dayOf(monday, 0), 2.5},
		booking{12, 4, dayOf(monday, 1), 1},
		booking{13, 1, dayOf(monday, 0), 1},
		booking{14, 1, dayOf(monday, 0), 0.5},
		booking{15, 0, dayOf(monday, 2), 0.5},
	)
	f.days = daysOf(map[string]dayMark{
		dayOf(monday, 0): {target: 8, counts: true},
		dayOf(monday, 1): {target: 8, counts: true},
		dayOf(monday, 2): {target: 8, counts: true},
		dayOf(monday, 3): {holiday: "germanUnity"},
		dayOf(monday, 4): {absence: "vacation"},
	})

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	_, number := monday.ISOWeek()
	if title := fmt.Sprintf("Week %d · %s – %s", number, dayOf(monday, 0), dayOf(monday, 6)); !strings.Contains(shows, title) {
		t.Errorf("the week is not headed %q:\n%s", title, shows)
	}

	if lineWith(shows, "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun", "Total") == "" ||
		lineWith(shows, monday.Format("01-02"), monday.AddDate(0, 0, 6).Format("01-02")) == "" {
		t.Errorf("the days are not headed by their names and dates:\n%s", shows)
	}

	for name, want := range map[string][]string{
		"Consulting": {"2.5", "1", "–", "–", "–", "–", "–", "3.5"},
		"Website":    {"1.5*", "–", "–", "–", "–", "–", "–", "1.5"},
		"No project": {"–", "–", "0.5", "–", "–", "–", "–", "0.5"},
		"Booked":     {"4", "1", "0.5", "0", "0", "0", "0", "5.5"},
		"Target":     {"8", "8", "8", "0", "0", "0", "0", "24"},
		"Balance":    {"−4", "−7", "−7.5", "–", "–", "–", "–", "−18.5"},
	} {
		if got := cellsOf(shows, name); !slices.Equal(got, want) {
			t.Errorf("the row of %s says %v, want %v:\n%s", name, got, want, shows)
		}
	}

	if at := strings.Index; at(shows, "Consulting ") > at(shows, "Website ") || at(shows, "Website ") > at(shows, "No project ") {
		t.Errorf("the rows are not in the page's order:\n%s", shows)
	}

	thursday, friday := monday.AddDate(0, 0, 3).Format("01-02"), monday.AddDate(0, 0, 4).Format("01-02")
	if !strings.Contains(shows, "Thu "+thursday+": German Unity Day") || !strings.Contains(shows, "Fri "+friday+": Leave") {
		t.Errorf("the holiday and the day away are not named with their days:\n%s", shows)
	}

	if !strings.Contains(shows, "* several entries") {
		t.Errorf("the mark on a cell of several entries is not explained:\n%s", shows)
	}

	w.close()
}

// A week with nothing booked says so where its rows would be, and still says
// what its days asked.
func TestAWeekWithNothingBookedSaysSo(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	f.list = listOf(0)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if !strings.Contains(shows, "Nothing booked this week yet.") {
		t.Errorf("an empty week does not say so:\n%s", shows)
	}

	if got := cellsOf(shows, "Target"); !slices.Equal(got, []string{"8", "8", "8", "8", "8", "0", "0", "40"}) {
		t.Errorf("an empty week's targets say %v:\n%s", got, shows)
	}

	w.close()
}

// The grid moves a week back and on with < and >, reading each week it moves to
// - its entries and its days - and t brings it back to this week, which it
// offers only once it is somewhere else.
func TestTheWeekMovesWeekByWeek(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	f.list = listOf(0)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if strings.Contains(shows, "This week") {
		t.Errorf("this week offers to go to this week:\n%s", shows)
	}

	w.press("<")
	shows = w.until("the week before is read", weekOn(dayOf(monday, -7)))

	if !strings.Contains(shows, "t This week") {
		t.Errorf("the week before does not offer to go back to this week:\n%s", shows)
	}

	asked := func(route string) bool {
		for _, sent := range f.sentTo(route) {
			if q, ok := sent["?"].(url.Values); ok && q.Get("from") == dayOf(monday, -7) && q.Get("to") == dayOf(monday, -1) {
				return true
			}
		}

		return false
	}

	if !asked("GET /timesheets") || !asked("GET /me/days") {
		t.Errorf("the week before was not read from its Monday to its Sunday: %v %v",
			f.sentTo("GET /timesheets"), f.sentTo("GET /me/days"))
	}

	w.press(">", ">")
	w.until("the week after is read", weekOn(dayOf(monday, 7)))

	w.press("t")
	w.until("this week is read again", weekOn(dayOf(monday, 0)))

	w.close()
}

// Every entry of the week is in the grid, however many pages the list answers
// them in - a week totalled from its first page is a week short.
func TestEveryEntryOfTheWeekIsReadHoweverManyPagesItTakes(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)

	var five []booking
	for i := range 5 {
		five = append(five, booking{uint(20 + i), 4, dayOf(monday, 0), 1})
	}

	f.list = listOf(2, five...)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if got := cellsOf(shows, "Consulting"); len(got) != 8 || got[0] != "5*" || got[7] != "5" {
		t.Errorf("the week's five entries came to %v:\n%s", got, shows)
	}

	w.close()
}
