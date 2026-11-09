package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// statisticsOf answers /me/statistics for the period its query asks: eight hours
// on its first day and four and a half on its second, the rest of its days empty
// as the endpoint sends them, nine and a half hours on Consulting, two and a half
// on no project and half an hour on one deleted since, and three of them naming
// #review.
func statisticsOf(q url.Values) string {
	from, _ := time.Parse(time.DateOnly, q.Get("from"))
	to, _ := time.Parse(time.DateOnly, q.Get("to"))

	days := []map[string]any{}

	for d, i := from, 0; !d.After(to); d, i = d.AddDate(0, 0, 1), i+1 {
		hours := 0.0

		switch i {
		case 0:
			hours = 8
		case 1:
			hours = 4.5
		}

		days = append(days, map[string]any{"date": d.Format(time.DateOnly), "hours": hours})
	}

	raw, _ := json.Marshal(map[string]any{
		"from": q.Get("from"), "to": q.Get("to"), "totalHours": 12.5, "days": days,
		"projects": []map[string]any{
			{"projectId": 4, "name": "Consulting", "hours": 9.5},
			{"projectId": nil, "name": "", "hours": 2.5},
			{"projectId": 9, "name": "", "hours": 0.5},
		},
		"tags":          []map[string]any{{"tag": "review", "hours": 3}},
		"untaggedHours": 9.5,
	})

	return string(raw)
}

// onOvertime is a screen on the overtime's tab with the hours read for the
// period from first to last.
func onOvertime(first, last string) func(screen) bool {
	return func(m screen) bool {
		return settled(m) && m.currentTab().view == viewOvertime && m.ot.stats.From == first && m.ot.stats.To == last
	}
}

// overtimeOpen moves to the overtime's tab once the day is read - three tabs on
// from the time entries - and waits for this month's hours so far.
func (w *watched) overtimeOpen(first time.Time) string {
	w.t.Helper()

	w.until("the time entries are on screen", onTab(viewTimesheets))
	w.press("right", "right", "right")

	return w.until("the hours are read", func(m screen) bool {
		return settled(m) && m.currentTab().view == viewOvertime && m.ot.stats.From == first.Format(time.DateOnly)
	})
}

// period chooses the period from first to last as somebody types it: z, then
// each day over the one the field offers.
func (w *watched) period(first, last string) {
	w.t.Helper()

	w.press("z")
	w.until("the first day is asked", asking("statsFrom"))

	for range 10 {
		w.press("backspace")
	}

	w.press(first, "enter")
	w.until("the last day is asked", asking("statsTo"))

	for range 10 {
		w.press("backspace")
	}

	w.press(last, "enter")
}

// The overtime is the tab after the calendar, as on the page.
func TestTheOvertimeIsTheTabAfterTheCalendar(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.until("the time entries are on screen", onTab(viewTimesheets))

	row := lineWith(shows, "Calendar", "Overtime", "My account")
	if row == "" || strings.Index(row, "Calendar") > strings.Index(row, "Overtime") ||
		strings.Index(row, "Overtime") > strings.Index(row, "My account") {
		t.Errorf("the overtime is not between the calendar and My account:\n%s", shows)
	}

	w.close()
}

