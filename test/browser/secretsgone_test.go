//go:build browser

package browser

import (
	"encoding/json"
	"testing"

	"github.com/chromedp/chromedp"
)

// Nothing marked as a secret is still on screen when the session ends.
//
// The markup names them: four elements carry class="secret" - the enrolment's
// shared key, the otpauth URI that carries it with the account's address, a
// token's value, and a calendar feed's address, which is its key. The first three
// were found separately, and each was left behind by a sign-out for its own
// reason.
//
// This checks the class rather than the ids, so the fourth was covered by being
// marked rather than by somebody remembering this case exists. That is the whole
// reason the class is worth having: it is the markup saying which text on the
// page must not be handed on. A QR code carries its secret in its picture rather
// than as text, so every img.qr is asked for its address the same way - the
// enrolment's, the one a new token is shown with for an app to scan, and the
// calendar address's.
//
// In two rounds, because the enrolment and the other two are on different tabs of
// My account, and changing tabs takes a secret off the screen as leaving the
// screen does: a token and a calendar address, signed out; then an enrolment,
// signed out. Between them every marked element has to have held something, or
// the case is not covering what it means to.
func TestNothingMarkedSecretSurvivesTheSignOut(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	filled := func(when string) map[string]string {
		t.Helper()

		var out string

		p.run("read every secret "+when, chromedp.Evaluate(`JSON.stringify(Object.fromEntries([
			...[...document.querySelectorAll('.secret')]
				.map((node) => [node.id || node.className, node.textContent.trim()]),
			...[...document.querySelectorAll('img.qr')]
				.map((node) => [node.id, node.getAttribute('src') ?? '']),
		]))`, &out))

		var read map[string]string

		if err := json.Unmarshal([]byte(out), &read); err != nil {
			t.Fatalf("reading the secrets (%q): %v", out, err)
		}

		return read
	}

	held := map[string]bool{}

	// signOutAndLook notes what was held, signs out and reports whatever is still held.
	signOutAndLook := func(round string) map[string]string {
		t.Helper()

		before := filled("while signed in " + round)

		for id, value := range before {
			if value != "" {
				held[id] = true
			}
		}

		p.run("sign out "+round, p.click("#logout"),
			chromedp.WaitVisible("#form-login", chromedp.ByID))

		for id, value := range filled("after signing out " + round) {
			if value != "" {
				t.Errorf("#%s is marked as a secret and still holds %q after the session "+
					"ended %s", id, value, round)
			}
		}

		return before
	}

	p.run("make a token and a calendar address",
		p.click(`.tab[data-view="settings"]`),
		p.click(`#settings-tabs [data-card-tab="apps"]`),
		chromedp.SendKeys(`#form-token input[name="name"]`, "Reader", chromedp.ByQuery),
		p.click(`#form-token button[type="submit"]`),
		chromedp.WaitVisible("#token-secret", chromedp.ByID),
		p.click("#feed-create"),
		chromedp.WaitVisible("#feed-address", chromedp.ByID))

	marked := signOutAndLook("with a token and a calendar address")

	if len(marked) == 0 {
		t.Fatal("nothing on the page is marked as a secret, so this case checks nothing")
	}

	p.signIn(workerEmail, workerPassword)
	p.waitGone("#login-screen")

	p.run("start an enrolment",
		p.click(`.tab[data-view="settings"]`),
		p.click(`#settings-tabs [data-card-tab="signin"]`),
		chromedp.WaitVisible("#totp-card", chromedp.ByID),
		p.click("#totp-begin"),
		chromedp.WaitVisible("#totp-setup", chromedp.ByID))

	signOutAndLook("in the middle of an enrolment")

	if len(held) < len(marked) || len(held) < 8 {
		t.Fatalf("only %d of %d marked elements held anything in either round, so this case "+
			"is not covering what it means to: %v", len(held), len(marked), held)
	}
}
