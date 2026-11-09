package cli

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// aWeek has the fake installation answer a week with something in it: on its
// Monday 2.5 hours on Consulting and two entries on Website, on Tuesday an hour
// on the archived project, and on Wednesday half an hour on none.
func aWeek(f *fakeInstallation, monday time.Time) {
	f.list = listOf(0,
		booking{11, 4, dayOf(monday, 0), 2.5},
		booking{13, 1, dayOf(monday, 0), 1},
		booking{14, 1, dayOf(monday, 0), 0.5},
		booking{16, 3, dayOf(monday, 1), 1},
		booking{15, 0, dayOf(monday, 2), 0.5},
	)
}

// onCell is a screen picking a cell of the week grid, with the cursor in the row
// of key on the day so many days after Monday.
func onCell(key string, day int) func(screen) bool {
	return func(m screen) bool { return m.picking == "cell" && m.cell.row == key && m.cell.day == day }
}

// cellOpen offers to enter hours, picks the cell of key on the day so many days
// after Monday - going to Monday first, wherever the cursor started - and waits
// for what enter on it does.
func (w *watched) cellOpen(key string, day int) {
	w.t.Helper()

	w.press("e")
	w.until("a cell is to be picked", picking("cell"))
	w.press("left", "left", "left", "left", "left", "left")

	for range day {
		w.press("right")
	}

	w.until("the cursor is on the cell", onCell(key, day))
	w.press("enter")
}

// A number in an empty cell books it on the row's project and the column's day,
// and the cursor stays in the grid for the next cell, as the page keeps the
// focus there.
func TestHoursTypedIntoAnEmptyCellBookThem(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if !offered(shows, "Enter hours", "e") {
		t.Fatalf("entering hours is not offered:\n%s", shows)
	}

	w.cellOpen("4", 2)

	shows = w.until("the hours are asked", asking("hours"))
	if lineWith(shows, "›") != "Week › Enter hours › Hours" {
		t.Errorf("the breadcrumb says %q", lineWith(shows, "›"))
	}

	if !strings.Contains(shows, "Consulting · ") {
		t.Errorf("the form does not say which cell it is for:\n%s", shows)
	}

	w.press("2", "enter")
	shows = w.until("the hours are booked", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "POST /timesheets")
	if sent["projectId"] != 4.0 || sent["durationHours"] != 2.0 || sent["date"] != dayOf(monday, 2) {
		t.Errorf("the cell was booked as %v, want 2 hours on Consulting on %s", sent, dayOf(monday, 2))
	}

	if !strings.Contains(shows, "✓ Time booked") {
		t.Errorf("the screen does not say the time was booked:\n%s", shows)
	}

	w.until("the cursor is still in the grid", onCell("4", 2))
	w.press("esc")
	w.close()
}

// A changed number in a cell with one entry behind it corrects that entry, and
// the same number again changes nothing.
func TestAChangedFigureCorrectsTheEntryBehindIt(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	w.weekOpen(monday)

	w.cellOpen("4", 0)
	w.until("the hours are asked", asking("hours"))
	w.press("enter")
	w.until("the cursor is back in the grid", func(m screen) bool { return settled(m) && m.picking == "cell" })

	if len(f.sentTo("PUT /timesheets/11")) > 0 {
		t.Fatal("the same number was sent as a correction")
	}

	w.press("enter")
	w.until("the hours are asked again", asking("hours"))
	w.press("backspace", "backspace", "backspace", "3", "enter")
	w.until("the entry is corrected", func(m screen) bool { return settled(m) && m.news != "" })

	if sent := sentOnce(t, f, "PUT /timesheets/11"); sent["durationHours"] != 3.0 || len(sent) != 1 {
		t.Errorf("the entry was corrected with %v, want its hours alone, as 3", sent)
	}

	w.press("esc")
	w.close()
}

