//go:build browser

package browser

import (
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// Settings and My account are each sorted into tabs, and only the cards of the
// chosen tab are on screen.
//
// Both had grown to sixteen cards in one column - Settings to well over six
// thousand pixels - which is a screen somebody scrolls through rather than reads.
// The tabs are a tab strip in every sense a keyboard and a screen reader know:
// the chosen one is selected, and the arrow keys walk along them.
func TestTheLongScreensAreSortedIntoTabs(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open Settings", p.click(`.tab[data-view="admin"]`), chromedp.WaitVisible(`#admin-tabs`, chromedp.ByQuery))

	if !p.visible("#form-branding") || p.visible("#form-ldap") || p.visible("#log-card") {
		t.Error("Settings does not open on its first tab alone: the appearance should be shown, " +
			"and the directory and the log should not")
	}

	p.run("choose the sign-in tab", p.click(`#admin-tabs [data-card-tab="signin"]`))

	if !p.visible("#form-ldap") || p.visible("#form-branding") {
		t.Error("choosing Sign-in did not put the directory on screen in place of the appearance")
	}

	if selected := p.attr(`#admin-tabs [data-card-tab="signin"]`, "aria-selected"); selected != "true" {
		t.Errorf("the chosen tab says aria-selected=%q", selected)
	}

	p.run("walk on with the arrow key", chromedp.KeyEvent(kb.ArrowRight))

	if !p.visible("#form-datasource") || p.visible("#form-ldap") {
		t.Error("the right arrow did not move on to the next tab")
	}

	var focused string

	p.run("ask what has the focus", chromedp.Evaluate(`document.activeElement?.dataset.cardTab ?? ''`, &focused))

	if focused != "operation" {
		t.Errorf("after the right arrow the focus is on %q, want the tab it moved to", focused)
	}

	p.run("open My account", p.click(`.tab[data-view="settings"]`), chromedp.WaitVisible(`#settings-tabs`, chromedp.ByQuery))

	// It comes back on the tab it was left on, and replacing the initial password
	// left it on the one the password is filed under.
	if selected := p.attr(`#settings-tabs [data-card-tab="signin"]`, "aria-selected"); selected != "true" {
		t.Errorf("My account did not come back on the tab it was left on: Sign-in says aria-selected=%q", selected)
	}

	p.run("choose the working time", p.click(`#settings-tabs [data-card-tab="work"]`))

	if p.visible("#form-password") || !p.visible("#form-my-timezone") {
		t.Error("the working-time tab of My account does not show the timezone alone without the password")
	}

	p.run("choose the sign-in tab", p.click(`#settings-tabs [data-card-tab="signin"]`))

	if !p.visible("#form-password") || !p.visible("#totp-card") {
		t.Error("the sign-in tab of My account does not hold the password and the second factor")
	}
}

// A tab with nothing on it is not offered, and comes back when something is.
//
// What a card holds and who may see it is decided by the code that loads it and
// by the account's rights, neither of which knows about tabs. So the tabs follow
// the cards: one whose every card is hidden goes, and the chosen one moves to the
// first that is left.
func TestATabWithNothingOnItIsNotOffered(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open the logs", p.click(`.tab[data-view="admin"]`), chromedp.WaitVisible(`#admin-tabs`, chromedp.ByQuery),
		p.click(`#admin-tabs [data-card-tab="logs"]`))

	p.run("hide every card on the logs tab", chromedp.Evaluate(`
		for (const card of document.querySelectorAll('#admin-panel-logs > .card')) card.hidden = true;`, nil))
	p.waitGone(`#admin-tabs [data-card-tab="logs"]`)

	if selected := p.attr(`#admin-tabs [data-card-tab="general"]`, "aria-selected"); selected != "true" {
		t.Errorf("with the chosen tab gone, the first tab says aria-selected=%q", selected)
	}

	p.run("show the log again", chromedp.Evaluate(`document.querySelector('#log-card').hidden = false`, nil),
		chromedp.WaitVisible(`#admin-tabs [data-card-tab="logs"]`, chromedp.ByQuery))
}

// Whatever points at a card opens the tab it is on.
//
// A banner's link to the version card, and the guided tour's step on the log,
// both used to be a scroll to an element that was always on the page. On a tab
// that is not chosen the element is there and not shown, and a scroll to it
// lands nowhere - so each opens the card's tab first.
func TestWhatPointsAtACardOpensItsTab(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("follow the way to the version card", chromedp.Evaluate(`openTheVersionCard()`, nil))

	if !p.visible("#update-card") {
		t.Error("the way to the version card did not show it")
	}

	var reachable bool

	p.run("ask whether the tour would show the log", chromedp.Evaluate(`
		applicableTourSteps().some((step) => step.target === '#log-card')`, &reachable))

	if !reachable {
		t.Fatal("the tour leaves out the log card, which is only on another tab")
	}

	p.run("walk the tour to the log", chromedp.Evaluate(`
		(async () => {
			await startTour();
			tour.index = tour.steps.findIndex((step) => step.target === '#log-card');
			await renderTourStep();
		})()`, nil, awaitPromise))

	if !p.visible("#log-card") {
		t.Error("the tour's step on the log did not open the tab the log is on")
	}
}

// The log is read only while its tab is open.
//
// It polls, and it was told to stop whenever its screen was left so that a log
// left open overnight would not fill the server's buffer with its own requests.
// On a tab that is not chosen it is just as unseen.
func TestTheLogIsReadOnlyWhileItsTabIsOpen(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	var active bool

	p.run("open Settings on its first tab", p.click(`.tab[data-view="admin"]`),
		chromedp.WaitVisible(`#admin-tabs`, chromedp.ByQuery),
		chromedp.Evaluate(`logViewerActive()`, &active))

	if active {
		t.Error("the log is read while Settings shows another tab")
	}

	p.run("open the logs", p.click(`#admin-tabs [data-card-tab="logs"]`),
		chromedp.Evaluate(`logViewerActive()`, &active))

	if !active {
		t.Error("the log is not read while its tab is open")
	}
}
