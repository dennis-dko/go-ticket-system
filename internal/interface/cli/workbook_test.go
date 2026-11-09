package cli

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeWorkbook is what the fake installation exports, which is not a real
// workbook: the client saves what it is answered with, byte for byte.
const fakeWorkbook = "PK a workbook"

// refusedPlan is a file one of whose two rows is refused, as the installation
// answers a check of it.
const refusedPlan = `{"dryRun":true,"rows":[` +
	`{"row":2,"date":"2026-10-22","user":"dana@example.com","project":"Website","hours":1.5,"description":"x","problem":""},` +
	`{"row":3,"date":"2026-10-22","user":"dana@example.com","project":"Archive","hours":2,"description":"y",` +
	`"problem":"the project is closed for booking","problemCode":"projectClosedForBooking"}],` +
	`"writable":1,"rejected":1,"imported":0}`

// readyPlan is a file both of whose rows would be written.
const readyPlan = `{"dryRun":true,"rows":[` +
	`{"row":2,"date":"2026-10-22","user":"dana@example.com","project":"Website","hours":1.5,"description":"x","problem":""},` +
	`{"row":3,"date":"2026-10-23","user":"dana@example.com","project":"Website","hours":2,"description":"y","problem":""}],` +
	`"writable":2,"rejected":0,"imported":0}`

// workbookOpen opens the spreadsheet card's menu once the day is read.
func (w *watched) workbookOpen() string {
	w.t.Helper()

	w.until("the day is read", settled)
	w.press("t")

	return w.until("the spreadsheet's menu is open", func(m screen) bool { return settled(m) && m.sub == subWorkbook })
}

// The entries are exported as the entries card narrows them, as the page's
// export takes the list's filters, in the client's language, and saved under the
// page's name for the file - written as the installation answered it.
func TestTheEntriesAreExportedAsTheCardNarrowsThem(t *testing.T) {
	t.Chdir(t.TempDir())

	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.entriesOpen()
	w.press("o")
	w.until("the entries without a project are read", func(m screen) bool { return settled(m) && m.card.filter.project == "none" })
	w.press("esc")

	shows := w.workbookOpen()
	if !offered(shows, "Export as .xlsx", "e") {
		t.Fatalf("exporting is not offered:\n%s", shows)
	}

	w.press("e")
	shows = w.until("the file is asked", asking("file"))

	name := "time-entries-" + time.Now().In(berlin(t)).Format(time.DateOnly) + ".xlsx"
	if !strings.Contains(shows, name) {
		t.Errorf("the file is not named as the page names it, %s:\n%s", name, shows)
	}

	w.press("enter")
	shows = w.until("the export is saved", func(m screen) bool { return settled(m) && m.news != "" })

	if written, err := os.ReadFile(name); err != nil || string(written) != fakeWorkbook {
		t.Errorf("the export was saved as %q, %v", written, err)
	}

	asked, _ := sentOnce(t, f, "GET /timesheets/export")["?"].(url.Values)
	if asked.Get("projectId") != "none" || asked.Get("lang") != "en" {
		t.Errorf("the export was asked for as %v, want the card's filter in English", asked)
	}

	if !strings.Contains(shows, "✓ Export saved") {
		t.Errorf("the screen does not say the export was saved:\n%s", shows)
	}

	w.close()
}

// An export is not written over a file that is there already.
func TestAnExportIsNotWrittenOverAFile(t *testing.T) {
	t.Chdir(t.TempDir())

	f, server := newFakeInstallation(t)

	name := "time-entries-" + time.Now().In(berlin(t)).Format(time.DateOnly) + ".xlsx"
	if err := os.WriteFile(name, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}

	w := watch(t, server.URL, f.token)
	w.workbookOpen()
	w.press("e")
	w.until("the file is asked", asking("file"))
	w.press("enter")
	w.until("the name is refused", func(m screen) bool { return asking("file")(m) && m.form.Errors() != nil })

	if kept, _ := os.ReadFile(name); string(kept) != "kept" {
		t.Errorf("the file there was written over: %q", kept)
	}

	if len(f.sentTo("GET /timesheets/export")) > 0 {
		t.Error("the export was asked for although it could not be saved")
	}

	w.press("esc")
	w.close()
}

