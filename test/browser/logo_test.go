//go:build browser

package browser

import (
	"math"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// The sign-in screen and the tab keep TimePulse's mark whatever logo is saved.
//
// The logo was drawn on the sign-in card and became the tab's icon, until the
// maintainer decided that both belong to the application: the logo stands at the
// left of the bar once somebody has signed in, and nowhere else. Whether the
// sign-in card really shows the mark is a question only a browser answers - the
// card is filled in by the script, from the branding it fetched.
func TestTheSignInScreenKeepsTimePulsesMarkWhateverTheLogo(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open Settings", p.click(`.tab[data-view="admin"]`),
		p.click(`#admin-tabs [data-card-tab="general"]`),
		chromedp.WaitVisible("#form-branding", chromedp.ByID))

	// Through the API rather than the file picker: what is being checked is what
	// a stored logo reaches, and driving a file input would be checking the picker.
	p.storeBranding(t, wideLogo)

	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))

	// Reloaded, because branding is read once at start-up: without this the page
	// still holds what it fetched before the logo existed, and the test would be
	// asserting the state of a screen no visitor ever sees.
	p.run("reload the sign-in screen", chromedp.Reload(),
		chromedp.WaitVisible("#form-login", chromedp.ByID))

	if !p.visible("#login-mark") {
		t.Error("the sign-in card does not show TimePulse's mark")
	}

	var shown struct {
		Pictures int    `json:"pictures"`
		Icon     string `json:"icon"`
	}

	p.evalJSON(`JSON.stringify({
		pictures: [...document.querySelectorAll('#login-screen img')]
			.filter((img) => (img.getAttribute('src') ?? '').startsWith('data:image/')).length,
		icon: document.querySelector('link[rel="icon"]')?.getAttribute('href') ?? '',
	})`, &shown)

	if shown.Pictures != 0 {
		t.Errorf("the sign-in screen draws %d uploaded pictures; it carries TimePulse's mark only",
			shown.Pictures)
	}

	if shown.Icon != "/favicon.svg" {
		t.Errorf("the tab is pointed at %q rather than at TimePulse's mark", shown.Icon)
	}
}

// storeBranding writes the instance's logo through the API, as the Settings form
// does. An empty string clears it.
func (p *page) storeBranding(t *testing.T, logo string) {
	t.Helper()

	var status string

	p.run("store the branding", chromedp.Evaluate(`
		(async () => {
			const csrf = document.cookie.split(';').map(c => c.trim())
				.find(c => c.startsWith('timepulse_csrf='))?.slice('timepulse_csrf='.length) ?? '';

			const r = await fetch('/api/v1/settings/branding', {
				method: 'PUT',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
				body: JSON.stringify({ title: 'Zeiterfassung', logo: '`+logo+`' }),
			});

			return String(r.status);
		})()`, &status, awaitPromise))

	if status != "200" {
		t.Fatalf("could not store the branding: HTTP %s", status)
	}
}

