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

// Choosing another file takes the last file's verdict off the screen for good.
//
// The preview is drawn through redrawable, so a language change re-renders it
// without sending the file again - and stopRedrawing is how a card says the
// answer it held is gone. The card's reset calls it. Choosing a different file
// makes exactly the same thing true - the change handler hides the preview and
// empties the summary - and once did not call it.
//
// So the draw stayed registered against the old result, and the next language
// change put it back on screen: the verdict for a file nobody has chosen any
// more, sitting above a file nobody has checked. If that verdict was clean the
// Import button comes back with it, and it would upload the newly chosen file,
// whose rows have never been looked at. The card exists to show what a file
// would do before it does it.
func TestChoosingAnotherFileDoesNotBringTheOldVerdictBack(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	write := func(name string, rows []spreadsheet.UserRow) string {
		t.Helper()

		book, err := spreadsheet.WriteUsers("en", rows)
		if err != nil {
			t.Fatalf("building %s: %v", name, err)
		}

		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, book, 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}

		return path
	}

	first := write("first.xlsx", []spreadsheet.UserRow{
		{Name: "The file that was checked", Email: "checked@example.com", Role: "agent"},
	})

	second := write("second.xlsx", []spreadsheet.UserRow{
		{Name: "The file that was not", Email: "unchecked@example.com", Role: "agent"},
	})

	const card = `#view-users .card:has(input[type="file"])`

	p.run("check the first file", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible(card, chromedp.ByQuery),
		chromedp.SetUploadFiles(card+` input[type="file"]`, []string{first}, chromedp.ByQuery),
		p.click(card+` button[data-i18n="wb.preview"]`))

	p.waitShown(card + " .table-wrap")

	p.run("choose a different file",
		chromedp.SetUploadFiles(card+` input[type="file"]`, []string{second}, chromedp.ByQuery))

	p.waitGone(card + " .table-wrap")

	// The real trigger: a language change redraws every card that registered one.
	p.run("read the page in German", chromedp.Evaluate(`(() => {
		const picker = document.querySelector('#language-picker');
		picker.value = 'de';
		picker.dispatchEvent(new Event('change', { bubbles: true }));

		return picker.value;
	})()`, nil))

	// Long enough for the redraw to have happened if it was going to.
	time.Sleep(2 * time.Second)

	if p.visible(card + " .table-wrap") {
		t.Errorf("a language change put the previous file's verdict back on screen " +
			"while a different, unchecked file is chosen. The change handler hides the " +
			"preview without calling stopRedrawing, which the card's reset has")
	}
}
