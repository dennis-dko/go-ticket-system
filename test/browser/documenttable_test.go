//go:build browser

package browser

import (
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// A statement line whose project has no rate yet is in the statement's document.
//
// The screen lists its hours with "no rate yet" where the rate would be, which is
// what the statement promises: what the hours come to is not guessed, and the
// hours are not hidden. The document left the line out, because the reader of a
// table skipped every row holding a cell marked empty - meant for the line a table
// shows when it has nothing to show - and "no rate yet" is marked the same way. A
// statement of work sent on without one project's hours is wrong in the way
// nobody checks.
//
// The line a table shows when it has nothing to show stays out, which is the
// other half of the same question: it is a message, not a row of figures.
func TestAStatementLineWithNoRateYetIsInItsDocument(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	p.catchDocuments()

	p.run("work out a statement of nothing", p.click(`.tab[data-view="report"]`),
		p.click(`#report-tabs [data-card-tab="statement"]`),
		chromedp.WaitVisible("#form-statement", chromedp.ByID),
		p.click(`#form-statement button[type="submit"]`))
	p.waitForText("#table-statement tbody", "No billable hours in this period.")

	before := p.documentsAsked()

	p.run("export the statement of nothing", p.click("#statement-pdf"))

	if rows := p.documentAsked(before, "#statement-pdf").table(t, 0).Rows; len(rows) != 0 {
		t.Errorf("a statement with no billable hours holds the rows %q, want none: the line "+
			"saying so is a message", rows)
	}

	var booked bool

	p.awaitJSON(`(async () => {
		const project = await api('/projects', { method: 'POST', body: JSON.stringify({
			name: 'Gutachten', startDate: todayISO(), billable: true }) });
		await api('/timesheets', { method: 'POST', body: JSON.stringify({
			projectId: project.id, date: todayISO(), durationHours: 2 }) });
		return true;
	})()`, &booked)

	p.run("work out the statement", p.click(`#form-statement button[type="submit"]`))
	p.waitForText("#table-statement tbody", "Gutachten")

	before = p.documentsAsked()

	p.run("export the statement", p.click("#statement-pdf"))

	table := p.documentAsked(before, "#statement-pdf").table(t, 0)

	if len(table.Rows) != 1 || len(table.Rows[0]) == 0 || table.Rows[0][0] != "Gutachten" {
		t.Errorf("the statement's document holds the rows %q, want the line for Gutachten, "+
			"whose project has no rate yet", table.Rows)
	}
}

// askedDocument is a document as the page asked for it to be laid out.
type askedDocument struct {
	Subtitle string `json:"subtitle"`
	Sections []struct {
		Table *askedTable `json:"table"`
	} `json:"sections"`
	Summary []struct {
		Label string `json:"label"`
		Value string `json:"value"`
		Tone  string `json:"tone"`
	} `json:"summary"`
}

// askedTable is one table of an asked document.
type askedTable struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	Tones   [][]string `json:"tones"`
}

// table is the table of the section at index, which the case needs to be there.
func (d askedDocument) table(t *testing.T, index int) askedTable {
	t.Helper()

	if index >= len(d.Sections) || d.Sections[index].Table == nil {
		t.Fatalf("the document has no table in section %d: %+v", index, d.Sections)
	}

	return *d.Sections[index].Table
}

// documentAsked waits for a document after the first before, and returns it.
func (p *page) documentAsked(before int, button string) askedDocument {
	p.t.Helper()

	deadline := time.Now().Add(waitPatience)

	for time.Now().Before(deadline) {
		var arrived bool

		p.run("look for the document", chromedp.Evaluate(
			`window.__documents.length > `+strconv.Itoa(before), &arrived))

		if arrived {
			var asked askedDocument

			p.run("read the document", chromedp.Evaluate(`window.__documents.at(-1)`, &asked))

			return asked
		}

		time.Sleep(100 * time.Millisecond)
	}

	p.t.Fatalf("%s never asked for a document", button)

	return askedDocument{}
}
