//go:build browser

package browser

import (
	"context"
	"testing"

	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/go-time-recording/internal/infrastructure/selfupdate"
	"github.com/dennis-dko/go-time-recording/internal/support/qrcode"
)

// The card on My account offers the Android app and nothing else: a code for a
// phone's camera that downloads it, and the release it comes from. The suite's
// instance runs no release, so both lead to the newest release's page - which is
// where a build from source is sent.
func TestTheInstallCardOffersTheAndroidAppAsACode(t *testing.T) {
	t.Parallel()

	page, _ := selfupdate.AndroidApp("dev")

	p := open(t)
	p.readyWorker()

	p.run("open my account", p.click(`.tab[data-view="settings"]`),
		chromedp.WaitVisible("#install-card", chromedp.ByID))
	p.waitShown("#install-qr")

	if want, err := qrcode.SVGDataURI(page); err != nil || p.attr("#install-qr", "src") != want {
		t.Errorf("the code on the card is not the code of %s", page)
	}

	if href := p.attr("#install-android-link", "href"); href != page {
		t.Errorf("the card's link leads to %q, want %q", href, page)
	}

	// Installing the page in the browser is no longer offered here, nor its
	// address as a code: the card is about the Android app.
	var left []string

	p.awaitJSON(`Promise.resolve(['#install-now', '#install-manual', '#install-phone', '#install-address']
		.filter((selector) => document.querySelector(selector)))`, &left)

	if len(left) != 0 {
		t.Errorf("the card still holds %v, which offered installing the page in the browser", left)
	}
}

// Inside the Android app the card is not there: it offers the app being used.
func TestTheInstallCardIsNotInTheAndroidApp(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.run("load the page as the app shows it", chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := cdppage.AddScriptToEvaluateOnNewDocument(`window.GoTimeRecordingFiles = {};`).Do(ctx)

		return err
	}), chromedp.Reload())
	p.settled()
	p.showView("settings")
	p.waitShown("#form-my-timezone")

	if p.visible("#install-card") {
		t.Error("inside the Android app the card offers the Android app")
	}
}
