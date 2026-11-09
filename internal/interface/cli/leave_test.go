package cli

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// leaveOpen opens the leave's menu under the calendar.
func (w *watched) leaveOpen(first time.Time) string {
	w.t.Helper()

	shows := w.calendarOpen(first)
	if !offered(shows, "Leave …", "u") {
		w.t.Fatalf("the leave is not offered:\n%s", shows)
	}

	w.press("u")

	return w.until("the leave's menu is open", func(m screen) bool { return settled(m) && m.sub == subLeave })
}

// onLeaveYear is a screen whose leave was read for year.
func onLeaveYear(year int) func(screen) bool {
	return func(m screen) bool { return settled(m) && m.sub == subLeave && m.cal.leave.Year == year }
}

// A year's leave is shown as the page's card shows it: headed by the year, what
// is left, what makes it up, and the days a year given from which year on - a
// whole number as it is and a fraction to two places, as the page fills them in.
func TestTheLeaveIsShownAsThePageShowsIt(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.leaveOpen(first)

	shows := prose(w.until("this year's leave is read", onLeaveYear(first.Year())))

	for _, said := range []string{"Leave " + strconv.Itoa(first.Year()), "Days left: 23.50",
		"Given: 30 · carried over: 2 · taken: 6.50 · planned: 2"} {
		if !strings.Contains(shows, said) {
			t.Errorf("the leave does not say %q:\n%s", said, shows)
		}
	}

	if lineWith(shows, "2025", "30.00") == "" {
		t.Errorf("the days given from 2025 on are not listed:\n%s", shows)
	}

	w.close()
}

// The leave moves a year back and on with < and > while its menu is open,
// reading each year it moves to, as the page's card has arrows of its own.
func TestTheLeaveMovesYearByYear(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	shows := w.leaveOpen(first)

	if !strings.Contains(shows, "</> years") {
		t.Errorf("the keys that move the leave are not named:\n%s", shows)
	}

	w.press("<")
	w.until("last year's leave is read", onLeaveYear(first.Year()-1))

	asked := false
	for _, sent := range f.sentTo("GET /me/vacation") {
		q, _ := sent["?"].(url.Values)
		asked = asked || q.Get("year") == strconv.Itoa(first.Year()-1)
	}

	if !asked {
		t.Errorf("last year's leave was not asked for: %v", f.sentTo("GET /me/vacation"))
	}

	w.press(">")
	w.until("this year's leave is read again", onLeaveYear(first.Year()))

	w.close()
}

// Days of leave are given a year from a year on, as the page's form gives them:
// half days count, and a number of days that is not one is refused.
func TestDaysOfLeaveAreGivenFromAYear(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.leaveOpen(first)

	w.press("n")
	w.until("the days are asked", asking("leaveDays"))
	w.press("30.3", "enter")
	w.until("days that are not whole or half are refused", func(m screen) bool {
		return asking("leaveDays")(m) && strings.Contains(prose(m.View().Content), "Write the days as whole or half days")
	})

	w.press("backspace", "backspace", "backspace", "backspace", "27,5", "enter")
	w.until("the year is asked", asking("leaveYear"))
	w.press("enter")
	shows := w.until("the leave is saved", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "PUT /me/vacation")
	if sent["days"] != 27.5 || sent["fromYear"] != float64(first.Year()) {
		t.Errorf("the leave was given as %v, want 27.5 days from %d", sent, first.Year())
	}

	if !strings.Contains(shows, "✓ Leave saved") {
		t.Errorf("the screen does not say the leave was saved:\n%s", shows)
	}

	w.close()
}

// The days given from a year on are taken back once the answer is yes.
func TestAnEntitlementIsTakenBackAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.leaveOpen(first)

	w.press("d")
	w.until("an entitlement is to be picked", picking("leaveDelete"))
	w.press("enter")

	shows := prose(w.until("taking it back is asked about", asking("confirm")))
	if !strings.Contains(shows, "from 2025 will be deleted.") {
		t.Errorf("the question does not name the entitlement as the page does:\n%s", shows)
	}

	w.press("y")
	shows = w.until("the entitlement is taken back", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /me/vacation/2025")) != 1 {
		t.Errorf("the entitlement was not taken back once: %v", f.sentTo("DELETE /me/vacation/2025"))
	}

	if !strings.Contains(shows, "✓ Entitlement taken back") {
		t.Errorf("the screen does not say it was taken back:\n%s", shows)
	}

	w.close()
}

// An account that may not set its own settings sees its leave and is offered
// nothing to change in it, as the page shows it no form.
func TestAnAccountThatMayNotSetItsSettingsOnlyReadsItsLeave(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.permissions = []string{"timesheets:read:own", "timesheets:write:own"}
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	shows := w.leaveOpen(first)

	if offered(shows, "Give days of leave a year", "n") || offered(shows, "Take back days given a year", "d") {
		t.Errorf("an account that may not set its settings is offered to change its leave:\n%s", shows)
	}

	w.close()
}
