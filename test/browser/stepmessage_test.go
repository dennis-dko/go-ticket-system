//go:build browser

package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// A step the browser refuses is said in the screen's words, as every other
// refusal is, and the steps that are left are the server's: whole minutes for the
// idle time, while a session lifetime of 7.75 hours is taken as the server takes
// it.
func TestAStepIsTheServersAndSaidInTheScreensWords(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()
	p.showView("admin")
	p.waitShown("#form-operational")

	p.run("ask for a sign-out after two and a half minutes",
		chromedp.SetValue(`#form-operational input[name="sessionIdleMinutes"]`, "2.5", chromedp.ByQuery),
		p.click(`#form-operational button[type="submit"]`))
	p.waitEvaluates("what the field says",
		`document.querySelector('#form-operational input[name="sessionIdleMinutes"]').validationMessage`,
		"Please enter a whole number.")

	var taken bool

	p.run("type a session lifetime of 7.75 hours", chromedp.Evaluate(`(() => {
		const field = document.querySelector('#form-operational input[name="sessionLifetimeHours"]');
		field.value = '7.75';
		return field.validity.valid;
	})()`, &taken))

	if !taken {
		t.Error("the form refuses a session lifetime of 7.75 hours, which the server takes")
	}
}
