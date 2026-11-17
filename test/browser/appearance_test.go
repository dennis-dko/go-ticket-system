//go:build browser

package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// Signing out takes the appearance with it.
//
// Appearance is chosen per device, which is right while somebody is using it
// and wrong the moment they leave: the next person at that machine arrived to
// the last one's dark mode, on a screen with nothing else of theirs on it.
func TestSigningOutForgetsTheAppearance(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("choose dark", p.chooseOption("#theme-picker", "dark"))

	var chosen struct {
		Preference string `json:"preference"`
		Stored     string `json:"stored"`
	}

	p.evalJSON(`JSON.stringify({
		preference: document.documentElement.dataset.themePreference,
		stored: localStorage.getItem('timepulse.theme') ?? '',
	})`, &chosen)

	if chosen.Preference != "dark" || chosen.Stored != "dark" {
		t.Fatalf("choosing dark left the page at %+v, so this proves nothing", chosen)
	}

	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))

	var after struct {
		Preference string `json:"preference"`
		Stored     string `json:"stored"`
		Picker     string `json:"picker"`
	}

	p.evalJSON(`JSON.stringify({
		preference: document.documentElement.dataset.themePreference,
		stored: localStorage.getItem('timepulse.theme') ?? '',
		picker: document.querySelector('#theme-picker').value,
	})`, &after)

	if after.Stored != "" {
		t.Errorf("the appearance %q is still kept on this device after signing out",
			after.Stored)
	}

	if after.Preference != "auto" {
		t.Errorf("the page follows %q rather than the time of day after signing out",
			after.Preference)
	}

	if after.Picker != "auto" {
		t.Errorf("the picker still offers %q as the current choice", after.Picker)
	}
}

// The picker writes to localStorage and stamps the document; both have to
// happen or the choice is lost on the next page load.
func TestTheAppearancePickerChangesTheTheme(t *testing.T) {
	t.Parallel()

	p := open(t)

	for _, want := range []string{"dark", "light"} {
		p.run("choose "+want, chromedp.SetValue("#theme-picker", want, chromedp.ByID),
			chromedp.Evaluate(
				`document.querySelector('#theme-picker').dispatchEvent(new Event('change'))`, nil))

		p.waitEvaluates("the theme", `document.documentElement.dataset.theme`, want)
	}
}

// What a page from before the rename to TimePulse kept on the device is read
// under its new name from the first load after the update, and nothing is left
// under the old one - or a form half filled in, the screen last open and the
// appearance chosen would all be forgotten the day the installation is updated.
func TestWhatWasKeptBeforeTheRenameIsKeptUnderTheNewName(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	// Both shapes the old names had, a dot after the prefix and an underscore,
	// under names nothing on the page writes back - and a draft, which it reads.
	p.run("keep what a page from before the rename kept", chromedp.Evaluate(`(() => {
		localStorage.setItem('gtr.kept-by-this-case', 'dot');
		localStorage.setItem('gtr_kept_by_this_case', 'underscore');
		sessionStorage.setItem('gtr.draft.' + me.user.id + '.form-timesheet', '{"description":"half done"}');
	})()`, nil))
	p.run("load the page as the update leaves it", chromedp.Reload())
	p.settled()

	var kept struct {
		Dot        string   `json:"dot"`
		Underscore string   `json:"underscore"`
		Draft      string   `json:"draft"`
		Left       []string `json:"left"`
	}

	p.evalJSON(`JSON.stringify({
		dot: localStorage.getItem('timepulse.kept-by-this-case') ?? '',
		underscore: localStorage.getItem('timepulse_kept_by_this_case') ?? '',
		draft: sessionStorage.getItem('timepulse.draft.' + me.user.id + '.form-timesheet') ?? '',
		left: [...Object.keys(localStorage), ...Object.keys(sessionStorage)].filter((key) => key.startsWith('gtr')),
	})`, &kept)

	if kept.Dot != "dot" || kept.Underscore != "underscore" || kept.Draft == "" {
		t.Errorf("taken over: %+v, want both values and the draft under the new names", kept)
	}

	if len(kept.Left) != 0 {
		t.Errorf("still kept under the old name: %v", kept.Left)
	}

	// The appearance too. Signed in, the account decides it and the device's copy
	// is only what the first paint is drawn in; signed out, the copy is what the
	// sign-in screen is drawn in, so that is where it can be seen to survive.
	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))
	p.run("choose dark before the rename", chromedp.Evaluate(`localStorage.setItem('gtr.theme', 'dark')`, nil))
	p.run("load the sign-in screen as the update leaves it", chromedp.Reload(),
		chromedp.WaitVisible("#form-login", chromedp.ByID))

	var screen struct {
		Theme   string `json:"theme"`
		Applied string `json:"applied"`
	}

	p.evalJSON(`JSON.stringify({
		theme: localStorage.getItem('timepulse.theme') ?? '',
		applied: document.documentElement.dataset.themePreference,
	})`, &screen)

	if screen.Theme != "dark" || screen.Applied != "dark" {
		t.Errorf("the appearance chosen before the rename is kept as %q and applied as %q, want dark",
			screen.Theme, screen.Applied)
	}
}
