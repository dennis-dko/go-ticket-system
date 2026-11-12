//go:build browser

package browser

import (
	"context"
	"testing"

	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// In the Android app the Reminders card asks the phone, not a browser, for
// notifications. The app's WebView has no Notification of its own and registers
// GoTicketSystemNotifications instead, which this stands in for before the page
// loads, answering as the app does. The card is held to it: the app's button, a
// press that goes to the app rather than to a browser, and the app's sentence
// once the phone has answered - which the app says with gts-notifications.
func TestTheRemindersCardAsksTheAppForItsNotifications(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	asTheAppShowsIt := chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := cdppage.AddScriptToEvaluateOnNewDocument(`(() => {
			window.askedTheApp = 0;
			window.thePhoneSays = 'default';
			window.GoTicketSystemNotifications = {
				state: () => window.thePhoneSays,
				request: () => { window.askedTheApp += 1; },
			};
		})()`).Do(ctx)

		return err
	})

	p.run("load the page as the app shows it", asTheAppShowsIt, chromedp.Reload())
	p.settled()
	p.showView("settings")
	p.waitShown("#notify-allow")

	var label string

	p.run("read the button", chromedp.Text("#notify-allow", &label, chromedp.ByID))

	if label != "Allow notifications from this app" {
		t.Errorf("in the app the button reads %q, which is a browser's question", label)
	}

	p.run("press it", p.click("#notify-allow"))

	var asked int

	p.awaitJSON(`window.askedTheApp`, &asked)

	if asked != 1 {
		t.Fatalf("the press asked the app %d times, so the phone is never asked", asked)
	}

	p.run("the phone allows them", chromedp.Evaluate(
		`window.thePhoneSays = 'granted'; window.dispatchEvent(new Event('gts-notifications'))`, nil))
	p.waitGone("#notify-allow")

	var said string

	p.run("read what the card says", chromedp.Text("#notify-state", &said, chromedp.ByID))

	if said != "This app shows the reminders as notifications, whether it is open or not." {
		t.Errorf("once the phone allows them the card says %q", said)
	}
}
