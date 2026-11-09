package cli

import (
	"strings"
	"testing"
)

// onTab is a screen that has read the day and shows the tab named view.
func onTab(view string) func(screen) bool {
	return func(m screen) bool { return settled(m) && m.currentTab().view == view }
}

// lineWith is the first line of the screen naming every one of names, inside the
// frame; empty where none does.
func lineWith(shows string, names ...string) string {
	for _, line := range strings.Split(shows, "\n") {
		named := true

		for _, name := range names {
			named = named && strings.Contains(line, name)
		}

		if named {
			return strings.Trim(line, "│ ")
		}
	}

	return ""
}

// hasLine reports whether a line of the screen says exactly want, inside the
// frame.
func hasLine(shows, want string) bool {
	for _, line := range strings.Split(shows, "\n") {
		if strings.Trim(line, "│ ") == want {
			return true
		}
	}

	return false
}

// The screen is headed by the page's tabs, in the page's order, and opens on
// the time entries as the page does - which is where the breadcrumb starts.
func TestTheScreenIsHeadedByThePagesTabs(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.until("the time entries are on screen", onTab(viewTimesheets))

	row := lineWith(shows, "Time entries", "My account")
	if row == "" || strings.Index(row, "Time entries") > strings.Index(row, "My account") {
		t.Errorf("the screen is not headed by the time entries and My account, in that order:\n%s", shows)
	}

	if !hasLine(shows, "Time entries") {
		t.Errorf("the breadcrumb does not say the tab on screen:\n%s", shows)
	}

	w.press("b")
	shows = w.until("the hours are asked", asking("hours"))

	if lineWith(shows, "›") != "Time entries › Book time by hand › Hours" {
		t.Errorf("the breadcrumb under a form does not start at the tab: %q", lineWith(shows, "›"))
	}

	w.press("esc")
	w.close()
}

// A tab whose right the account does not hold is not offered, as the page hides
// it, and the screen opens on the first tab the account may open.
func TestATabTheAccountMayNotOpenIsNotOffered(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own"}

	w := watch(t, server.URL, f.token)
	shows := w.until("My account is on screen", onTab(viewAccount))

	if strings.Contains(shows, "Time entries") {
		t.Errorf("an account that may not read time entries is offered their tab:\n%s", shows)
	}

	if !offered(shows, "Language: English", "1") {
		t.Errorf("My account does not offer the settings:\n%s", shows)
	}

	w.close()
}

// The arrows move from tab to tab and round the end, and what is offered under
// each is that tab's: the stopwatch's changes under the time entries, the
// account's settings under My account.
func TestTheArrowsMoveFromTabToTab(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.until("the time entries are on screen", onTab(viewTimesheets))

	w.press("right")
	w.until("the week is on screen", onTab(viewWeek))

	w.press("right")
	w.until("the calendar is on screen", onTab(viewCalendar))

	w.press("right")
	shows := w.until("My account is on screen", onTab(viewAccount))

	if !offered(shows, "Language: English", "1") || !offered(shows, "Appearance: Automatic", "2") {
		t.Errorf("My account does not offer the settings by number:\n%s", shows)
	}

	if offered(shows, "Switch to another project", "w") {
		t.Errorf("My account offers the stopwatch's changes:\n%s", shows)
	}

	w.press("right")
	shows = w.until("the arrow goes round to the time entries", onTab(viewTimesheets))

	if !offered(shows, "Switch to another project", "w") {
		t.Errorf("the time entries do not offer the stopwatch's changes:\n%s", shows)
	}

	if offered(shows, "Language: English", "1") {
		t.Errorf("the time entries offer the account's settings:\n%s", shows)
	}

	w.press("left")
	w.until("the left arrow goes back round to My account", onTab(viewAccount))

	w.close()
}
