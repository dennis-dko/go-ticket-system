//go:build browser

package browser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/go-ticket-system/internal/support/spreadsheet"
)

// A form pressed twice writes once.
//
// The form sends its write and resets itself when the answer comes, and nothing
// stood between the two: a double click - or Enter pressed twice - sent the
// second request before the first had been answered, and the same record was
// written twice and shown as two rows the reader had to notice and delete.
// Measured, with a booking: two entries, created the same second. A personal
// token is the write here, because every account can make one.
func TestATokenPressedTwiceIsMadeOnce(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.showView("settings")
	p.waitShown("#form-token")

	p.run("fill in a token",
		chromedp.SendKeys(`#form-token input[name="name"]`, "pressed twice", chromedp.ByQuery))

	p.run("press it twice", chromedp.Evaluate(`(() => {
		const button = document.querySelector('#form-token button[type="submit"]');
		button.click();
		button.click();
		return true;
	})()`, nil))

	// Done once the form has been reset by the answer and nothing is in flight -
	// the page's own count of requests - so a second request, had one been sent,
	// has been answered too.
	done := false

	for deadline := time.Now().Add(waitPatience); !done && time.Now().Before(deadline); {
		p.run("see whether the token is answered", chromedp.Evaluate(`progress.inFlight === 0
			&& document.querySelector('#form-token input[name="name"]').value === ''`, &done))

		if !done {
			time.Sleep(50 * time.Millisecond)
		}
	}

	if !done {
		t.Fatal("the token was never answered")
	}

	var count int

	p.run("ask the server how many there are", chromedp.Evaluate(`(async () => {
		const res = await fetch('/api/v1/me/tokens', { credentials: 'same-origin' });
		const body = await res.json();
		return (body.data.items ?? []).filter((e) => e.name === 'pressed twice').length;
	})()`, &count, awaitPromise))

	if count != 1 {
		t.Errorf("one token pressed twice was made %d times", count)
	}
}

// An import pressed twice imports once.
//
// The same hole as a form, with every row of the file behind it: the import asks
// fetch directly, with the file as its body, and a second press sent the file
// again before the first had been answered. A role import matches its rows by
// name, so a second one would change nothing anybody could count in the roles -
// which is why this counts what the administration log recorded instead: one
// import is one entry there.
func TestAnImportPressedTwiceImportsOnce(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	book, err := spreadsheet.WriteRoles("", []string{"tickets:read"}, []spreadsheet.RoleRow{
		{Name: "imported-once", Description: "imported once", Granted: []string{"tickets:read"}},
	})
	if err != nil {
		t.Fatalf("building the workbook: %v", err)
	}

	path := filepath.Join(t.TempDir(), "once.xlsx")
	if err := os.WriteFile(path, book, 0o600); err != nil {
		t.Fatalf("writing the workbook: %v", err)
	}

	const card = `#view-roles .card:has(input[type="file"])`

	p.showView("roles")
	p.waitShown(card)

	p.run("choose the file",
		chromedp.SetUploadFiles(card+` input[type="file"]`, []string{path}, chromedp.ByQuery))

	p.waitShown(card + ` button[data-i18n="wb.preview"]`)

	p.run("check it", p.click(card+` button[data-i18n="wb.preview"]`))
	p.waitShown(card + ` button[data-i18n="wb.import"]`)

	// Counted from here rather than from nothing, so that what the check left in
	// the log is not taken for a second import.
	imports := func() int {
		var count int

		p.run("ask the log how many imports there were", chromedp.Evaluate(`(async () => {
			const res = await fetch('/api/v1/admin/changes', { credentials: 'same-origin' });
			const body = await res.json();
			return (body.data.entries ?? []).filter((e) => e.action === 'roles.imported').length;
		})()`, &count, awaitPromise))

		return count
	}

	before := imports()

	p.run("import it twice", chromedp.Evaluate(`(() => {
		const button = document.querySelector('`+card+` button[data-i18n="wb.import"]');
		button.click();
		button.click();
		return true;
	})()`, nil))

	// Done once the card has been reset by the answer and nothing is in flight.
	done := false

	for deadline := time.Now().Add(waitPatience); !done && time.Now().Before(deadline); {
		p.run("see whether the import is answered", chromedp.Evaluate(
			`progress.inFlight === 0 && document.querySelector('`+card+` button[data-i18n="wb.import"]').hidden`, &done))

		if !done {
			time.Sleep(50 * time.Millisecond)
		}
	}

	if !done {
		t.Fatal("the import was never answered")
	}

	if made := imports() - before; made != 1 {
		t.Errorf("one import pressed twice was made %d times", made)
	}
}
