package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// onYear is a screen showing the tiles of year.
func onYear(year int) func(screen) bool {
	return func(m screen) bool { return settled(m) && m.sub == subYear && m.ot.year.year == year }
}

// yearOpen moves to the overtime's tab, opens the year's menu with y and waits
// for the tiles of year.
func (w *watched) yearOpen(first time.Time, year int) string {
	w.t.Helper()

	w.overtimeOpen(first)
	w.press("y")

	return w.until("the year is read", onYear(year))
}

// tileOf is the tile the year's grid draws for a day, read off the screen: in
// the row of its weekday, after the weekday's name, in the column of its week.
func tileOf(shows string, day time.Time) string {
	names := language{}.weekdayNames()
	lead := (int(time.Date(day.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Weekday()) + 6) % 7
	at := lead + day.YearDay() - 1
	row := "│  " + names[at%7] + " "

	for _, line := range strings.Split(ansi.Strip(shows), "\n") {
		if _, tiles, ok := strings.Cut(line, row); ok {
			if cells := []rune(tiles); at/7 < len(cells) {
				return string(cells[at/7])
			}
		}
	}

	return ""
}

// A past year is drawn as the page's tiles: a column a week from Monday, a row a
// weekday, each day as dark as the hours booked on it - two hours a step - and a
// public holiday or a day away by a mark of its own; under the year's words,
// which compare the whole of it with the whole year before, and over the legend
// that says what each means and the months as figures.
func TestAPastYearIsDrawnAsThePagesTiles(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	shown := first.Year() - 1

	day := func(year int, month time.Month, d int) time.Time {
		return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
	}
	iso := func(at time.Time) string { return at.Format(time.DateOnly) }

	f.days = daysOf(map[string]dayMark{
		iso(day(shown, 1, 1)):    {holiday: "newYear"},
		iso(day(shown, 1, 2)):    {booked: 1},
		iso(day(shown, 1, 5)):    {booked: 3},
		iso(day(shown, 1, 6)):    {booked: 5},
		iso(day(shown, 1, 7)):    {booked: 8},
		iso(day(shown, 1, 8)):    {absence: "vacation"},
		iso(day(shown, 1, 9)):    {booked: 9},
		iso(day(shown, 3, 3)):    {booked: 2},
		iso(day(shown-1, 1, 2)):  {booked: 7.5},
		iso(day(shown-1, 12, 1)): {booked: 4},
	})

	w := watch(t, server.URL, f.token)
	w.yearOpen(first, first.Year())
	w.press("<")
	shows := w.until("the year before is read", onYear(shown))

	asked := map[string]bool{}
	for _, sent := range f.sentTo("GET /me/days") {
		q, _ := sent["?"].(url.Values)
		asked[q.Get("from")+" "+q.Get("to")] = true
	}

	for _, year := range []int{shown, shown - 1} {
		if y := strconv.Itoa(year); !asked[y+"-01-01 "+y+"-12-31"] {
			t.Errorf("the days of %d were not asked for as a whole year: %v", year, asked)
		}
	}

	for _, tile := range []struct {
		day  time.Time
		want string
	}{
		{day(shown, 1, 1), "◆"}, {day(shown, 1, 2), "░"}, {day(shown, 1, 5), "▒"}, {day(shown, 1, 6), "▓"},
		{day(shown, 1, 7), "█"}, {day(shown, 1, 8), "◇"}, {day(shown, 1, 9), "█"}, {day(shown, 1, 12), "·"},
		{day(shown, 3, 3), "░"}, {day(shown, 12, 31), "·"},
	} {
		if got := tileOf(shows, tile.day); got != tile.want {
			t.Errorf("%s is drawn %q, want %q:\n%s", iso(tile.day), got, tile.want, shows)
		}
	}

	for _, said := range []string{
		"The year " + strconv.Itoa(shown),
		fmt.Sprintf("Booked: 28.00 h · days with hours: 6 · %d to the same day: 11.50 h", shown-1),
	} {
		if !strings.Contains(shows, said) {
			t.Errorf("the year does not say %q:\n%s", said, shows)
		}
	}

	if lineWith(shows, "Less", "More", "Public holiday", "Away") == "" {
		t.Errorf("there is no legend:\n%s", shows)
	}

	// In order: a line naming all three would pass with the two years swapped.
	if table := fmt.Sprintf("Month %d %d January 26.00 h 7.50 h", shown, shown-1); !strings.Contains(prose(shows), table) {
		t.Errorf("the months are not set out as figures against the year before:\n%s", shows)
	}

	w.close()
}

// The year that is running is compared with the year before up to the same day,
// as the page compares them - a year half done against a whole one would always
// say less - while the days with hours are counted over the whole year; and
// today's tile is filled with the accent, as the page rings it.
func TestTheRunningYearIsComparedUpToTheSameDay(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	year := first.Year()

	f.days = daysOf(map[string]dayMark{
		fmt.Sprintf("%d-01-01", year):   {booked: 6},
		fmt.Sprintf("%d-12-31", year):   {booked: 4},
		fmt.Sprintf("%d-01-01", year-1): {booked: 2},
		fmt.Sprintf("%d-12-31", year-1): {booked: 3},
	})

	w := watch(t, server.URL, f.token)
	shows := w.yearOpen(first, year)

	booked, before := 6.0, 2.0
	if time.Now().In(mustZone(t, "Europe/Berlin")).Format("01-02") == "12-31" {
		booked, before = 10, 5
	}

	want := fmt.Sprintf("Booked: %.2f h · days with hours: 2 · %d to the same day: %.2f h", booked, year-1, before)
	if !strings.Contains(shows, want) {
		t.Errorf("the running year does not say %q:\n%s", want, shows)
	}

	// What until answers with has its colours taken off, so the fill is looked for
	// in the screen as it is drawn.
	var (
		drawn string
		dark  bool
	)

	w.until("the screen is drawn", func(m screen) bool { drawn, dark = m.View().Content, m.darkness(); return true })

	glyph := tileOf(shows, time.Now().In(mustZone(t, "Europe/Berlin")))
	if glyph == "" || !strings.Contains(drawn, paletteFor(dark).tabOn.Render(glyph)) {
		t.Errorf("today, drawn %q, is not filled with the accent:\n%s", glyph, shows)
	}

	w.close()
}

// < and > move the tiles a year back and on while the year's menu is open, which
// the help line says, and read the year moved to with the one before it.
func TestTheArrowsMoveTheYear(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	year := first.Year()

	w := watch(t, server.URL, f.token)
	shows := w.yearOpen(first, year)

	if !strings.Contains(shows, "</> years") || !strings.Contains(shows, "Overtime › Year") {
		t.Errorf("the year's menu does not say where it is and which keys move it:\n%s", shows)
	}

	w.press("<")
	shows = w.until("the year before is read", onYear(year-1))

	if !strings.Contains(shows, "The year "+strconv.Itoa(year-1)) {
		t.Errorf("the year before is not headed by its year:\n%s", shows)
	}

	w.press(">")
	w.until("this year is read again", onYear(year))

	w.close()
}

// A year with more months than the window has room for under its tiles shows as
// many as fit and says which, and PgUp and PgDn page through the rest.
func TestTheYearsMonthsArePagedWhereTheWindowIsShort(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	shows := w.yearOpen(first, first.Year())

	if !strings.Contains(shows, "of 12 · pgup/pgdn") || lineWith(shows, "January") == "" || lineWith(shows, "December") != "" {
		t.Errorf("the months are not paged:\n%s", shows)
	}

	w.press("pgdown")
	shows = w.until("the months move on", func(m screen) bool { return settled(m) && m.ot.yearTop > 0 })

	if lineWith(shows, "January") != "" {
		t.Errorf("January is still drawn after a page down:\n%s", shows)
	}

	w.close()
}

// A year read for tiles moved away from since - a slow read overtaken by a quick
// one - is not taken over the year moved to. It is handed to the screen after the
// newer one rather than raced for, so the order tested is the same every time.
func TestAYearReadForTilesMovedAwayFromIsNotTaken(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	year := first.Year()

	w := watch(t, server.URL, f.token)
	w.yearOpen(first, year)

	w.press("<")
	w.until("the year before is read", onYear(year-1))

	w.p.Send(yearRead{days: yearDays{year: year}})
	shows := w.until("the late answer is handed over", settled)

	if !strings.Contains(shows, "The year "+strconv.Itoa(year-1)) {
		t.Errorf("a year moved away from was taken over the one moved to:\n%s", shows)
	}

	w.close()
}

// A read of the day that set out before the tiles moved brings the year they
// showed, and it is not taken over the year moved to. The installation holds
// that read until the year has moved and been read, so the order tested is the
// same every time.
func TestADayReadFromBeforeTheYearMovedLeavesItsTiles(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	year := first.Year()

	w := watch(t, server.URL, f.token)
	w.yearOpen(first, year)

	held, release := make(chan struct{}, 1), make(chan struct{})

	var once sync.Once

	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)

	f.mu.Lock()
	f.days = func(q url.Values) string {
		if q.Get("from") == strconv.Itoa(year)+"-01-01" {
			select {
			case held <- struct{}{}:
			default:
			}

			<-release
		}

		return daysOf(nil)(q)
	}
	f.mu.Unlock()

	w.press("r")

	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the day was not read again with its year")
	}

	w.press("<")
	w.until("the year before is read while the day is", func(m screen) bool { return m.ot.year.year == year-1 })

	free()
	shows := w.until("the day is read", settled)

	if !strings.Contains(shows, "The year "+strconv.Itoa(year-1)) {
		t.Errorf("the year the tiles moved away from was taken over the one moved to:\n%s", shows)
	}

	w.close()
}
