//go:build browser

package browser

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/chromedp/chromedp"
)

// A field, a form's own button and the quieter button beside it end on the same
// line, in both themes.
//
// The quieter one was a box inside a border, a pixel taller than the form's own
// button above and below, so the two never lined up in a row - and beside a
// field it read as one more field. It is a tint of the text now, with no border,
// and the field's border is paid for out of its padding, so all three are the
// same height.
func TestAFieldAndBothKindsOfButtonAreEquallyTall(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	for _, theme := range []string{"light", "dark"} {
		var measured string

		p.run("measure the three in the "+theme+" theme", chromedp.Evaluate(`
			(() => {
				document.documentElement.dataset.theme = '`+theme+`';

				// On the page itself, where the rules that draw every field and button
				// apply and nothing a screen adds does - and in a row that does not
				// stretch them, which would make any three the same height.
				const row = document.createElement('div');
				row.innerHTML = '<input type="text"><button type="button">Save</button>' +
					'<button type="button" class="secondary">Cancel</button>';
				Object.assign(row.style, {
					position: 'fixed', top: '0', left: '0', zIndex: '9999',
					display: 'flex', alignItems: 'flex-start', width: '600px',
				});
				document.body.append(row);

				const heights = [...row.children].map((control) => control.getBoundingClientRect().height);
				row.remove();

				return JSON.stringify(heights);
			})()`, &measured))

		var heights []float64
		if err := json.Unmarshal([]byte(measured), &heights); err != nil || len(heights) != 3 {
			t.Fatalf("measuring the controls in the %s theme answered %q", theme, measured)
		}

		field, primary, secondary := heights[0], heights[1], heights[2]

		if field == 0 {
			t.Fatalf("the controls measured nothing in the %s theme, so this compares nothing", theme)
		}

		if math.Abs(primary-field) > 0.5 || math.Abs(secondary-field) > 0.5 {
			t.Errorf("in the %s theme a field is %.1fpx tall, the form's own button %.1fpx and the quieter one %.1fpx",
				theme, field, primary, secondary)
		}
	}
}
