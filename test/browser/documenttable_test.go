//go:build browser

package browser

import (
	"reflect"
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
// other half of the same question: it is a message, not a row of figures. And
// the rate the line does not have yet is in the grey the screen shows it in.
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

	p.everyColourIsCarried("#table-statement")

	before = p.documentsAsked()

	p.run("export the statement", p.click("#statement-pdf"))

	table := p.documentAsked(before, "#statement-pdf").table(t, 0)

	if len(table.Rows) != 1 || len(table.Rows[0]) == 0 || table.Rows[0][0] != "Gutachten" {
		t.Errorf("the statement's document holds the rows %q, want the line for Gutachten, "+
			"whose project has no rate yet", table.Rows)
	}

	if want := [][]string{{"", "", "muted", ""}}; !reflect.DeepEqual(table.Tones, want) {
		t.Errorf("the line's cells say %q with their colour, want %q - the rate it does not "+
			"have yet in the screen's grey", table.Tones, want)
	}
}

// Each limit of the Working Hours Act a day went past is its own sentence in the
// working time's document, as it is its own line on the screen.
//
// The cell holds one element per limit, set one under the other, and the reader
// of a table took the cell's text whole - which runs them together, so a day
// that was too long and had too short a break read "…(Working Hours Act § 3)45
// minutes of breaks needed…" on paper.
func TestEachLimitADayWentPastIsItsOwnSentenceInTheDocument(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	p.recordAMonthPastTheLimits()
	p.catchDocuments()

	before := p.documentsAsked()

	p.run("export the month", p.click("#workday-pdf"))

	table := p.documentAsked(before, "#workday-pdf").table(t, 0)

	const act = 6

	if len(table.Columns) <= act || table.Columns[act] != "Working Hours Act" {
		t.Fatalf("the document's columns are %q, want the Working Hours Act at %d", table.Columns, act)
	}

	want := "More than ten hours (Working Hours Act § 3) · 45 minutes of breaks needed (Working Hours Act § 4)"

	for _, row := range table.Rows {
		if len(row) > act && row[act] == want {
			return
		}
	}

	t.Errorf("no day of the document reads %q under the Working Hours Act; its rows are %q", want, table.Rows)
}

// The overtime document is in the colours of its screen: a day's balance in
// credit green and one in debt red, and the balance they come to as well.
//
// This is the screen the question was asked about. It colours the sign of every
// balance, and the document set them all in black, with the total in the
// installation's blue whichever way it went.
func TestTheOvertimeDocumentIsInTheColoursOfItsScreen(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	var booked bool

	p.awaitJSON(`(async () => {
		await api('/timesheets', { method: 'POST', body: JSON.stringify({ date: '2026-03-02', durationHours: 9 }) });
		await api('/timesheets', { method: 'POST', body: JSON.stringify({ date: '2026-03-03', durationHours: 7.5 }) });
		return true;
	})()`, &booked)

	p.run("work out that week's balance", p.click(`.tab[data-view="overtime"]`),
		chromedp.WaitVisible("#form-overtime", chromedp.ByID),
		chromedp.Evaluate(`(() => {
			const form = document.querySelector('#form-overtime');
			setDateField(form.elements.from, '2026-03-02');
			setDateField(form.elements.to, '2026-03-08');
		})()`, nil),
		p.click(`#form-overtime button[type="submit"]`))
	p.waitShown("#overtime-result")
	p.waitForRows("#table-overtime tbody tr", 2)

	p.everyColourIsCarried("#table-overtime")

	p.catchDocuments()
	before := p.documentsAsked()

	p.run("export the balance", p.click("#overtime-pdf"))

	asked := p.documentAsked(before, "#overtime-pdf")

	want := [][]string{{"", "", "", "plus"}, {"", "", "", "minus"}}

	if got := asked.table(t, 0).Tones; !reflect.DeepEqual(got, want) {
		t.Errorf("the overtime document's cells say %q with their colour, want %q - the day "+
			"an hour over in credit and the day half an hour short in debt", got, want)
	}

	if len(asked.Summary) != 1 || asked.Summary[0].Tone != "plus" {
		t.Errorf("the overtime document's summary is %+v, want the balance of half an hour "+
			"over in credit", asked.Summary)
	}
}