// The hours of a period are drawn as the page's charts draw them: a bar a day
// with its date and hours, the empty days among them, a bar a project - the
// hours on none and on one deleted since each named so - and a bar an activity
// with the hours naming none last, under the period and its total. The period
// starts as the page's does: from the first of the month to today.
func TestMyHoursAreDrawnAsThePagesBars(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	shows := w.overtimeOpen(first)

	sent := f.sentTo("GET /me/statistics")
	q, _ := sent[len(sent)-1]["?"].(url.Values)

	if q.Get("from") != first.Format(time.DateOnly) || q.Get("to") != time.Now().In(mustZone(t, "Europe/Berlin")).Format(time.DateOnly) {
		t.Errorf("the hours were asked for %v, want from the first of the month to today", q)
	}

	if !strings.Contains(shows, "My hours") || !strings.Contains(shows, "Total: 12.50 h") {
		t.Errorf("the card is not headed by what it is and the period's total:\n%s", shows)
	}

	for _, bar := range [][]string{
		{first.Format(time.DateOnly), "█", "8.00 h"},
		{"Consulting", "█", "9.50 h"},
		{"No project", "█", "2.50 h"},
		{"deleted project", "█", "0.50 h"},
		{"#review", "█", "3.00 h"},
		{"No activity", "█", "9.50 h"},
	} {
		if lineWith(shows, bar...) == "" {
			t.Errorf("there is no bar of %v:\n%s", bar, shows)
		}
	}

	for _, heading := range []string{"Hours per day", "Hours per project", "Hours per activity"} {
		if !strings.Contains(shows, heading) {
			t.Errorf("the chart %q is not headed:\n%s", heading, shows)
		}
	}

	w.close()
}

// The period is chosen as the page's two fields choose it, and the hours are
// read again for it.
func TestThePeriodOfMyHoursIsChosen(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.overtimeOpen(first)

	w.period("2026-10-01", "2026-10-03")
	shows := w.until("October's hours are read", onOvertime("2026-10-01", "2026-10-03"))

	if lineWith(shows, "2026-10-01", "8.00 h") == "" || lineWith(shows, "2026-10-03", "0.00 h") == "" {
		t.Errorf("the days of the period chosen are not drawn, the empty ones among them:\n%s", shows)
	}

	w.close()
}

// A period with more days than the window has room for shows as many as fit and
// says which, and PgUp and PgDn page through the rest, as the entries do.
func TestALongPeriodsDaysArePaged(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.overtimeOpen(first)

	w.period("2026-01-01", "2026-03-31")
	shows := w.until("the quarter's hours are read", onOvertime("2026-01-01", "2026-03-31"))

	if !strings.Contains(shows, "of 90 · pgup/pgdn") || lineWith(shows, "2026-01-01", "8.00 h") == "" ||
		lineWith(shows, "2026-03-31", "0.00 h") != "" {
		t.Errorf("a quarter's days are not paged:\n%s", shows)
	}

	w.press("pgdown")
	shows = w.until("the days move on", func(m screen) bool { return settled(m) && m.ot.top > 0 })

	if lineWith(shows, "2026-01-01", "8.00 h") != "" {
		t.Errorf("the first day is still drawn after a page down:\n%s", shows)
	}

	w.close()
}

// Every page of days is drawn to the scale of the whole period, as the page's one
// chart draws them, so that a bar on one page can be held against a bar on the
// next: four hours late in the quarter are half as long as eight in its first
// day, not as long as the longest bar of their own page.
func TestAPageOfDaysIsDrawnToTheScaleOfTheWholePeriod(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	f.mu.Lock()
	f.stats = func(q url.Values) string {
		var answer map[string]any

		_ = json.Unmarshal([]byte(statisticsOf(q)), &answer)

		if days, _ := answer["days"].([]any); len(days) > 59 {
			days[59].(map[string]any)["hours"] = 4
		}

		raw, _ := json.Marshal(answer)

		return string(raw)
	}
	f.mu.Unlock()

	w := watch(t, server.URL, f.token)
	w.overtimeOpen(first)

	w.period("2026-01-01", "2026-03-31")
	shows := w.until("the quarter's hours are read", onOvertime("2026-01-01", "2026-03-31"))
	longest := strings.Count(lineWith(shows, "2026-01-01", "8.00 h"), "█")

	top := 0

	for range 40 {
		if lineWith(shows, "2026-03-01", "4.00 h") != "" {
			break
		}

		was := top

		w.press("pgdown")
		shows = w.until("the days move on", func(m screen) bool {
			top = m.ot.top

			return settled(m) && m.ot.top > was
		})
	}

	half := strings.Count(lineWith(shows, "2026-03-01", "4.00 h"), "█")
	if longest == 0 || (2*half != longest && 2*half != longest-1) {
		t.Errorf("four hours are drawn %d long where eight were drawn %d:\n%s", half, longest, shows)
	}

	w.close()
}

