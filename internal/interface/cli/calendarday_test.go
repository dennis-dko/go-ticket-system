package cli

import (
	"strings"
	"testing"
	"time"
)

// aMonth has the fake installation answer a month with something in it: on its
// second day 2.5 hours on Consulting and an hour on Website, and on its tenth
// half an hour on no project.
func aMonth(f *fakeInstallation, first time.Time) {
	f.list = listOf(0,
		booking{11, 4, dayOf(first, 1), 2.5},
		booking{12, 1, dayOf(first, 1), 1},
		booking{13, 0, dayOf(first, 9), 0.5},
	)
}

// onDay is a screen picking a day of the calendar, with the cursor on date.
func onDay(date string) func(screen) bool {
	return func(m screen) bool { return m.picking == "showDay" && m.cal.cursor == date }
}

// dayPick offers to show a day and puts the cursor on the day so many days after
// the month's first - going to the first before, wherever the cursor started.
func (w *watched) dayPick(first time.Time, days int) {
	w.t.Helper()

	w.press("s")
	w.until("a day is to be picked", picking("showDay"))

	for range 31 {
		w.press("left")
	}

	for range days {
		w.press("right")
	}

	w.until("the cursor is on the day", onDay(dayOf(first, days)))
}

// dayOpen picks the day so many days after the month's first and opens it.
func (w *watched) dayOpen(first time.Time, days int) string {
	w.t.Helper()

	w.dayPick(first, days)
	w.press("enter")

	return w.until("the day is open", func(m screen) bool { return settled(m) && m.cal.day == dayOf(first, days) })
}

// A day of the calendar is picked with the arrows and shows its entries under
// the month, as the page's day card does: headed by its date, each entry's
// project, hours and description, and what can be done with them.
func TestADayOfTheCalendarShowsItsEntries(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aMonth(f, first)

	w := watch(t, server.URL, f.token)
	shows := w.calendarOpen(first)

	if !offered(shows, "Show a day", "s") {
		t.Fatalf("showing a day is not offered:\n%s", shows)
	}

	shows = w.dayOpen(first, 1)

	if !hasLine(shows, dayOf(first, 1)) {
		t.Errorf("the day is not headed by its date:\n%s", shows)
	}

	for name, hours := range map[string]string{"Consulting": "2.50", "Website": "1.00"} {
		if row := lineWith(shows, name, hours); row == "" || strings.HasPrefix(row, dayOf(first, 1)) {
			t.Errorf("the day does not list %s with %s under the page's columns:\n%s", name, hours, shows)
		}
	}

	for called, key := range map[string]string{"Correct an entry": "e", "Delete an entry": "d", "Close the day": "c"} {
		if !offered(shows, called, key) {
			t.Errorf("an open day does not offer %q under %s:\n%s", called, key, shows)
		}
	}

	w.close()
}

// Only a day with something booked on it opens, as the page offers only those
// to press, and an open day closes - when asked, and when the month moves on.
func TestOnlyADayWithEntriesOpensAndADayCloses(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.calendarOpen(first)

	w.dayPick(first, 0)
	w.press("enter")
	w.until("the empty day leaves the cursor where it was", func(m screen) bool { return onDay(dayOf(first, 0))(m) && m.cal.day == "" })

	w.press("right", "enter")
	w.until("the day with entries opens", func(m screen) bool { return settled(m) && m.cal.day == dayOf(first, 1) })

	w.press("c")
	w.until("the day is closed", func(m screen) bool { return settled(m) && m.cal.day == "" })

	w.dayOpen(first, 1)
	w.press(">")
	w.until("the month after has no day open", func(m screen) bool {
		return onMonth(first.AddDate(0, 1, 0).Format(time.DateOnly))(m) && m.cal.day == ""
	})

	w.close()
}

// An entry of an open day is corrected there, in the booking form filled with
// it, as the page's day card opens it for editing, and the day stays open.
func TestAnEntryOfADayIsCorrectedThere(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.calendarOpen(first)
	w.dayOpen(first, 1)

	w.press("e")
	shows := w.until("an entry of the day is to be picked", picking("edit"))

	if lineWith(shows, "›") != "Calendar › Correct an entry" {
		t.Errorf("the breadcrumb says %q", lineWith(shows, "›"))
	}

	if row := lineWith(shows, "> Consulting"); row == "" {
		t.Errorf("the cursor is not on the day's first entry:\n%s", shows)
	}

	w.press("enter")
	w.until("the hours are asked", asking("hours"))
	w.press("backspace", "backspace", "backspace", "backspace", "3", "enter")
	w.until("the project is asked", asking("project"))
	w.press("enter")
	w.until("the day is asked", asking("day"))
	w.press("enter")
	w.until("the description is asked", asking("description"))
	w.press("enter")

	w.until("the correction is saved", func(m screen) bool { return settled(m) && m.news != "" && m.cal.day == dayOf(first, 1) })

	sent := sentOnce(t, f, "PUT /timesheets/11")
	if sent["durationHours"] != 3.0 || sent["projectId"] != 4.0 || sent["date"] != dayOf(first, 1) {
		t.Errorf("the correction was sent as %v", sent)
	}

	w.close()
}

// An entry of an open day is deleted there once the answer is yes - the day's
// own, which on the tenth is the month's third and not the first the list holds.
func TestAnEntryOfADayIsDeletedAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.calendarOpen(first)
	w.dayOpen(first, 9)

	w.press("d")
	w.until("an entry of the day is to be picked", picking("delete"))
	w.press("enter")
	w.until("the deletion is asked about", asking("confirm"))
	w.press("n")
	w.until("the menu is back", func(m screen) bool { return settled(m) && m.form == nil && m.picking == "" })

	if len(f.sentTo("DELETE /timesheets/13")) > 0 {
		t.Fatal("an entry was deleted although the answer was no")
	}

	w.press("d")
	w.until("an entry of the day is to be picked again", picking("delete"))
	w.press("enter")
	w.until("the deletion is asked about again", asking("confirm"))
	w.press("y")
	w.until("the entry is deleted", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /timesheets/13")) != 1 {
		t.Errorf("the tenth's entry was not deleted once, and the month's first was sent %v",
			f.sentTo("DELETE /timesheets/11"))
	}

	w.close()
}

// An account that may not book sees a day's entries and is offered nothing to
// change in them.
func TestAnAccountThatMayNotBookOnlyReadsADay(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own", "timesheets:read:own"}
	first := monthHere(t)
	aMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.calendarOpen(first)
	shows := w.dayOpen(first, 1)

	if offered(shows, "Correct an entry", "e") || offered(shows, "Delete an entry", "d") {
		t.Errorf("an account that may not book is offered to change a day's entries:\n%s", shows)
	}

	if lineWith(shows, "Consulting", "2.50") == "" {
		t.Errorf("the day's entries are not shown:\n%s", shows)
	}

	w.close()
}
