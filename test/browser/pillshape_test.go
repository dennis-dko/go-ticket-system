//go:build browser

package browser

import (
	"fmt"
	"strconv"
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

	p.everyScreen("administrator", measure)

	p.becomeWorker()
	p.everyScreen("worker", measure)

	if len(found) > 0 {
		t.Errorf("these buttons are stretched past their own height by what stands beside them:\n\t%s",
			strings.Join(found, "\n\t"))
	}
}

// A link in a row stands level with the controls beside it.
//
// A row's pills stand at its foot, so that one beside a label whose words stand
// above its field is not stretched to the height of both. A link is a button
// too, but no taller than its words, and put at the foot as well its words sank
// below those of the pills beside it: "stopped earlier?" stood under "Discard"
// for as long as a clock ran. So every screen is walked again, with the
// stopwatch running for whoever works here because that is when that link is
// shown, and a link whose middle is not the middle of a control on its line is
// reported.
func TestALinkInARowStandsLevelWithTheControlsBesideIt(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	var found []string

	measure := func(where string) {
		var askew []string

		p.evalJSON(`JSON.stringify([...document.querySelectorAll('.row > button.link')]
			.filter((link) => link.offsetParent !== null)
			.flatMap((link) => {
				const middle = (box) => (box.top + box.bottom) / 2;
				const own = link.getBoundingClientRect();

				// The pills of the row, and the fields its labels hold.
				const controls = [...link.parentElement.children].flatMap((el) =>
					el.matches('button:not(.link, .link-button)') ? [el]
						: el.matches('label')
							? [...el.querySelectorAll(':scope > select, :scope > input:not([type="checkbox"], [type="radio"], [type="file"])')]
							: []);

				return controls
					.filter((control) => control.offsetParent !== null)
					.map((control) => control.getBoundingClientRect())
					.filter((box) => box.top < own.bottom && own.top < box.bottom)
					.map((box) => middle(own) - middle(box))
					.filter((off) => Math.abs(off) > 2)
					.slice(0, 1)
					.map((off) => (link.id || link.textContent.trim().slice(0, 30)) + ': ' +
						Math.abs(Math.round(off)) + 'px ' + (off > 0 ? 'below' : 'above') + ' the control beside it');
			}))`, &askew)

		for _, s := range askew {
			found = append(found, where+": "+s)
		}
	}

	p.everyScreen("administrator", measure)

	p.becomeWorker()
	p.run("start the stopwatch", p.click(`.tab[data-view="timesheets"]`),
		chromedp.WaitVisible("#timer-start", chromedp.ByID),
		p.click("#timer-start"),
		chromedp.WaitVisible("#timer-end-open", chromedp.ByID))
	p.everyScreen("worker with the clock running", measure)

	if len(found) > 0 {
		t.Errorf("these links do not stand level with the controls beside them:\n\t%s",
			strings.Join(found, "\n\t"))
	}
}

// everyScreen opens each screen the signed-in account is offered, and each tab
// of it, and hands look the name of where it stands.
func (p *page) everyScreen(who string, look func(where string)) {
	p.t.Helper()

	var views []string

	p.run("list "+who+"'s screens", chromedp.Evaluate(
		`[...document.querySelectorAll('.tab[data-view]')].filter(tab => !tab.hidden).map(tab => tab.dataset.view)`,
		&views))

	if len(views) < 2 {
		p.t.Fatalf("too few screens to walk for %s: %v", who, views)
	}

	for _, view := range views {
		p.run("open "+view, p.click(fmt.Sprintf(`.tab[data-view=%q]`, view)))
		look(who + " " + view)

		var tabs []string

		p.run("list the tabs of "+view, chromedp.Evaluate(fmt.Sprintf(
			`[...document.querySelectorAll('#view-%s .screen-tab')].filter(tab => !tab.hidden).map(tab => tab.id)`,
			view), &tabs))

		for _, tab := range tabs {
			p.run("open "+tab, p.click("#"+tab))
			look(who + " " + view + " " + tab)
		}
	}
}

