//go:build browser

package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// linesOfWords is a script function counting the lines an element's words are
// laid out on: one per distinct line box, read off the words themselves. Not
// the element's height over its line height, which a padded button or a cell as
// tall as its row turns into a wrong answer.
const linesOfWords = `(element) => {
	const tops = [];
	const words = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);

	for (let node = words.nextNode(); node; node = words.nextNode()) {
		if (!node.textContent.trim()) continue;

		const range = document.createRange();
		range.selectNodeContents(node);

		for (const line of range.getClientRects()) {
			if (line.width > 0 && !tops.some((top) => Math.abs(top - line.top) < line.height / 2)) {
				tops.push(line.top);
			}
		}
	}

	return tops.length;
}`

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "…"
}

// waitForFilled waits until an element has something written in it.
//
// For the screens whose cards are in the markup and whose contents arrive by
// request: the card is there from the first paint, and only what is in it says
// whether the answer has come back.
func (p *page) waitForFilled(selector string) {
	p.t.Helper()

	deadline := time.Now().Add(waitPatience)

	for time.Now().Before(deadline) {
		if strings.TrimSpace(p.text(selector)) != "" {
			return
		}

		time.Sleep(250 * time.Millisecond)
	}

	p.t.Fatalf("%s was never written to, so the screen stopped loading before it;"+
		" the log says:\n%s", selector, p.app.Log())
}

// waitForValue waits until a field has something in it.
//
// For the screens whose forms are in the markup and whose contents arrive by
// request: the field is there from the first paint, and only what is in it says
// whether the answer has come back.
func (p *page) waitForValue(selector string) {
	p.t.Helper()

	deadline := time.Now().Add(waitPatience)

	for time.Now().Before(deadline) {
		if p.value(selector) != "" {
			return
		}

		time.Sleep(250 * time.Millisecond)
	}

	p.t.Fatalf("%s was never filled in, so the screen stopped loading before it;"+
		" the log says:\n%s", selector, p.app.Log())
}

// waitForNode waits until something matches.
func (p *page) waitForNode(selector string) {
	p.t.Helper()

	deadline := time.Now().Add(waitPatience)

	for time.Now().Before(deadline) {
		if p.count(selector) > 0 {
			return
		}

		time.Sleep(200 * time.Millisecond)
	}

	p.t.Fatalf("nothing ever matched %s", selector)
}

// submitAndAwaitReload sends the appearance form and waits for the page to come
// back.
//
// A changed mark reloads: no engine takes a new tab icon from a link swapped in
// afterwards, and a differently cropped logo is a different icon. The reload
// takes the saved notice away with it, so waiting for that notice is waiting for
// something that is being removed - which is a wait that sometimes sees it and
// sometimes does not, depending on how quickly the request came back.
//
// The reload itself is what is waited for, by a mark left on the document that
// is about to be replaced.
func (p *page) submitAndAwaitReload(t *testing.T, submit chromedp.Action) {
	t.Helper()

	p.run("mark this document", chromedp.Evaluate(`window.__beforeReload = true`, nil))
	p.run("save", submit)

	deadline := time.Now().Add(waitPatience)

	for {
		var gone bool

		// An evaluation that lands while the old document is being replaced fails
		// with the very navigation it is waiting for, so a failure here means "not
		// yet" - the deadline below still ends a reload that never comes.
		err := chromedp.Run(p.ctx, chromedp.Evaluate(`window.__beforeReload === undefined`, &gone))
		if err == nil && gone {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("the save never reloaded the page; the notice says %q",
				p.text("#toast"))
		}

		time.Sleep(250 * time.Millisecond)
	}

	p.run("wait for the screen", chromedp.WaitVisible("#form-branding", chromedp.ByID))
}
