//go:build browser

package browser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// No button is stretched taller than a control by what stands beside it.
//
// A button that acts is a pill as high as a field, and a row lays out its
// children as flex does by default, stretching each to the tallest of them. A
// button beside a label whose words stand above its field came out as tall as
// the two together: "Copy last week" beside "Add a row for", seventy-two pixels
// of oval where forty-eight were meant, which the maintainer saw at once. So
// every screen is walked, every tab of it, as the administrator and as somebody
// who works here, and a button whose words take one line but which is taller
// than its own minimum is reported.
func TestNoButtonIsStretchedPastAControlsHeight(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	var found []string

	measure := func(where string) {
		var stretched []string

		p.evalJSON(`JSON.stringify([...document.querySelectorAll('button')]
			.filter((button) => button.offsetParent !== null)
			.filter((button) => {
				const least = parseFloat(getComputedStyle(button).minHeight);
				if (!(least > 0)) return false;

				// Words on more than one line make a button taller as they should.
				const words = document.createRange();
				words.selectNodeContents(button);
				const lines = new Set([...words.getClientRects()].map((r) => Math.round(r.top))).size;

				return lines <= 1 && button.getBoundingClientRect().height > least + 2;
			})
			.map((button) => (button.id || button.textContent.trim().slice(0, 30)) + ': ' +
				Math.round(button.getBoundingClientRect().height) + 'px'))`, &stretched)

		for _, s := range stretched {
			found = append(found, where+": "+s)
		}
	}

	walk := func(who string) {
		var views []string

		p.run("list "+who+"'s screens", chromedp.Evaluate(
			`[...document.querySelectorAll('.tab[data-view]')].filter(tab => !tab.hidden).map(tab => tab.dataset.view)`,
			&views))

		if len(views) < 2 {
			t.Fatalf("too few screens to walk for %s: %v", who, views)
		}

		for _, view := range views {
			p.run("open "+view, p.click(fmt.Sprintf(`.tab[data-view=%q]`, view)))
			measure(who + " " + view)

			var tabs []string

			p.run("list the tabs of "+view, chromedp.Evaluate(fmt.Sprintf(
				`[...document.querySelectorAll('#view-%s .screen-tab')].filter(tab => !tab.hidden).map(tab => tab.id)`,
				view), &tabs))

			for _, tab := range tabs {
				p.run("open "+tab, p.click("#"+tab))
				measure(who + " " + view + " " + tab)
			}
		}
	}

	walk("administrator")

	p.becomeWorker()
	walk("worker")

	if len(found) > 0 {
		t.Errorf("these buttons are stretched past their own height by what stands beside them:\n\t%s",
			strings.Join(found, "\n\t"))
	}
}