// The place a company's own mark goes stays empty until it puts one there.
//
// It fell back to the shipped mark for a while, on the reasoning that a header of
// words alone looks unfinished. It is the wrong place for it: filling the slot
// meant for whoever runs the installation makes an unbranded one look branded by
// somebody else. The application's own mark has its own places - the browser tab,
// the sign-in card and the button beside the title - which say which program this
// is without taking the space meant for the company.
func TestTheLogoSlotsStayEmptyUntilALogoIsConfigured(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	// Nothing configured: this instance has never been given a logo.
	if src := p.attr("#brand-logo", "src"); src != "" {
		t.Errorf("the header shows %.40q on an installation with no logo", src)
	}

	if p.visible("#brand-logo") {
		t.Error("the header shows a mark on an installation that has configured none")
	}

	// Something stands in the slot all the same: the installation's initial,
	// taken from the title it was given. The corner was empty until somebody
	// configured a logo, which is a fresh installation saying nothing about
	// itself.
	if !p.visible("#brand-mark") {
		t.Error("the logo slot is empty on an installation that has uploaded none, " +
			"so the corner says nothing about which installation this is")
	}

	if letter := p.text("#brand-initial"); letter == "" {
		t.Error("the placeholder mark carries no letter, so it is an empty chip")
	}

	// And the application's own mark is beside the title, where it belongs.
	//
	// The two used to share one drawing and show one of it, so a fresh
	// installation wore the application's mark in the logo slot and had a bare
	// title in the middle - the title losing its mark to a slot that is not
	// about the application at all. Both are filled now, and they say different
	// things: the left one is this installation, the middle one is this program.
	if !p.visible(".app-mark") {
		t.Error("the title has no mark beside it, so nothing in the middle of the " +
			"bar says which application this is")
	}

	// Drawn, not typed. The house character it replaced is a font glyph: a
	// different picture on every platform and a hollow box where the font has no
	// house in it.
	var marks int

	p.evalJSON(`JSON.stringify(document.querySelectorAll('.app-mark svg, svg.app-mark').length)`,
		&marks)

	if marks == 0 {
		t.Error("the mark beside the title is not drawn")
	}

	// And a configured logo fills the header.
	p.run("open Settings", p.click(`.tab[data-view="admin"]`),
		p.click(`#admin-tabs [data-card-tab="general"]`),
		chromedp.WaitVisible("#form-branding", chromedp.ByID))

	p.storeBranding(t, wideLogo)

	p.reload()

	if src := p.attr("#brand-logo", "src"); !strings.HasPrefix(src, "data:image/") {
		t.Errorf("the header shows %.40q after a logo was configured", src)
	}

	if !p.visible("#brand-logo") {
		t.Error("the header's mark is hidden after a logo was configured")
	}

	// The mark beside the title is untouched by any of this: it is the
	// application, and the application is there whatever the installation has
	// uploaded.
	if !p.visible(".app-mark") {
		t.Error("nothing on the bar says which application this is once the logo " +
			"slot belongs to the installation")
	}

	// The placeholder steps aside, though - that is what it is for.
	if p.visible("#brand-mark") {
		t.Error("the placeholder initial is still in the logo slot beside the logo " +
			"that replaced it")
	}

	// The logo at the left end and the name in the middle, which is the way round
	// they were swapped to. The account keeps the right.
	//
	// Measured on a window wide enough for the whole row, because that is where
	// "in the middle" is exact. On a narrower one the right-hand side needs more
	// than its half and the name gives way to the left rather than be drawn over
	// - by 18 pixels at the suite's own 1,280 in the faces the CI runner has,
	// which is how this case came to say so: it had measured there, and passed
	// for as long as the name was allowed to lie under the account's.
	//
	// 1,600 was wide enough until the name became "TimePulse | Zeiterfassung",
	// 317 pixels in those faces. The right-hand column needs 609 there, and 713
	// from 1,800 on, where the account's name is given more room, so the name sat
	// 3 pixels left of the middle at 1,600 and 7 at 1,800. 1,920 leaves room
	// on both sides of that step.
	p.run("a wide window", chromedp.EmulateViewport(1920, 1080))

	var placed struct {
		TitleLeft float64 `json:"titleLeft"`
		LogoLeft  float64 `json:"logoLeft"`
		Centre    float64 `json:"centre"`
		BarCentre float64 `json:"barCentre"`
	}

	p.evalJSON(`JSON.stringify((() => {
		const title = document.querySelector('#app-title').getBoundingClientRect();
		const logo = document.querySelector('#brand-logo').getBoundingClientRect();
		const bar = document.querySelector('.topbar').getBoundingClientRect();

		return {
			titleLeft: title.left,
			logoLeft: logo.left,
			// The name itself, not the column holding it.
			centre: title.left + title.width / 2,
			barCentre: bar.left + bar.width / 2,
		};
	})())`, &placed)

	if placed.TitleLeft < placed.LogoLeft {
		t.Errorf("the title is at %.0f and the logo at %.0f, so the words come "+
			"before the picture", placed.TitleLeft, placed.LogoLeft)
	}

	if math.Abs(placed.Centre-placed.BarCentre) > 2 {
		t.Errorf("the name sits at %.0f and the bar's middle is %.0f",
			placed.Centre, placed.BarCentre)
	}

	p.run("back to the suite's window", chromedp.EmulateReset())

	// The banner's copy, which is the one made for the room the bar gives a logo.
	// The header's own copy is smaller than that room, and drawn there it would
	// be enlarged and blurred.
	var drawn string

	p.run("read what the bar draws", chromedp.Evaluate(`
		(async () => {
			const r = await fetch('/api/v1/branding', { credentials: 'same-origin' });
			const branding = (await r.json())?.data ?? {};
			const src = document.querySelector('#brand-logo').getAttribute('src');

			return src === branding.logoBanner ? 'the banner' : src === branding.logoHeader ? 'the header' : 'neither';
		})()`, &drawn, awaitPromise))

	if drawn != "the banner" {
		t.Errorf("the bar draws %s's copy of the logo, want the banner's", drawn)
	}
}

// Saving the logo redraws the bar where it stands, without a reload.
//
// A save used to reload the page, because no engine takes a new tab icon from a
// link swapped in afterwards - and the reload landed at the top of a long screen,
// away from what had just been changed. The tab keeps TimePulse's mark now, so
// there is nothing left that only a new document can show.
func TestSavingTheLogoRedrawsTheBarWithoutAReload(t *testing.T) {
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

	before := p.attr("#brand-logo", "src")
	if !strings.HasPrefix(before, "data:image/") {
		t.Fatalf("the bar shows %.40q, so there is no logo to see redrawn", before)
	}

	// A different part of the logo for the bar, which is a different copy.
	p.run("choose a part", p.click(`.logo-use-button[data-crop="banner"]`),
		chromedp.WaitVisible("#crop-overlay", chromedp.ByID))
	p.drag(`.crop-handle-se`, -120, 0)
	p.run("use it", p.click("#crop-apply"))

	p.submitAndAwaitSave(t, p.click(`#form-branding button[type="submit"]`))

	p.waitFor("the bar to draw the part just chosen", func() bool {
		return p.attr("#brand-logo", "src") != before
	})
}
