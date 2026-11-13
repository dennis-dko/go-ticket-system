//go:build browser

package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Five delete buttons once asked nothing at all - a role, a token and a passkey
// among them went straight to DELETE on one click. The four that did
// ask used window.confirm, which the browser draws itself: unstyled, naming the
// origin, unreadable in a dark theme and impossible to translate.
//
// Only a browser can check either half: that the question appears, and that
// answering "no" leaves the thing alone.
func TestDeletingAsksFirstAndCancellingChangesNothing(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	// A token of the account's own, so nothing else has to exist.
	p.makeTokens([]string{"kept-token"})

	// The delete button is a link button in the row's action cell.
	p.run("press delete", p.click(`#table-tokens tbody button.danger`),
		chromedp.WaitVisible(".confirm-overlay", chromedp.ByQuery))

	// The question is ours, not the browser's - so it is in the page and can be
	// seen, which a native dialog cannot be.
	if !p.visible(".confirm-card") {
		t.Fatal("no dialog appeared before deleting")
	}

	// Cancelling has to leave the token exactly where it was.
	p.run("cancel", p.click(`.confirm-actions button.secondary`))
	p.waitGone(".confirm-overlay")

	if !strings.Contains(p.text("#table-tokens tbody"), "kept-token") {
		t.Error("cancelling the dialog deleted the token anyway")
	}

	// And confirming deletes it, or the dialog is a wall rather than a question.
	p.run("press delete again", p.click(`#table-tokens tbody button.danger`),
		chromedp.WaitVisible(".confirm-overlay", chromedp.ByQuery))
	p.run("confirm", p.click(`.confirm-actions button.danger`))
	p.waitGone(".confirm-overlay")

	deadline := time.Now().Add(waitPatience)
	for time.Now().Before(deadline) {
		if !strings.Contains(p.text("#table-tokens tbody"), "kept-token") {
			return
		}

		time.Sleep(200 * time.Millisecond)
	}

	t.Error("confirming the dialog did not delete the token")
}

// Escape is the ambiguous keypress, and a dialog with a destructive option has
// to read it as "no".
func TestEscapeCancelsTheConfirmation(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.makeTokens([]string{"escaped-token"})

	p.run("press delete", p.click(`#table-tokens tbody button.danger`),
		chromedp.WaitVisible(".confirm-overlay", chromedp.ByQuery))

	p.run("press escape", chromedp.KeyEvent("\u001b"))
	p.waitGone(".confirm-overlay")

	if !strings.Contains(p.text("#table-tokens tbody"), "escaped-token") {
		t.Error("escape deleted the token")
	}
}
