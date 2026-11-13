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

// A session that has ended takes the screen with it, uploads included.
//
// api() states the rule and enforces it for every request that carries JSON:
// "A session that is no longer accepted takes the screen with it." Some
// requests do not go through api() - the download and the imports - because each
// has a reason of its own: a blob to save, a multipart boundary only the browser
// knows. Each was then written out by hand, and each of the copies stopped at the
// message. None of them notices a 401, a maintenance 503 or the
// permission revision that travels on every answer.
//
// The upload is the worst place to lose that. It is the longest-running thing
// this interface does, so it is the request most likely to be the one that finds
// the session gone - and what it showed instead was a red toast on a screen that
// still looked signed in, with the file still chosen and the Import button still
// offering to try again.
func TestAnEndedSessionDuringAnUploadHandsBackTheScreen(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	// A perfectly good file: this case is about the answer, not the contents.
	book, err := spreadsheet.WriteUsers("en", []spreadsheet.UserRow{
		{Name: "Fine either way", Email: "fine@example.com", Role: "agent"},
	})
	if err != nil {
		t.Fatalf("building the workbook: %v", err)
	}

	path := filepath.Join(t.TempDir(), "people.xlsx")
	if err := os.WriteFile(path, book, 0o600); err != nil {
		t.Fatalf("writing the workbook: %v", err)
	}

	// The session, gone by the time the upload lands. Only the import is answered
	// this way, so everything that got the page here is left alone.
	p.run("the session ends underneath the upload", chromedp.Evaluate(`(() => {
		const real = window.fetch;

		window.fetch = async (input, init) => {
			const url = typeof input === 'string' ? input : input.url;

			if (url.includes('/users/import')) {
				return new Response(JSON.stringify({ error: {
					code: 'sessionExpired',
					message: 'The session has ended. Please sign in again.',
				} }), { status: 401, headers: { 'Content-Type': 'application/json' } });
			}

			return real(input, init);
		};

		return true;
	})()`, nil))

	const card = `#view-users .card:has(input[type="file"])`

	p.run("choose the file", p.click(`.tab[data-view="users"]`),
		chromedp.WaitVisible(card, chromedp.ByQuery),
		chromedp.SetUploadFiles(card+` input[type="file"]`, []string{path}, chromedp.ByQuery))

	p.waitShown(card + ` button[data-i18n="wb.preview"]`)
	p.run("check the file", p.click(card+` button[data-i18n="wb.preview"]`))

	// Given time to hand the screen back, rather than asserted on immediately.
	deadline := time.Now().Add(waitPatience)
	for time.Now().Before(deadline) {
		if p.visible("#login-screen") {
			return
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Error("the upload was answered with 401 and the interface stayed signed in. " +
		"api() hands the screen back on that status; the import asks with a raw " +
		"fetch and reads only the message out of the answer, so the session was " +
		"gone and nothing on screen said so")
}
