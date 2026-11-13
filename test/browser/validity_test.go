//go:build browser

package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// A field the browser refuses is refused in the screen's language.
//
// The forms state their rules in the markup - a required field, a number between
// two bounds, a label of at most so many characters - and the browser holds a form
// to them before anything is sent, in its own words: on a German screen in an
// English browser, "Value must be less than or equal to 3650." and "Please fill
// out this field." The words are the application's now, in the screen's language;
// decided on 2026-10-17.
//
// A refusal of its own makes a field invalid until it is taken back, so a value
// the script puts in afterwards - which raises no input event - must not meet the
// old refusal.
func TestABrowserRefusalSpeaksTheScreensLanguage(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()
	p.chooseLanguage("de")

	p.showView("settings")
	p.waitShown("#form-token")

	const (
		label = `#form-token input[name="name"]`
		days  = `#form-token input[name="expiresInDays"]`
	)

	p.run("label the token", chromedp.SetValue(label, "a token", chromedp.ByQuery))

	for _, attempt := range []struct{ value, want string }{
		{"4000", "Höchstens 3650."},
		{"-1", "Mindestens 0."},
	} {
		var said string

		p.run("ask for "+attempt.value+" days",
			chromedp.SetValue(days, attempt.value, chromedp.ByQuery),
			p.click(`#form-token button[type="submit"]`),
			chromedp.Evaluate(`document.querySelector('`+days+`').validationMessage`, &said))

		if said != attempt.want {
			t.Errorf("%q days is refused with %q, where the German screen would say %q",
				attempt.value, said, attempt.want)
		}
	}

	var said string

	// Emptied by hand rather than with chromedp's SetValue, which drops an empty
	// argument and so writes "undefined" into a text field - measured. A number
	// field sanitises that back to nothing, which is how the same call emptied the
	// hours this case used to ask about.
	p.run("leave the label out",
		chromedp.SetValue(days, "30", chromedp.ByQuery),
		chromedp.Evaluate(`(() => {
			const field = document.querySelector('`+label+`');
			field.value = '';
			field.dispatchEvent(new Event('input', { bubbles: true }));
		})()`, nil),
		p.click(`#form-token button[type="submit"]`),
		chromedp.Evaluate(`document.querySelector('`+label+`').validationMessage`, &said))

	if want := "Bitte fülle dieses Feld aus."; said != want {
		t.Errorf("a token without a label is refused with %q, where the German screen would say %q",
			said, want)
	}

	// Assigned the way the script fills a form, which raises no event at all -
	// chromedp's SetValue raises input and change, and would take the old refusal
	// back by the ordinary road.
	p.run("put a good label in from the script and make the token",
		chromedp.Evaluate(`document.querySelector('`+label+`').value = 'filled-in-by-script'`, nil),
		p.click(`#form-token button[type="submit"]`))

	p.waitForText("#table-tokens tbody", "filled-in-by-script")
}
