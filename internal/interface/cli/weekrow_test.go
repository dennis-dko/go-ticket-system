package cli

import (
	"strings"
	"testing"
)

// A row is added for a project the grid does not show yet - an active one, as
// the page's picker offers only those - and the cursor goes to its first day,
// where somebody adding a row starts typing.
func TestARowAddedForAProjectIsFilledFromItsFirstDay(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if !offered(shows, "Add a row", "a") {
		t.Fatalf("adding a row is not offered:\n%s", shows)
	}

	w.press("a")
	shows = w.until("the project is asked", asking("row"))

	if !strings.Contains(shows, "Webshop") {
		t.Errorf("the active project with no row is not offered:\n%s", shows)
	}

	for _, shown := range []string{"Consulting", "Website", "Archive", "No project"} {
		if strings.Contains(shows, shown) {
			t.Errorf("a project with a row already, or one that takes no hours, is offered (%q):\n%s", shown, shows)
		}
	}

	w.press("enter")
	shows = w.until("the cursor is on the new row's Monday", onCell("2", 0))

	if got := cellsOf(shows, "Webshop"); len(got) != 8 || got[0] != "–" || got[7] != "0" {
		t.Errorf("the new row is not drawn empty: %v\n%s", got, shows)
	}

	w.press("enter")
	w.until("the hours are asked", asking("hours"))
	w.press("4", "enter")
	w.until("the hours are booked", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "POST /timesheets")
	if sent["projectId"] != 2.0 || sent["date"] != dayOf(monday, 0) || sent["durationHours"] != 4.0 {
		t.Errorf("the new row's Monday was booked as %v", sent)
	}

	w.press("esc")
	w.close()
}

// An empty week has no cell to type into, so a row is where it starts - for a
// project, or for the hours without one.
func TestAnEmptyWeekStartsWithARow(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	f.list = listOf(0)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if offered(shows, "Enter hours", "e") {
		t.Errorf("a week with no rows offers to enter hours:\n%s", shows)
	}

	w.press("a")
	shows = w.until("the project is asked", asking("row"))

	for _, name := range []string{"Website", "Webshop", "Consulting", "No project"} {
		if !strings.Contains(shows, name) {
			t.Errorf("%s is not offered a row:\n%s", name, shows)
		}
	}

	if strings.Contains(shows, "Archive") {
		t.Errorf("the archived project is offered a row:\n%s", shows)
	}

	w.press("esc")
	w.until("the menu is back", func(m screen) bool { return settled(m) && m.picking == "" })
	w.close()
}

// A row added by hand belongs to the week it was added to, as the page keeps it:
// gone from the week after, and there again on coming back.
func TestARowAddedByHandStaysWithItsWeek(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	f.list = listOf(0)

	w := watch(t, server.URL, f.token)
	w.weekOpen(monday)

	w.press("a")
	w.until("the project is asked", asking("row"))
	w.press("enter")
	w.until("the cursor is in the new row", func(m screen) bool { return m.picking == "cell" })
	w.press("esc")

	shows := w.until("the menu is back", func(m screen) bool { return settled(m) && m.picking == "" })
	if cellsOf(shows, "Website") == nil {
		t.Fatalf("the row added is not drawn:\n%s", shows)
	}

	w.press(">")
	shows = w.until("the week after is read", weekOn(dayOf(monday, 7)))

	if cellsOf(shows, "Website") != nil {
		t.Errorf("a row added to one week is drawn in the next:\n%s", shows)
	}

	w.press("<")
	shows = w.until("the week is read again", weekOn(dayOf(monday, 0)))

	if cellsOf(shows, "Website") == nil {
		t.Errorf("the row added is gone on coming back to its week:\n%s", shows)
	}

	w.close()
}

// Copying last week books entries rather than filling a form, so it asks first,
// in the page's words, and says afterwards what it did - the days it left alone
// and the entries it left out included.
func TestCopyingLastWeekAsksFirstAndSaysWhatItDid(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if !offered(shows, "Copy last week", "c") {
		t.Fatalf("copying last week is not offered:\n%s", shows)
	}

	w.press("c")

	shows = prose(w.until("the copy is asked about", asking("copy")))
	if !strings.Contains(shows, "Copy last week?") ||
		!strings.Contains(shows, "The entries of the week from "+dayOf(monday, -7)+" are booked again") {
		t.Errorf("the copy is not asked about in the page's words:\n%s", shows)
	}

	w.press("n")
	w.until("the menu is back", func(m screen) bool { return settled(m) && m.form == nil })

	if len(f.sentTo("POST /me/weeks/copy")) > 0 {
		t.Fatal("last week was copied although the answer was no")
	}

	w.press("c")
	w.until("the copy is asked about again", asking("copy"))
	w.press("y")

	shows = prose(w.until("last week is copied", func(m screen) bool { return settled(m) && m.news != "" }))

	if sent := sentOnce(t, f, "POST /me/weeks/copy"); sent["from"] != dayOf(monday, -7) || sent["to"] != dayOf(monday, 0) {
		t.Errorf("the copy was asked for as %v, want from last Monday into this one", sent)
	}

	for _, said := range []string{"Entries copied: 3 (7.50 h).", "Days already booked, left as they were: 1.",
		"Left out because their project no longer takes bookings: 1."} {
		if !strings.Contains(shows, said) {
			t.Errorf("the screen does not say %q:\n%s", said, shows)
		}
	}

	w.close()
}
