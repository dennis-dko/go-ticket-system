//go:build browser

package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// A refusal from the server reaches the reader in their own language.
//
// The messages are written where the rule is enforced, in English, which is right
// for the log and wrong for the person who tripped over it: an English sentence was
// shown to a German reader whatever they had chosen.
// The reason now travels as a code with the values the sentence interpolated, and
// the interface looks the sentence up.
//
// Proved in a browser because that is the only place the lookup happens - on the
// wire the message is still English, deliberately, and the integration tests check
// exactly that.
func TestAServerRefusalIsShownInTheReadersLanguage(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()
	p.chooseLanguage("de")

	// The same account twice: the second is refused with the address it names,
	// which is the value the sentence would lose if the values were dropped.
	addWilma := func(what string) {
		p.run(what, p.click(`.tab[data-view="users"]`),
			chromedp.WaitVisible("#form-user", chromedp.ByID),
			chromedp.SendKeys(`#form-user input[name="name"]`, "Wilma", chromedp.ByQuery),
			chromedp.SendKeys(`#form-user input[name="email"]`, "wilma@example.com", chromedp.ByQuery),
			p.chooseOption(`#form-user select[name="role"]`, "agent"),
			chromedp.SendKeys(`#form-user input[name="password"]`, "wilma-password-1", chromedp.ByQuery),
			p.click(`#form-user button[type="submit"]`))
	}

	addWilma("add Wilma")
	p.waitForText("#table-users tbody", "wilma@example.com")

	p.run("clear the notices", chromedp.Evaluate(
		`document.querySelector('#toast').replaceChildren()`, nil))

	addWilma("add Wilma again")

	shown := ""

	deadline := time.Now().Add(waitPatience)
	for time.Now().Before(deadline) {
		if shown = p.text("#toast"); strings.Contains(shown, "wilma@example.com") {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	if shown == "" {
		t.Fatalf("the refusal raised no notice at all\n\napplication log:\n%s", p.app.Log())
	}

	// The German sentence, not the English one it was built from.
	if !strings.Contains(shown, "Es gibt bereits") {
		t.Errorf("the notice is not the German sentence: %q", shown)
	}

	if strings.Contains(shown, "already exists") {
		t.Errorf("the notice is still the server's English wording: %q", shown)
	}

	// And the value survived the translation.
	if !strings.Contains(shown, "wilma@example.com") {
		t.Errorf("the notice lost the address it is about: %q", shown)
	}
}

// A field the server rejects is named the way the form names it.
func TestARejectedFieldIsNamedNotIdentified(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.chooseLanguage("de")

	// A negative idle time, which is refused by field rather than by sentence.
	p.run("save an impossible idle time",
		chromedp.Click(`.tab[data-view="admin"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#form-operational input[name="sessionIdleMinutes"]`, chromedp.ByQuery),
		chromedp.Evaluate(`(() => {
			const field = document.querySelector('#form-operational input[name="sessionIdleMinutes"]');
			field.value = '-3';
			field.removeAttribute('min');

			// Announced the way typing announces itself, which is what tells the
			// page somebody is part way through this form. Without it a loader
			// answering between here and the press below puts the stored value
			// back, the save is a valid one, and the case waits for a refusal
			// that was never going to come - which is what it reported, as a
			// refusal that named no field.
			field.dispatchEvent(new Event('input', { bubbles: true }));
		})()`, nil),
		p.click(`#form-operational button[type="submit"]`),
	)

	shown := ""

	deadline := time.Now().Add(waitPatience)
	for time.Now().Before(deadline) {
		if shown = p.text("#toast"); strings.Contains(shown, "Untätigkeit") {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	if !strings.Contains(shown, "Untätigkeit") {
		t.Errorf("the refusal does not name the field as the form does: %q\n\n"+
			"application log:\n%s", shown, p.app.Log())
	}

	if strings.Contains(shown, "sessionIdleMinutes") {
		t.Errorf("the refusal still shows the column name: %q", shown)
	}

	if strings.Contains(shown, "invalid parameter") {
		t.Errorf("the refusal still carries GoFr's parameter count: %q", shown)
	}
}
