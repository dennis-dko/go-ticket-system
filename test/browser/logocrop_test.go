//go:build browser

package browser

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// The bar can be given a part of the logo rather than all of it.
//
// A wordmark with a symbol at one end can read better as the symbol alone, and
// nobody can guess which part is worth showing, so it is chosen. The selection
// opens on the shape of the room the bar gives a logo, which is the one selection
// nothing has to be done to.
func TestAPartOfTheLogoCanBeChosenForTheBar(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open Settings", p.click(`.tab[data-view="admin"]`),
		p.click(`#admin-tabs [data-card-tab="general"]`),
		chromedp.WaitVisible("#form-branding", chromedp.ByID))

	p.storeBranding(t, wideLogo)

	p.reload()
	p.run("open Settings again", p.click(`.tab[data-view="admin"]`),
		p.click(`#admin-tabs [data-card-tab="general"]`),
		chromedp.WaitVisible("#form-branding", chromedp.ByID))

	// The preview is a control, and opens the chooser. One, because the bar is
	// the one place the logo is drawn.
	if p.count(".logo-use-button") != 1 {
		t.Fatalf("%d previews can be pressed, want the bar's alone",
			p.count(".logo-use-button"))
	}

	p.run("choose the part used in the bar",
		p.click(`.logo-use-button[data-crop="banner"]`),
		chromedp.WaitVisible("#crop-overlay", chromedp.ByID))

	// The bar's shape, 656 by 192. The box is the application's answer to "as
	// much of this as that shape can hold", before anything is dragged.
	var box struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}

	p.evalJSON(`JSON.stringify((() => {
		const r = document.querySelector('#crop-box').getBoundingClientRect();
		return { width: r.width, height: r.height };
	})())`, &box)

	if box.Width < 8 || box.Height < 8 {
		t.Fatalf("the selection opened at %.0fx%.0f, which cannot be aimed at",
			box.Width, box.Height)
	}

	if ratio := box.Width / box.Height; ratio < 3.1 || ratio > 3.75 {
		t.Errorf("the bar's selection is %.0fx%.0f, a ratio of %.2f - the bar's room "+
			"is 656 by 192", box.Width, box.Height, ratio)
	}

	// Moved to the left end of the logo, which is where a wordmark keeps its
	// mark, and stored.
	p.run("take the left end", chromedp.Evaluate(
		`(() => {
			cropBox = { ...cropBox, x: 0, y: 0 };
			drawCropBox();
			document.querySelector('#crop-apply').click();

			return 1;
		})()`, nil))

	// What the page believes it chose, before saving - so a failure says which of
	// the two halves broke rather than only that the answer is empty.
	var chosen string

	p.run("read the chosen part", chromedp.Evaluate(
		`JSON.stringify(logoCrops)`, &chosen))

	if !strings.Contains(chosen, "banner") {
		t.Fatalf("the chooser stored nothing for the bar: %s", chosen)
	}

	// What the request actually carried, recorded as it goes: the page is where
	// this was failing, and the body it sends is the one thing neither side's log
	// shows.
	p.run("watch the save", chromedp.Evaluate(`
		(() => {
			window.__sent = 'nothing was sent';
			const real = window.fetch;

			window.fetch = (url, options) => {
				if (String(url).includes('/settings/branding') && options?.method === 'PUT') {
					window.__sent = options?.body ?? '';
				}

				return real(url, options);
			};

			return 1;
		})()`, nil))

	p.submitAndAwaitSave(t, p.click(`#form-branding button[type="submit"]`))

	var sent string

	p.run("read what was sent", chromedp.Evaluate(`String(window.__sent)`, &sent))

	if !strings.Contains(sent, "crops") {
		t.Fatalf("the save carried no crops: %.300s", sent)
	}

	// What was chosen comes back, so the chooser reopens on it rather than
	// starting again - and the server has it, which is what the bar's copy is
	// made from.
	var stored struct {
		Banner struct {
			X float64 `json:"x"`
			W float64 `json:"w"`
		} `json:"banner"`
	}

	var raw string

	p.run("read what was stored", chromedp.Evaluate(`
		(async () => {
			const r = await fetch('/api/v1/branding', { credentials: 'same-origin' });
			const body = await r.json();

			return JSON.stringify(body?.data?.crops ?? {});
		})()`, &raw, awaitPromise))

	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("reading what was stored: %v; %s", err, raw)
	}

	if stored.Banner.W <= 0 {
		t.Fatal("nothing was stored for the bar's part of the logo")
	}

	if stored.Banner.W >= 1 {
		t.Errorf("the whole logo was stored (w=%.2f) rather than the part chosen",
			stored.Banner.W)
	}

	if stored.Banner.X > 0.05 {
		t.Errorf("the part stored starts at %.2f rather than at the left end",
			stored.Banner.X)
	}
}