// A file is checked before it is imported, as the page checks it: sent with
// dryRun, its rows shown with why each refused one is, and nothing offered to
// write while any row is refused.
func TestAFileIsCheckedBeforeItIsImported(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hours.xlsx")
	if err := os.WriteFile(file, []byte("PK the file"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, server := newFakeInstallation(t)
	f.imports = func(bool) string { return refusedPlan }

	w := watch(t, server.URL, f.token)
	shows := w.workbookOpen()

	if !offered(shows, "Import a file", "i") {
		t.Fatalf("importing is not offered:\n%s", shows)
	}

	w.press("i")
	w.until("the file is asked", asking("file"))
	w.press(file, "enter")

	shows = w.until("the file is checked", func(m screen) bool { return settled(m) && m.plan != nil })

	sent := sentOnce(t, f, "POST /timesheets/import")
	if sent["dryRun"] != "true" || sent["file"] != "hours.xlsx" || sent["content"] != "PK the file" {
		t.Errorf("the file was sent to be checked as %v", sent)
	}

	for _, want := range []string{"1 of 2 rows can be imported. Nothing is written while any row is refused.",
		"✗ the project is closed for booking", "Archive"} {
		if !strings.Contains(prose(shows), want) {
			t.Errorf("the check does not say %q:\n%s", want, shows)
		}
	}

	if offered(shows, "Import", "i") || !offered(shows, "Choose another file", "o") {
		t.Errorf("a file with a refused row is offered to be written, or no other file:\n%s", shows)
	}

	w.close()
}

// Why a row cannot be written is said in the page's words for it: the row's own
// sentence where the dictionary has one, the refusal's otherwise with its values
// written the way the page writes them, and the fields named in its words. In
// English it is the installation's own sentence.
func TestWhyARowIsRefusedIsSaidInThePagesWords(t *testing.T) {
	german := speaking("de")

	for _, tc := range []struct {
		row  importRow
		want string
	}{
		{importRow{Problem: "the date is missing", ProblemCode: "dateMissing"}, "Das Datum fehlt."},
		{importRow{Problem: "closed", ProblemCode: "projectClosedForBooking", ProblemValues: []any{"Archive", "completed"}},
			"Projekt „Archive“ ist abgeschlossen und nimmt keine Zeiteinträge mehr an."},
		{importRow{Problem: "invalid", ProblemCode: "invalidFields", ProblemValues: []any{"date", "durationHours"}},
			"Ungültige Felder: Datum, Stunden"},
	} {
		if said := german.rowProblem(tc.row); said != tc.want {
			t.Errorf("row %s is said as %q, want %q", tc.row.ProblemCode, said, tc.want)
		}
	}

	closed := importRow{Problem: "the project is closed for booking", ProblemCode: "projectClosedForBooking",
		ProblemValues: []any{"Archive", "completed"}}
	if said := speaking("en").rowProblem(closed); said != closed.Problem {
		t.Errorf("in English the row is said as %q, want the installation's own sentence", said)
	}
}

// A file all of whose rows can be written is imported once that is asked for:
// sent again without dryRun, and the screen says how many entries it made.
func TestAFileAllOfWhoseRowsCanBeWrittenIsImported(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hours.xlsx")
	if err := os.WriteFile(file, []byte("PK the file"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, server := newFakeInstallation(t)
	f.imports = func(dryRun bool) string {
		if dryRun {
			return readyPlan
		}

		return `{"dryRun":false,"rows":[],"writable":2,"rejected":0,"imported":2}`
	}

	w := watch(t, server.URL, f.token)
	w.workbookOpen()
	w.press("i")
	w.until("the file is asked", asking("file"))
	w.press(file, "enter")

	shows := w.until("the file is checked", func(m screen) bool { return settled(m) && m.plan != nil })
	if !strings.Contains(prose(shows), "All 2 rows can be imported.") || !offered(shows, "Import", "i") {
		t.Fatalf("a file all of whose rows can be written is not offered to be imported:\n%s", shows)
	}

	w.press("i")
	shows = w.until("the file is imported", func(m screen) bool { return settled(m) && m.news != "" && m.plan == nil })

	sent := f.sentTo("POST /timesheets/import")
	if len(sent) != 2 || sent[1]["dryRun"] == "true" || sent[1]["content"] != "PK the file" {
		t.Errorf("the file was not sent again to be written: %v", sent)
	}

	if !strings.Contains(shows, "✓ The file was imported. 2 entries created.") {
		t.Errorf("the screen does not say what the import made:\n%s", shows)
	}

	w.close()
}
