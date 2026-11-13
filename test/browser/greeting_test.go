//go:build browser

package browser

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// The greeting is a screen, reachable from the title.
//
// It used to be two half-measures: a modal on a first sign-in, and a card wedged
// above another screen afterwards. Neither could be gone back to - the only ways
// to read the greeting were to be new or to have just arrived - which is why it is
// a screen now, and why the title in the header leads to it from anywhere.
func TestTheGreetingIsAScreenReachableFromTheTitle(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("create a user",
		chromedp.Click(`.tab[data-view="users"]`, chromedp.ByQuery),
		chromedp.WaitVisible("#form-user", chromedp.ByID),
		chromedp.SendKeys(`#form-user input[name="name"]`, "Sven", chromedp.ByQuery),
		chromedp.SendKeys(`#form-user input[name="email"]`, "sven@example.com", chromedp.ByQuery),
		p.chooseOption(`#form-user select[name="role"]`, "agent"),
		chromedp.SendKeys(`#form-user input[name="password"]`, "sven-password-1", chromedp.ByQuery),
		p.click(`#form-user button[type="submit"]`),
	)

	time.Sleep(500 * time.Millisecond)

	p.run("sign out", p.click("#logout"), chromedp.WaitVisible("#form-login", chromedp.ByID))

	p.signIn("sven@example.com", "sven-password-1")
	p.waitGone("#login-screen")

	// The walk through, out of the way: what this case is about is the screen
	// behind it.
	p.settleWelcome()

	// Somewhere else entirely, so that arriving at the greeting is a navigation
	// rather than where the page happened to be.
	p.run("go to My account",
		chromedp.Click(`.tab[data-view="settings"]`, chromedp.ByQuery),
		chromedp.WaitVisible("#view-settings", chromedp.ByID))

	p.run("press the title", p.click("#app-title"),
		chromedp.WaitVisible("#view-welcome", chromedp.ByID))

	if title := p.text("#welcome-title"); !strings.Contains(title, "Sven") {
		t.Errorf("the greeting does not name the person: %q", title)
	}

	// The points offered are the ones this person can act on, and there is
	// something there at all: the list is built from permissions, so a near-empty
	// greeting means the building went wrong rather than that there is little to
	// say.
	points := p.text("#welcome-points")

	if len(points) < 60 {
		t.Errorf("the greeting lists almost nothing this person can do: %q", points)
	}

	if strings.Contains(strings.ToLower(points), "genehmig") {
		t.Errorf("the greeting promises approvals, which nobody does any more: %q", points)
	}

	// And it is a screen you leave the way you leave any other.
	p.run("carry on", p.click("#welcome-continue"))
	p.waitGone("#view-welcome")
}