// An emptied cell takes the entry behind it back, once the answer to whether it
// should is yes - in the page's words.
func TestAnEmptiedCellTakesItsEntryBackAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	w.weekOpen(monday)

	w.cellOpen("4", 0)
	w.until("the hours are asked", asking("hours"))
	w.press("backspace", "backspace", "backspace", "enter")

	shows := prose(w.until("taking it back is asked about", asking("confirm")))
	if !strings.Contains(shows, "Take this entry back?") || !strings.Contains(shows, "The 2.50 h booked on "+dayOf(monday, 0)+" are deleted.") {
		t.Errorf("the screen does not ask in the page's words:\n%s", shows)
	}

	w.press("n")
	w.until("the cursor is back in the grid", func(m screen) bool { return settled(m) && m.picking == "cell" })

	if len(f.sentTo("DELETE /timesheets/11")) > 0 {
		t.Fatal("the entry was taken back although the answer was no")
	}

	w.press("enter")
	w.until("the hours are asked again", asking("hours"))
	w.press("backspace", "backspace", "backspace", "enter")
	w.until("taking it back is asked about again", asking("confirm"))
	w.press("y")

	shows = w.until("the entry is taken back", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /timesheets/11")) != 1 {
		t.Errorf("the entry was not taken back once: %v", f.sentTo("DELETE /timesheets/11"))
	}

	if !strings.Contains(shows, "✓ Entry taken back") {
		t.Errorf("the screen does not say the entry was taken back:\n%s", shows)
	}

	w.press("esc")
	w.close()
}

// A cell that several entries make up is not one a number can be typed into -
// there is no telling which of them it would mean - and leads to them instead:
// the time entries, narrowed to its day and its project, with their menu open.
func TestACellOfSeveralEntriesLeadsToThem(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	w.weekOpen(monday)

	w.press("e")
	w.until("a cell is to be picked", picking("cell"))
	w.press("down", "left", "left", "left", "left", "left", "left")
	w.until("the cursor is on Website's Monday", onCell("1", 0))
	w.press("enter")

	want := entryFilter{from: dayOf(monday, 0), to: dayOf(monday, 0), project: "1"}
	shows := w.until("the entries are shown", func(m screen) bool {
		return settled(m) && m.currentTab().view == viewTimesheets && m.sub == subEntries && m.card.filter == want
	})

	asked := false
	for _, sent := range f.sentTo("GET /timesheets") {
		q, _ := sent["?"].(url.Values)
		asked = asked || (q.Get("from") == want.from && q.Get("to") == want.to && q.Get("projectId") == "1")
	}

	if !asked {
		t.Errorf("the entries of the cell's day and project were not asked for: %v", f.sentTo("GET /timesheets"))
	}

	if !strings.Contains(shows, "Website · "+dayOf(monday, 0)) {
		t.Errorf("the entries card does not say what it is narrowed to:\n%s", shows)
	}

	w.close()
}

// A closed project's row takes no hours, as the page shows its cells as figures
// rather than fields: the cursor passes over it.
func TestAClosedProjectsCellsTakeNoHours(t *testing.T) {
	f, server := newFakeInstallation(t)
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	w.weekOpen(monday)

	w.press("e")
	w.until("a cell is to be picked", picking("cell"))
	w.press("left", "left", "left", "left", "left", "left")
	w.until("the cursor starts in the first row that takes hours", onCell("4", 0))

	w.press("up")
	w.until("the cursor stays out of the archived project's row", func(m screen) bool {
		return onCell("4", 0)(m) && settled(m)
	})

	w.press("esc")
	w.until("the menu is back", func(m screen) bool { return settled(m) && m.picking == "" })
	w.close()
}

// An account that may not book its own time is offered no cell to type into -
// only the way to the entries behind a cell of several, which the page links to
// for anybody who can see the grid.
func TestAnAccountThatMayNotBookIsOfferedOnlyTheEntriesBehindACell(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own", "timesheets:read:own"}
	monday := mondayHere(t)
	aWeek(f, monday)

	w := watch(t, server.URL, f.token)
	shows := w.weekOpen(monday)

	if offered(shows, "Enter hours", "e") {
		t.Errorf("an account that may not book is offered to enter hours:\n%s", shows)
	}

	if !offered(shows, "Show a cell's entries", "e") {
		t.Errorf("an account that may not book is not offered the entries behind a cell:\n%s", shows)
	}

	w.press("e")
	w.until("a cell is to be picked", picking("cell"))
	w.until("the cursor is on the one cell of several", onCell("1", 0))

	w.press("esc")
	w.close()
}