// The tick on the sign-in button follows the button's rounded end.
//
// It was a square on a button whose ends were square too. Once the buttons
// became pills it stood in a round end as a box glued on, its corners reaching
// for the curve, and read as a tick box somebody could press. A round tick,
// as far from the edge on every side, is part of the button's shape: so it is
// asked to be round, and to keep the same distance from the button's outline at
// its left as above and below it.
func TestTheSignInTickFollowsTheButtonsRoundEnd(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.run("wait for the sign-in form", chromedp.WaitVisible("#form-login .login-submit", chromedp.ByQuery))

	var shape struct {
		Round bool    `json:"round"`
		Left  float64 `json:"left"`
		Top   float64 `json:"top"`
		Below float64 `json:"below"`
	}

	p.evalJSON(`JSON.stringify((() => {
		const button = document.querySelector('#form-login .login-submit');
		const tick = button.querySelector('.login-tick');
		const b = button.getBoundingClientRect();
		const k = tick.getBoundingClientRect();
		const corner = parseFloat(getComputedStyle(tick).borderTopLeftRadius);

		return {
			round: Math.abs(k.width - k.height) < 1 && corner >= k.width / 2 - 0.5,
			left: k.left - b.left,
			top: k.top - b.top,
			below: b.bottom - k.bottom,
		};
	})())`, &shape)

	if !shape.Round {
		t.Error("the tick on the sign-in button is not round, so it stands in the button's round end as a box")
	}

	if d := shape.Left - shape.Top; d > 1 || d < -1 || shape.Top-shape.Below > 1 || shape.Below-shape.Top > 1 {
		t.Errorf("the tick is %.1fpx from the button's left, %.1fpx from its top and %.1fpx from its bottom; "+
			"a tick that follows the round end is as far from it on every side", shape.Left, shape.Top, shape.Below)
	}
}

// A week's cell shows the hours in it whole.
//
// The cells are fields, and a field's room on either side of its words is a
// pill's: twenty pixels a side left a cell of five and a half ems too little for
// "2.75", whose last figure was cut by the cell's edge. Asked of the widest
// figure a day is likely to hold.
func TestAWeeksCellShowsItsHoursWhole(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	var projectID int

	p.awaitJSON(`(async () => {
		const made = await api('/projects', {
			method: 'POST', body: JSON.stringify({ name: 'Portal', startDate: '2026-01-01' }),
		});
		await loadProjects();
		return made.id;
	})()`, &projectID)

	p.run("open the week", p.click(`.tab[data-view="week"]`),
		chromedp.WaitVisible("#table-week", chromedp.ByID))

	// A row to have cells in, added as the select adds one.
	p.waitForNode(`#week-add-project option[value="` + strconv.Itoa(projectID) + `"]`)
	p.run("add a row for the project", chromedp.Evaluate(`(() => {
		const select = document.querySelector('#week-add-project');
		select.value = '`+strconv.Itoa(projectID)+`';
		select.dispatchEvent(new Event('change', { bubbles: true }));
	})()`, nil))
	p.waitForNode(`#week-card input.week-cell`)

	var cell struct {
		Room    float64 `json:"room"`
		Needed  float64 `json:"needed"`
		Spinner bool    `json:"spinner"`
	}

	// The room is the cell's width less its borders and padding; the arrows of a
	// number field are drawn inside that room, so a cell that keeps them is asked
	// about separately rather than measured, which no script can do.
	p.evalJSON(`JSON.stringify((() => {
		const cell = document.querySelector('#week-card input.week-cell');
		const style = getComputedStyle(cell);
		const box = cell.getBoundingClientRect();
		const words = document.createElement('canvas').getContext('2d');
		words.font = style.font;

		return {
			room: box.width - parseFloat(style.borderLeftWidth) - parseFloat(style.borderRightWidth) -
				parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
			needed: words.measureText('12.75').width,
			spinner: !['textfield', 'none'].includes(style.appearance),
		};
	})())`, &cell)

	if cell.Spinner {
		t.Error("a week's cell keeps the browser's arrows for stepping a number, which take the room of the " +
			"figures beside them")
	}

	if cell.Room+0.5 < cell.Needed {
		t.Errorf("a week's cell cuts 12.75: room for %.1fpx of the %.1f it needs", cell.Room, cell.Needed)
	}
}
