//go:build browser

package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// A row of buttons under a table stands clear of the table's last line.
//
// It sat directly on it: the statement's Export as PDF and Export as .xlsx
// touched the line under its totals, which reads as the buttons being part of the
// table rather than of the card. The report and the overtime are built the same
// way - a table, then a row of buttons - so the space belongs to the stylesheet
// rather than to each card.
func TestAButtonRowStandsClearOfTheTableAboveIt(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	var gap float64

	p.run("measure a table and the row under it", chromedp.Evaluate(`
		(() => {
			const card = document.createElement('div');
			card.className = 'card';
			card.innerHTML = '<div class="table-wrap"><table><tbody><tr><td>1</td></tr></tbody>' +
				'<tfoot><tr><td>2</td></tr></tfoot></table></div>' +
				'<div class="row"><button type="button" class="secondary">Export</button></div>';
			Object.assign(card.style, { position: 'fixed', top: '0', left: '0', width: '600px', zIndex: '9999' });
			document.body.append(card);

			const table = card.querySelector('table').getBoundingClientRect();
			const row = card.querySelector('.row').getBoundingClientRect();
			card.remove();

			return row.top - table.bottom;
		})()`, &gap))

	if gap < 12 {
		t.Errorf("the buttons under a table start %.1fpx below its last line, want them clear of it", gap)
	}
}