// An answer for a period chosen before, landing after another was chosen - a
// slow read overtaken by a quick one - is not taken over the hours of the one
// chosen since. It is handed to the screen after the newer one rather than
// raced for, so the order tested is the same every time.
func TestAnAnswerForAPeriodChosenBeforeIsNotTakenOverTheOneChosenSince(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.overtimeOpen(first)

	w.period("2026-10-01", "2026-10-03")
	w.until("October's hours are read", onOvertime("2026-10-01", "2026-10-03"))

	var late statistics
	if err := json.Unmarshal([]byte(statisticsOf(url.Values{"from": {"2026-09-01"}, "to": {"2026-09-02"}})), &late); err != nil {
		t.Fatal(err)
	}

	w.p.Send(statsRead{from: late.From, to: late.To, stats: late})
	shows := w.until("the late answer is handed over", settled)

	if lineWith(shows, "2026-10-01", "8.00 h") == "" {
		t.Errorf("an answer for the period before was taken over the one chosen since:\n%s", shows)
	}

	w.close()
}

// A read of the day that set out before another period was chosen brings the
// hours of the period before, and they are not taken over the hours of the one
// chosen since. The installation holds that read until the period has been
// chosen and read, so the order tested is the same every time.
func TestADayReadFromBeforeAPeriodWasChosenLeavesItsHours(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.overtimeOpen(first)

	held, release := make(chan struct{}, 1), make(chan struct{})

	var once sync.Once

	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)

	f.mu.Lock()
	f.stats = func(q url.Values) string {
		if q.Get("from") == first.Format(time.DateOnly) {
			select {
			case held <- struct{}{}:
			default:
			}

			<-release
		}

		return statisticsOf(q)
	}
	f.mu.Unlock()

	w.press("r")

	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the day was not read again")
	}

	w.period("2026-10-01", "2026-10-03")
	w.until("October's hours are read while the day is", func(m screen) bool {
		return m.form == nil && m.ot.stats.From == "2026-10-01"
	})

	free()
	shows := w.until("the day is read", settled)

	if lineWith(shows, "2026-10-01", "8.00 h") == "" {
		t.Errorf("the hours of the period before were taken over the one chosen since:\n%s", shows)
	}

	w.close()
}

// My hours are laid out as the page's document: a section a chart, as figures
// since a terminal draws no picture, under the period and the total.
func TestMyHoursAreExportedAsADocument(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	path := filepath.Join(t.TempDir(), "hours.pdf")

	w := watch(t, server.URL, f.token)
	w.overtimeOpen(first)

	w.press("p")
	shows := w.until("where it goes is asked", asking("file"))

	if !strings.Contains(shows, "statistics-") {
		t.Errorf("the file is not named as the page names it:\n%s", shows)
	}

	for range 60 {
		w.press("backspace")
	}

	w.press(path, "enter")
	w.until("the document is saved", func(m screen) bool { return settled(m) && strings.Contains(m.news, path) })

	if written, err := os.ReadFile(path); err != nil || string(written) != fakeDocument {
		t.Fatalf("the document was not saved as it came: %q, %v", written, err)
	}

	sent := sentOnce(t, f, "POST /exports/document")
	sections, _ := sent["sections"].([]any)

	if sent["title"] != "My hours" || len(sections) != 3 {
		t.Fatalf("the document is %v with %d sections, want My hours with three", sent["title"], len(sections))
	}

	projects, _ := sections[1].(map[string]any)
	table, _ := projects["table"].(map[string]any)
	rows, _ := table["rows"].([]any)

	if projects["heading"] != "Hours per project" || len(rows) != 3 || anyStrings(rows[0])[0] != "Consulting" {
		t.Errorf("the projects' section is %v", projects)
	}

	w.close()
}
