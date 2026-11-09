package cli

import (
	"strings"
	"testing"
	"time"
)

// someDaysAway has the fake installation answer the year's days away: the
// month's fourth on leave and its fifth off sick.
func someDaysAway(f *fakeInstallation, first time.Time) {
	f.answers = map[string]string{"GET /me/absences": `{"items":[` +
		`{"day":"` + dayOf(first, 3) + `","kind":"vacation"},{"day":"` + dayOf(first, 4) + `","kind":"sick"}],"totalCount":2}`}
}

// awayOpen opens the days away's menu under the calendar.
func (w *watched) awayOpen(first time.Time) string {
	w.t.Helper()

	shows := w.calendarOpen(first)
	if !offered(shows, "Days away …", "a") {
		w.t.Fatalf("the days away are not offered:\n%s", shows)
	}

	w.press("a")

	return w.until("the days away's menu is open", func(m screen) bool { return settled(m) && m.sub == subAway })
}

// The days away are shown as the page's card shows them: each day and why, in the
// page's words.
func TestTheDaysAwayAreShownAsThePageShowsThem(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	someDaysAway(f, first)

	w := watch(t, server.URL, f.token)
	shows := w.awayOpen(first)

	if lineWith(shows, dayOf(first, 3), "Leave") == "" || lineWith(shows, dayOf(first, 4), "Sick") == "" {
		t.Errorf("the days away are not listed with why:\n%s", shows)
	}

	w.close()
}

// Days away are recorded over a range, for one reason chosen from the page's
// five, and the screen says how many days the installation recorded - only the
// days the week asks something of are.
func TestDaysAwayAreRecordedOverARange(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.awayOpen(first)

	w.press("n")
	w.until("the first day is asked", asking("awayFrom"))

	for range 10 {
		w.press("backspace")
	}

	w.press(dayOf(first, 15), "enter")
	w.until("the last day is asked", asking("awayTo"))

	for range 10 {
		w.press("backspace")
	}

	w.press(dayOf(first, 17), "enter")

	shows := w.until("the reason is asked", asking("awayKind"))
	for _, reason := range []string{"Leave", "Half a day of leave", "Sick", "Day off against overtime", "Holiday where I work"} {
		if !strings.Contains(shows, reason) {
			t.Errorf("the reason %q is not offered:\n%s", reason, shows)
		}
	}

	w.press("down", "down", "enter")
	shows = w.until("the days are recorded", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "POST /me/absences")
	if sent["from"] != dayOf(first, 15) || sent["to"] != dayOf(first, 17) || sent["kind"] != "sick" {
		t.Errorf("the days away were recorded as %v", sent)
	}

	if !strings.Contains(shows, "✓ Days recorded: 3") {
		t.Errorf("the screen does not say how many days were recorded:\n%s", shows)
	}

	w.close()
}

// A day away is taken back once the answer is yes, named by its day and why, as
// the page's delete button names it.
func TestADayAwayIsTakenBackAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	someDaysAway(f, first)

	w := watch(t, server.URL, f.token)
	w.awayOpen(first)

	w.press("d")
	w.until("a day away is to be picked", picking("awayDelete"))
	w.press("down", "enter")

	shows := prose(w.until("taking it back is asked about", asking("confirm")))
	if !strings.Contains(shows, dayOf(first, 4)+", Sick will be deleted.") {
		t.Errorf("the question does not name the day and why:\n%s", shows)
	}

	w.press("y")
	shows = w.until("the day away is taken back", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /me/absences/"+dayOf(first, 4))) != 1 {
		t.Errorf("the day away was not taken back once: %v", f.sentTo("DELETE /me/absences/"+dayOf(first, 4)))
	}

	if !strings.Contains(shows, "✓ Absence taken back") {
		t.Errorf("the screen does not say it was taken back:\n%s", shows)
	}

	w.close()
}

// An account that may not book its own time is not offered its days away, as
// the page's card is behind that right.
func TestAnAccountThatMayNotBookIsNotOfferedTheDaysAway(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own", "timesheets:read:own"}
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	shows := w.calendarOpen(first)

	if offered(shows, "Days away …", "a") {
		t.Errorf("an account that may not book is offered its days away:\n%s", shows)
	}

	w.close()
}
