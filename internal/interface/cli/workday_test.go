package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// fakeDocument stands in for the PDF the installation lays out.
const fakeDocument = "%PDF a document"

// aWorkingMonth has the fake installation answer the month's working time: its
// second day recorded from eight to half past four with half an hour of breaks
// and past the ten hours the Act allows, and its tenth booked on without any
// times.
func aWorkingMonth(f *fakeInstallation, first time.Time) {
	f.answers = map[string]string{"GET /me/workdays": `{"days":[` +
		`{"date":"` + dayOf(first, 1) + `","recorded":true,"start":"08:00","end":"16:30","breakMinutes":30,` +
		`"workedHours":8,"bookedHours":3.5,"notices":[{"rule":"longDay","minutes":0}]},` +
		`{"date":"` + dayOf(first, 9) + `","recorded":false,"breakMinutes":0,"workedHours":0,"bookedHours":0.5,"notices":[]}` +
		`],"totalWorked":8,"totalBooked":4}`}
}

// workOpen opens the working time's menu under the calendar.
func (w *watched) workOpen(first time.Time) string {
	w.t.Helper()

	shows := w.calendarOpen(first)
	if !offered(shows, "Working time …", "z") {
		w.t.Fatalf("the working time is not offered:\n%s", shows)
	}

	w.press("z")

	return w.until("the working time's menu is open", func(m screen) bool { return settled(m) && m.sub == subWork })
}

// The working time is shown as the page's card shows it: a row a day with its
// start, end, breaks and the hours worked and booked, a day booked on without
// times as dashes, a limit of the Working Hours Act said under its day, and what
// the month came to.
func TestTheWorkingTimeIsShownAsThePageShowsIt(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aWorkingMonth(f, first)

	w := watch(t, server.URL, f.token)
	shows := w.workOpen(first)

	if row := lineWith(shows, dayOf(first, 1)); !strings.Contains(row, "08:00") || !strings.Contains(row, "16:30") ||
		!strings.Contains(row, "30") || !strings.Contains(row, "8.00 h") || !strings.Contains(row, "3.50 h") {
		t.Errorf("the recorded day is not shown with its times:\n%s", shows)
	}

	if row := lineWith(shows, dayOf(first, 9)); strings.Count(row, "–") != 4 || !strings.Contains(row, "0.50 h") {
		t.Errorf("the day without times is not shown with dashes and its booked hours: %q\n%s", row, shows)
	}

	if !strings.Contains(shows, "More than ten hours (Working Hours Act § 3)") {
		t.Errorf("the limit the day went past is not said:\n%s", shows)
	}

	if !strings.Contains(prose(shows), "Worked 8.00 h, booked 4.00 h · 1 days past a limit of the Working Hours Act.") {
		t.Errorf("what the month came to is not said:\n%s", shows)
	}

	w.close()
}

// A day is recorded with its start, end and breaks as a clock shows them - an
// hour written without its leading nought is read as the page's field would -
// and a time that is not one is refused before anything is sent.
func TestADayIsRecordedWithItsTimes(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aWorkingMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.workOpen(first)

	w.press("n")
	w.until("the day is asked", asking("workDay"))
	w.press("backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace",
		"backspace", "backspace", dayOf(first, 4), "enter")
	w.until("the start is asked", asking("workStart"))
	w.press("25:00", "enter")

	// Waited for by what the field says, which only a refusal makes it say: a
	// screen still asking might only not have moved on yet.
	w.until("the time is refused", func(m screen) bool {
		return asking("workStart")(m) &&
			strings.Contains(ansi.Strip(m.View().Content), "Write a time as a clock shows it, such as 08:00.")
	})

	w.press("backspace", "backspace", "backspace", "backspace", "backspace", "8:00", "enter")
	w.until("the end is asked", asking("workEnd"))
	w.press("16:30", "enter")
	w.until("the breaks are asked", asking("workBreaks"))
	w.press("backspace", "30", "enter")

	shows := w.until("the day is recorded", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "PUT /me/workdays/"+dayOf(first, 4))
	if sent["start"] != "08:00" || sent["end"] != "16:30" || sent["breakMinutes"] != 30.0 {
		t.Errorf("the day was recorded as %v", sent)
	}

	if !strings.Contains(shows, "✓ Working time recorded") {
		t.Errorf("the screen does not say the day was recorded:\n%s", shows)
	}

	w.close()
}