// The selection is free: a corner pulled in one direction moves in that
// direction only, and whatever shape comes out of it is what gets stored.
//
// The opening selection has the shape of the place it is for, and that used to
// be a rule rather than a starting point - a corner dragged upwards took the
// width with it, so a place could only ever be given a part of its own shape.
// This is the test that the rule is gone, dragged with real pointer input so that
// a corner nothing is wired to fails here rather than passing.
func TestTheChosenPartCanBeAnyShape(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open Settings", p.click(`.tab[data-view="admin"]`),
		p.click(`#admin-tabs [data-card-tab="general"]`),
		chromedp.WaitVisible("#form-branding", chromedp.ByID))

	p.storeBranding(t, wideLogo)

	p.reload()
	p.run("open Settings again", p.click(`.tab[data-view="admin"]`),
		p.click(`#admin-tabs [data-card-tab="general"]`),
		chromedp.WaitVisible("#form-branding", chromedp.ByID))

	p.run("choose the part used in the bar",
		p.click(`.logo-use-button[data-crop="banner"]`),
		chromedp.WaitVisible("#crop-overlay", chromedp.ByID))

	before := p.selection(t)

	// The logo is five times as wide as it is tall, so the bar's shape opens on
	// the full height of it - which is what makes pulling the bottom edge up a
	// fair question to ask.
	if before.H < 0.9 {
		t.Fatalf("the bar's selection opened %.2f of the way down the logo, "+
			"so there is nothing to pull up", before.H)
	}

	p.drag(`.crop-handle-se`, 0, -60)

	after := p.selection(t)

	if after.H >= before.H-0.1 {
		t.Fatalf("the bottom edge was pulled up and the selection is still "+
			"%.2f tall, was %.2f", after.H, before.H)
	}

	// The point of the whole exercise. Under the old rule this width would have
	// followed the height down to keep the shape.
	if math.Abs(after.W-before.W) > 0.02 {
		t.Errorf("pulling the bottom edge up changed the width from %.2f to %.2f; "+
			"a corner should move the two edges it is dragged along and no others",
			before.W, after.W)
	}

	// Still on the logo. A selection that has been dragged off it describes a
	// part that does not exist, which the server would have to guess about.
	if after.X < 0 || after.Y < 0 || after.X+after.W > 1.001 || after.Y+after.H > 1.001 {
		t.Errorf("the selection left the logo: %+v", after)
	}

	p.run("use this part", p.click("#crop-apply"))
	p.submitAndAwaitSave(t, p.click(`#form-branding button[type="submit"]`))

	// And it survives being stored: the shape that was chosen is the shape that
	// comes back, rather than one the server rounded to something it preferred.
	var stored struct {
		Banner struct {
			W float64 `json:"w"`
			H float64 `json:"h"`
		} `json:"banner"`
	}

	var raw string

	p.run("read what was stored", chromedp.Evaluate(`
		(async () => {
			const r = await fetch('/api/v1/branding', { credentials: 'same-origin' });
			const body = await r.json();

			return JSON.stringify(body?.data?.crops ?? {});
		})()`, &raw, awaitPromise))

	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("reading what was stored: %v; %s", err, raw)
	}

	if math.Abs(stored.Banner.H-after.H) > 0.02 || math.Abs(stored.Banner.W-after.W) > 0.02 {
		t.Errorf("chose %.2fx%.2f and %.2fx%.2f came back",
			after.W, after.H, stored.Banner.W, stored.Banner.H)
	}
}

// selection is the part of the logo the chooser currently has marked, in
// fractions of the whole image.
func (p *page) selection(t *testing.T) struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
} {
	t.Helper()

	var box struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"w"`
		H float64 `json:"h"`
	}

	// Read off the screen rather than out of the page's own variable: what is
	// drawn is what is being aimed at, and the two agreeing is part of what is
	// under test.
	p.evalJSON(`JSON.stringify((() => {
		const image = document.querySelector('#crop-image');
		const area = drawnImageArea(image);
		const box = document.querySelector('#crop-box').getBoundingClientRect();
		const stage = document.querySelector('#crop-stage').getBoundingClientRect();

		return {
			x: (box.left - stage.left - area.left) / area.width,
			y: (box.top - stage.top - area.top) / area.height,
			w: box.width / area.width,
			h: box.height / area.height,
		};
	})())`, &box)

	return box
}