// The working time's document is in the colours of its screen: a day booked on
// with no working time recorded in the grey of what is quieter, and a limit of
// the Working Hours Act on its tint.
func TestTheWorkingTimeDocumentIsInTheColoursOfItsScreen(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	p.recordAMonthPastTheLimits()

	p.everyColourIsCarried("#table-workdays")

	p.catchDocuments()
	before := p.documentsAsked()

	p.run("export the month", p.click("#workday-pdf"))

	quiet := []string{"muted", "muted", "muted", "muted", "muted", "muted", "muted"}
	want := [][]string{
		{"", "", "", "", "", "", "notice"},
		{"", "", "", "", "", "", "notice"},
		quiet,
	}

	if got := p.documentAsked(before, "#workday-pdf").table(t, 0).Tones; !reflect.DeepEqual(got, want) {
		t.Errorf("the working time's document says %q with its colours, want %q - each limit "+
			"on its tint and the day with nothing recorded quieter", got, want)
	}
}

// everyColourIsCarried fails the case on a cell of the table that the screen
// draws apart from the plain ones - in another colour, in the bold face, or on a
// tint - and that has no tone, so its document would print it plain. It asks the
// page's own toneOf, so a colour given to a table the documents are made of is
// caught here rather than found missing on paper.
func (p *page) everyColourIsCarried(selector string) {
	p.t.Helper()

	var missed []string

	p.run("compare the table's colours with its tones", chromedp.Evaluate(`(() => {
		const table = document.querySelector('`+selector+`');
		const plain = window.getComputedStyle(table);
		const kept = [...table.querySelectorAll('thead th')]
			.map((heading, i) => (heading.matches('.actions, .pick') ? -1 : i))
			.filter((i) => i >= 0);
		const tinted = (cell) => [cell, ...cell.querySelectorAll('*')].some((part) => {
			const ground = window.getComputedStyle(part).backgroundColor;
			return ground !== 'rgba(0, 0, 0, 0)' && ground !== 'transparent';
		});

		const missed = [];

		for (const row of table.querySelectorAll('tbody tr')) {
			if (row.children.length === 1) continue;

			for (const i of kept) {
				const cell = row.children[i];
				const look = window.getComputedStyle(cell);
				const apart = look.color !== plain.color || Number(look.fontWeight) >= 600 || tinted(cell);

				if (apart && !toneOf(cell)) missed.push(cell.textContent.trim() || '(an empty cell)');
			}
		}

		return missed;
	})()`, &missed))

	if len(missed) > 0 {
		p.t.Errorf("%s draws %q apart on the screen, and its document would print them plain",
			selector, missed)
	}
}

// recordAMonthPastTheLimits records the first two days of the calendar's month
// and books the third without recording it, and opens the month: the first day
// too long and with too short a break, the second begun too soon after it.
func (p *page) recordAMonthPastTheLimits() {
	p.t.Helper()

	var recorded bool

	p.awaitJSON(`(async () => {
		const first = currentCalendarMonth();
		const day = (n) => ISO_DAY(new Date(first.getFullYear(), first.getMonth(), n));
		await api('/me/workdays/' + day(1), {
			method: 'PUT', body: JSON.stringify({ start: '07:00', end: '19:00', breakMinutes: 15 }),
		});
		await api('/me/workdays/' + day(2), {
			method: 'PUT', body: JSON.stringify({ start: '05:00', end: '09:00', breakMinutes: 0 }),
		});
		await api('/timesheets', { method: 'POST', body: JSON.stringify({ date: day(3), durationHours: 2 }) });
		await loadWorkDays();
		return true;
	})()`, &recorded)

	p.run("open the working time", p.click(`.tab[data-view="calendar"]`),
		p.click(`#calendar-tabs [data-card-tab="worktime"]`),
		chromedp.WaitVisible("#workday-card", chromedp.ByID))
	p.waitForText("#table-workdays tbody", "Only 10.00 h of rest since the day before")
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
