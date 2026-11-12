//go:build browser

package browser

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/network"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Inside the Android app the page tells the app when its session ended without
// its being signed out of - the token it was opened with taken back, or the
// session run out - which the app cannot see for itself; and it does not tell
// it when somebody signs out, which the app sees in the requests and answers by
// disconnecting. Told on signing out as well, the app would open a new session
// with its token and sign straight back in whoever had just signed out.
func TestThePageTellsTheAppItsSessionEnded(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	// The bridge the app registers, stood in for: it counts in the tab's storage,
	// which outlives a document loaded afresh.
	p.run("load the page as the app shows it", chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := cdppage.AddScriptToEvaluateOnNewDocument(`window.GoTicketSystemSession = {
			ended() { sessionStorage.setItem('ended', String(Number(sessionStorage.getItem('ended') || 0) + 1)); },
		};`).Do(ctx)

		return err
	}), chromedp.Reload())
	p.settled()

	// The session gone from under the page, as taking its token back takes it: the
	// next thing the page asks is refused.
	p.run("end the session underneath the page", network.DeleteCookies("gts_session").WithURL(p.app.BaseURL()))
	p.awaitJSON(`api('/me').then(() => 'answered', () => 'refused')`, nil)
	p.waitEvaluates("what the app was told", `sessionStorage.getItem('ended') ?? '0'`, "1")

	p.signIn(workerEmail, workerPassword)
	p.waitGone("#login-screen")
	p.settled()

	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))
	p.settled()

	var told string

	p.run("read what the app was told", chromedp.Evaluate(`sessionStorage.getItem('ended') ?? '0'`, &told))

	if told != "1" {
		t.Errorf("signing out told the app %s time(s) in all that its session had ended, want once - for "+
			"the session that ended underneath the page, and not for the one signed out of", told)
	}
}