// A day is corrected from its row, in the form filled with what it holds.
func TestADayIsCorrectedFromItsRow(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aWorkingMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.workOpen(first)

	w.press("e")
	w.until("a day is to be picked", picking("workEdit"))
	w.press("enter")

	// Asked of the answer rather than of the picture, where an empty field shows
	// 08:00 as its placeholder.
	w.until("the start is asked, filled with the day's", func(m screen) bool {
		return asking("workStart")(m) && m.asked.workStart == "08:00"
	})

	w.press("enter")
	w.until("the end is asked", asking("workEnd"))
	w.press("backspace", "backspace", "backspace", "backspace", "backspace", "17:00", "enter")
	w.until("the breaks are asked", asking("workBreaks"))
	w.press("enter")
	w.until("the day is corrected", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "PUT /me/workdays/"+dayOf(first, 1))
	if sent["start"] != "08:00" || sent["end"] != "17:00" || sent["breakMinutes"] != 30.0 {
		t.Errorf("the day was corrected as %v", sent)
	}

	w.close()
}

// Only a day with times recorded can be taken back - one booked on alone has
// nothing to take - and it is, once the answer is yes.
func TestARecordedDayIsTakenBackAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aWorkingMonth(f, first)

	w := watch(t, server.URL, f.token)
	w.workOpen(first)

	w.press("d")
	w.until("a day is to be picked", picking("workDelete"))
	w.press("down", "enter")

	shows := prose(w.until("taking it back is asked about", asking("confirm")))
	if !strings.Contains(shows, dayOf(first, 1)+" will be deleted.") {
		t.Errorf("the day without times could be picked, or the question does not name the day:\n%s", shows)
	}

	w.press("y")
	w.until("the day is taken back", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /me/workdays/"+dayOf(first, 1))) != 1 {
		t.Errorf("the recorded day was not taken back once: %v", f.sentTo("DELETE /me/workdays/"+dayOf(first, 1)))
	}

	w.close()
}

// The working time is laid out as the page's document, from what the card shows,
// in the client's language, and saved where it was asked to go - never over a
// file that is already there.
func TestTheWorkingTimeIsExportedAsADocument(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	aWorkingMonth(f, first)

	path := filepath.Join(t.TempDir(), "month.pdf")

	w := watch(t, server.URL, f.token)
	w.workOpen(first)

	w.press("p")
	shows := w.until("where it goes is asked", asking("file"))

	if !strings.Contains(shows, "working-time-") {
		t.Errorf("the file is not named as the page names it:\n%s", shows)
	}

	for range 60 {
		w.press("backspace")
	}

	w.press(path, "enter")

	// The path is asked of what the screen says rather than of its picture,
	// where a long one wraps.
	shows = w.until("the document is saved, and where is said", func(m screen) bool {
		return settled(m) && strings.Contains(m.news, path)
	})

	written, err := os.ReadFile(path)
	if err != nil || string(written) != fakeDocument {
		t.Fatalf("the document was not saved as it came: %q, %v", written, err)
	}

	sent := sentOnce(t, f, "POST /exports/document")
	if sent["title"] != "Working time" || sent["subtitle"] != first.Format("January 2006") {
		t.Errorf("the document is headed %v / %v", sent["title"], sent["subtitle"])
	}

	sections, _ := sent["sections"].([]any)
	section, _ := sections[0].(map[string]any)
	table, _ := section["table"].(map[string]any)
	rows, _ := table["rows"].([]any)

	if len(rows) != 2 || !strings.Contains(strings.Join(anyStrings(rows[0]), " "), "08:00") {
		t.Errorf("the document's table is not the card's: %v", table)
	}

	if !strings.Contains(shows, "✓ Export saved") {
		t.Errorf("the screen does not say the document was saved:\n%s", shows)
	}

	w.close()
}

// anyStrings is a decoded JSON array of strings.
func anyStrings(v any) []string {
	values, _ := v.([]any)
	said := make([]string, len(values))

	for i, value := range values {
		said[i], _ = value.(string)
	}

	return said
}
