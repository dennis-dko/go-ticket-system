//go:build browser

package browser

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// A refusal for something that is not there names it the way the screen does.
//
// A correction or a deletion of an entry that is gone by the time it lands - a
// second tab, a colleague's import that replaced it - is answered "not found",
// with what was looked for as a value. The value is the server's own word for
// it, so a German screen read "timesheet mit der Kennung 42 wurde nicht
// gefunden" - and would read "role" where it means a Rolle.
func TestANotFoundRefusalNamesWhatWasLookedForInTheReadersWords(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()
	p.chooseLanguage("de")

	var said string

	p.run("correct a role that is not there", chromedp.Evaluate(`(async () => {
		const token = document.cookie.split('; ')
			.find((c) => c.startsWith('gts_csrf=')).split('=')[1];
		const res = await fetch('/api/v1/roles/999999', {
			method: 'PUT',
			credentials: 'same-origin',
			headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': token },
			body: JSON.stringify({ description: 'nowhere', permissions: ['tickets:read'] }),
		});
		const body = await res.json();
		return res.status + ' ' + describeRefusal(body.error);
	})()`, &said, awaitPromise))

	if !strings.HasPrefix(said, "404 ") {
		t.Fatalf("correcting a role that is not there answered %q, want a 404", said)
	}

	if strings.Contains(said, "role ") {
		t.Errorf("the refusal names the role by the server's word for it: %q", said)
	}

	if !strings.Contains(said, "Rolle") {
		t.Errorf("the refusal does not call the role what the screen calls it: %q", said)
	}
}
