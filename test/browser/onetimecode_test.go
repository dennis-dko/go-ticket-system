//go:build browser

package browser

import (
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/go-time-recording/internal/support/security"
)

// What a second factor's code field says when the browser refuses what was typed.
const sixDigits = "Please enter the six digits your authenticator app shows."

// A code for the second factor is held to its rule in the browser before it is
// sent: six digits. The card that switches the second factor off had no form at
// all, so five digits went to the server and came back as a wrong code - the case
// the maintainer reported. A code that keeps to the rule still switches it off.
func TestTheSecondFactorsCodeIsCheckedBeforeItIsSent(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	secret := p.enableTOTP(t)

	p.reload()
	p.settled()
	p.showView("settings")
	p.waitShown("#totp-disable")

	// Every request for the second factor counted, from here on.
	p.awaitJSON(`(() => {
		window.sentForTheFactor = 0;
		const sent = window.fetch;
		window.fetch = (resource, options) => {
			if (String(resource).includes('/me/totp')) window.sentForTheFactor += 1;
			return sent(resource, options);
		};
		return Promise.resolve(true);
	})()`, nil)

	p.run("press Disable with no code", p.click("#totp-disable"))
	p.waitEvaluates("what the empty field says", `document.querySelector('#totp-code').validationMessage`,
		"Please fill in this field.")

	p.run("type five digits and press Disable",
		chromedp.SetValue("#totp-code", "12345", chromedp.ByID), p.click("#totp-disable"))
	p.waitEvaluates("what the field says", `document.querySelector('#totp-code').validationMessage`, sixDigits)

	var sent int

	p.run("count what was sent", chromedp.Evaluate(`window.sentForTheFactor`, &sent))

	if sent != 0 {
		t.Errorf("a five-digit code was sent to the server %d time(s), where the browser could have refused it", sent)
	}

	// A code that keeps to the rule goes through and switches the factor off.
	code, err := security.TOTPCodeAt(secret, time.Now().Add(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	p.run("type the code and press Disable",
		chromedp.SetValue("#totp-code", code, chromedp.ByID), p.click("#totp-disable"))
	p.waitShown("#totp-begin")
}

// The sign-in's field for the second factor holds a code to the same rule.
func TestTheSignInsCodeIsCheckedBeforeItIsSent(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	p.enableTOTP(t)

	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))
	p.signIn(workerEmail, workerPassword)
	p.waitShown("#login-totp-field")

	p.run("type a code with a letter in it and sign in",
		chromedp.SetValue(`#login-totp-field input`, "12a456", chromedp.ByQuery),
		p.click(`#form-login button[type="submit"]`))
	p.waitEvaluates("what the field says", `document.querySelector('#login-totp-field input').validationMessage`, sixDigits)
}
