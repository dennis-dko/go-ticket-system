//go:build browser

package browser

import (
	"context"
	"strings"
	"testing"

	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// A card withheld from somebody who can open its screen is replaced by a
// sentence, not by nothing. Asked from inside the Android app why the passkeys
// were missing from My account, there was no telling a card kept back from a
// feature removed: the app shows the page in a WebView, which offers no passkeys,
// and the card simply was not there. A browser can lack them as well. Each is
// told why in the card's place, and the tour's step for the card says the same.
func TestAPasskeyCardKeptBackSaysWhy(t *testing.T) {
	t.Parallel()

	for _, place := range []struct{ name, script, says string }{
		{"the Android app", `window.GoTimeRecordingFiles = {};`, "The app signs in with the token"},
		{"a browser without passkeys", `delete window.PublicKeyCredential;`, "This browser cannot use passkeys here"},
	} {
		t.Run(place.name, func(t *testing.T) {
			t.Parallel()

			p := open(t)
			p.readyWorker()

			p.run("load the page as "+place.name, chromedp.ActionFunc(func(ctx context.Context) error {
				_, err := cdppage.AddScriptToEvaluateOnNewDocument(place.script).Do(ctx)

				return err
			}), chromedp.Reload())
			p.settled()
			p.showView("settings")
			p.waitShown("#passkey-elsewhere")

			said := p.text("#passkey-elsewhere")
			if !strings.HasPrefix(said, place.says) {
				t.Errorf("in %s the passkey card says %q, want it to start %q", place.name, said, place.says)
			}

			if p.visible("#form-passkey") || p.visible("#table-passkeys") {
				t.Errorf("in %s the passkey card offers a passkey nothing here could make", place.name)
			}

			var tour string

			p.run("read the tour's step for the card", chromedp.Evaluate(
				`TOUR_STEPS.find((step) => step.target === '#passkey-card').text()`, &tour))

			if tour != said {
				t.Errorf("in %s the tour says %q where the card says %q", place.name, tour, said)
			}
		})
	}
}
