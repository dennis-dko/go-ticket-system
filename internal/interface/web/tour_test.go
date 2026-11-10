package web_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The guided tour points at real controls by selector. That is what keeps it
// honest - it cannot describe a button that no longer exists - but it also
// means a renamed id turns a step into a highlight around nothing. The code
// skips a missing target at run time, so the failure is silent: the step just
// never appears, and nobody notices the tour got shorter.
//
// These tests make that loud instead.

var (
	tourStepPattern   = regexp.MustCompile(`(?s)const TOUR_STEPS = \[(.*?)\n\];`)
	tourTargetPattern = regexp.MustCompile(`target:\s*'([^']+)'`)
	tourViewPattern   = regexp.MustCompile(`view:\s*'([^']+)'`)
)

// tourSection returns the step table as written.
func tourSection(t *testing.T) string {
	t.Helper()

	js := asset(t, "/app.js")

	match := tourStepPattern.FindStringSubmatch(js)
	if match == nil {
		t.Fatal("could not find TOUR_STEPS in app.js")
	}

	return match[1]
}

// Every step has to point at something the page actually contains.
func TestEveryTourStepTargetsSomethingThatExists(t *testing.T) {
	html := asset(t, "/")
	section := tourSection(t)

	targets := tourTargetPattern.FindAllStringSubmatch(section, -1)
	if len(targets) == 0 {
		t.Fatal("the tour has no steps")
	}

	for _, match := range targets {
		selector := match[1]

		// Only id selectors are used, which keeps this check exact rather than
		// an approximation of what a browser would resolve.
		if !strings.HasPrefix(selector, "#") {
			t.Errorf("tour step targets %q; use an id so the target can be verified", selector)

			continue
		}

		if !strings.Contains(html, `id="`+strings.TrimPrefix(selector, "#")+`"`) {
			t.Errorf("tour step points at %q, which is not in the page", selector)
		}
	}
}

// notInTheTour are the cards on a screen the walk through leaves out, each
// with the reason. A card is named by its id, or by the first id inside it
// where it has none.
var notInTheTour = map[string]string{
	"the card holding #welcome-title": "the welcome screen, which the first step - " +
		"the title that leads there - describes",
	"welcome-week":   "part of the welcome screen the first step describes",
	"welcome-recent": "part of the welcome screen the first step describes",
	"calendar-day-card": "the entries of the day picked in the month, which the month's " +
		"step says a click opens",
	"overtime-result":  "the answer to the balance form above it, hidden until it is asked",
	"report-result":    "the answer to the report form above it, hidden until it is asked",
	"statement-result": "the answer to the statement form above it, hidden until it is asked",
	"the card holding #table-roles": "the roles the form above it makes, which that " +
		"form's step describes",
	"the card holding #tour-restart": "the walk through itself, started again from here",
	"sync-elsewhere": "the directory's step says whose the run is, in the words that " +
		"suit the account reading it",
	"sync-card": "the directory's step describes the run below the directory",
}

// screenCard is a card on one of the screens, and every id inside it -
// its own included.
type screenCard struct {
	name string
	ids  map[string]bool
}

var (
	markupComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	markupTag     = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)
	markupID      = regexp.MustCompile(`\bid="([^"]+)"`)
	markupClass   = regexp.MustCompile(`\bclass="([^"]+)"`)
)

// emptyElements close themselves, so they never wait for an end tag.
var emptyElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "source": true,
	"track": true, "wbr": true,
}

// screenCards reads every element whose class names card inside a screen -
// an element whose class names view - with the ids inside it. An end tag
// closes whatever was left open inside its element, so a paragraph or a
// cell closed implicitly does not hold the stack open.
func screenCards(t *testing.T, html string) []screenCard {
	t.Helper()

	type open struct {
		name       string
		card       int
		screen     bool
		insideView bool
	}

	var (
		cards []screenCard
		stack []open
	)

	for _, tag := range markupTag.FindAllStringSubmatch(markupComment.ReplaceAllString(html, ""), -1) {
		closing, name, attrs := tag[1] == "/", strings.ToLower(tag[2]), tag[3]

		if closing {
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].name == name {
					stack = stack[:i]

					break
				}
			}

			continue
		}

		var classes []string
		if match := markupClass.FindStringSubmatch(attrs); match != nil {
			classes = strings.Fields(match[1])
		}

		id := ""
		if match := markupID.FindStringSubmatch(attrs); match != nil {
			id = match[1]
		}

		insideView := len(stack) > 0 && (stack[len(stack)-1].insideView || stack[len(stack)-1].screen)
		element := open{name: name, card: -1, screen: slices.Contains(classes, "view"), insideView: insideView}

		if insideView && slices.Contains(classes, "card") {
			cards = append(cards, screenCard{name: id, ids: map[string]bool{}})
			element.card = len(cards) - 1
		}

		if id != "" {
			for _, outer := range append(stack, element) {
				if outer.card >= 0 {
					cards[outer.card].ids[id] = true

					if cards[outer.card].name == "" {
						cards[outer.card].name = "the card holding #" + id
					}
				}
			}
		}

		if !emptyElements[name] && !strings.HasSuffix(attrs, "/") {
			stack = append(stack, element)
		}
	}

	if len(cards) == 0 {
		t.Fatal("found no cards on any screen")
	}

	return cards
}

// Every card on a screen is in the walk through, or is named in notInTheTour
// with the reason it is not.
//
// Nothing asked this, and the walk fell behind the screens it walks: measured
// against them, twenty-four cards had no step - the week, the leave, the days
// away, the working time, the year, the statement, the bounds and the
// reminders, the holidays, the calendar subscription, the export and the
// managers among them. The test for the tour's targets could not notice,
// because every target it had still existed.
//
// A step that rings something inside a card counts for the card, so the month
// stands for the calendar's card and the accounts table for the users'.
func TestEveryCardIsInTheTour(t *testing.T) {
	targets := map[string]bool{}
	for _, match := range tourTargetPattern.FindAllStringSubmatch(tourSection(t), -1) {
		targets[strings.TrimPrefix(match[1], "#")] = true
	}

	named := map[string]bool{}

	for _, card := range screenCards(t, asset(t, "/")) {
		named[card.name] = true

		walked := false
		for id := range card.ids {
			walked = walked || targets[id]
		}

		_, leftOut := notInTheTour[card.name]

		switch {
		case walked && leftOut:
			t.Errorf("%s is in the tour, and notInTheTour still says why it is not", card.name)
		case !walked && !leftOut:
			t.Errorf("%s is in no step of the tour: give it one, or name it in notInTheTour "+
				"with the reason it has none", card.name)
		}
	}

	for name := range notInTheTour {
		if !named[name] {
			t.Errorf("notInTheTour names %s, which is no card on any screen", name)
		}
	}
}

// A step that switches to a view has to name one that exists, or the tour
// lands on a blank page.
func TestEveryTourViewExists(t *testing.T) {
	html := asset(t, "/")
	section := tourSection(t)

	for _, match := range tourViewPattern.FindAllStringSubmatch(section, -1) {
		view := match[1]

		if !strings.Contains(html, `id="view-`+view+`"`) {
			t.Errorf("tour step switches to view %q, which is not in the page", view)
		}
	}
}

// The tour is offered on a first sign-in, so it has to be reachable again
// afterwards - otherwise the only way to see it twice is a database edit.
func TestTheTourCanBeRestarted(t *testing.T) {
	html := asset(t, "/")
	js := asset(t, "/app.js")

	if !strings.Contains(html, `id="tour-restart"`) {
		t.Error("expected a control to start the tour again")
	}

	if !strings.Contains(js, "#tour-restart") {
		t.Error("the restart control is not wired to anything")
	}
}

// Every way out has to record the tour as seen, or someone who skipped it is
// greeted with it again on the next sign-in - overriding a decision they
// already made.
func TestSkippingTheTourCountsAsSeen(t *testing.T) {
	js := asset(t, "/app.js")

	if !strings.Contains(functionSource(t, js, "endTour"), "recordTourSeen") {
		t.Error("ending the tour must record it as seen")
	}

	if !strings.Contains(functionSource(t, js, "recordTourSeen"), "/me/tour") {
		t.Error("recordTourSeen no longer tells the server the tour has been seen")
	}

	// The skip button and the finish button both have to go through it.
	if !strings.Contains(js, "$('#tour-end').addEventListener('click', endTour)") {
		t.Error("the skip button must end the tour the same way finishing does")
	}
}

// Two things walking someone through the application at once is worse than
// either alone, and the setup wizard is the one that has to happen first.
//
// The built-in administrator is left out of it as well. That account arrives at the
// wizard, which is its own introduction and covers the screens it actually uses; a
// walk through booking time and reading an overtime balance would be a walk through
// somebody else's job. The card under My account still starts it on request.
func TestTheGreetingYieldsToTheWizardAndSkipsTheAdministrator(t *testing.T) {
	js := asset(t, "/app.js")

	start := strings.Index(js, "async function maybeWelcome()")
	if start < 0 {
		t.Fatal("could not find maybeWelcome in app.js")
	}

	body := js[start : start+600]

	for _, guard := range []struct{ needle, why string }{
		{"#setup-wizard", "the greeting must not appear while the setup wizard is showing"},
		{"tourSeen", "the greeting must only be offered to someone who has not seen it"},
		{"administersOnly()", "an account that only administers must not be greeted with " +
			"a tour of booking time, which is a tour of somebody else's job"},
		{"mustChangePassword", "the greeting must wait until the initial password is replaced"},
	} {
		if !strings.Contains(body, guard.needle) {
			t.Error(guard.why)
		}
	}

	// Declining has to count, or the greeting returns on the next sign-in and
	// overrides a decision somebody already made.
	if !strings.Contains(js, "recordTourSeen()") {
		t.Error("declining the greeting must record it as seen")
	}
}

// The restart notice appears when something is waiting for a restart, and not
// otherwise.
//
// It used to stay on screen wherever restarting is impossible - on Windows,
// which has no execve - whether anything was pending or not, so that nobody
// could discover the limitation by pressing a button that never appears. The
// reasoning was sound and the result was not: a warning that is on the screen
// every time you open it is furniture, read once and looked past thereafter,
// including on the day it finally has something to say.
//
// The limitation is still explained, at the moment it costs something. The first
// save that needs a restart brings the notice up, and it puts the reason where
// the button would have been. Nothing is hidden; it is said when it matters
// instead of always.
//
// Checked in the asset rather than in a browser because the browser suite runs on
// Linux, where restarting is supported and this branch is never taken.
func TestTheRestartNoticeAppearsOnlyWhenSomethingIsWaiting(t *testing.T) {
	js := asset(t, "/app.js")

	if !strings.Contains(js, "banner.hidden = pending.length === 0;") {
		t.Error("the restart notice is shown on an empty pending list, which makes a " +
			"standing warning out of something that should speak up when it has something")
	}

	// And the explanation is translated rather than shown as the server wrote it,
	// keyed on which refusal it is - see TestEveryRestartRefusalIsExplained.
	if !strings.Contains(js, "restart.unsupported.${state.reasonCode") {
		t.Error("the limitation is shown in the server's English wording, or is not " +
			"keyed on which refusal it is")
	}

	// The hint above the list promises saved changes, so it goes when there are
	// none: the card is always on the settings screen, and a standing promise
	// above an empty list reads as a rendering fault.
	if !strings.Contains(js, "$('#restart-card-hint').hidden = !waiting") {
		t.Error("the notice promises a list of pending changes even when there are none")
	}
}

// The restart notice is a banner, and only for the people who administer.
//
// It was a card under Settings, which is only read by somebody who has already
// gone looking - so an administrator who saved something, went off to do
// anything else and came back the next day had nothing anywhere telling them the
// installation was still running on the old values. A banner is on every screen,
// which is where a fact about the whole installation belongs.
//
// That is also what makes the permission its own decision. On a card behind a
// tab that needs settings:manage, who could see it was decided by the tab; a
// banner has no tab to inherit from, and nobody who cannot act on this should be
// told an installation they use is waiting for something.
//
// There is a card again, and this asserted for a while that there was not. The
// reason given was that the same thing would be said twice and one copy would go
// stale - which was right about the risk and wrong about what the card is for.
// The banner is the notice: something is waiting, here is what. The card is the
// control: this installation can be restarted, here is what that does here, and
// here is the button. Without it there was no way to restart on purpose at all,
// and on a container deployment - where what the button does is the interesting
// part - the screen said nothing about it.
//
// The staleness is answered directly instead, below: one function fills both
// from one answer, so there is nothing to drift.
func TestTheRestartNoticeIsABannerAndTheControlIsACard(t *testing.T) {
	js := asset(t, "/app.js")
	html := asset(t, "/")

	if !strings.Contains(html, `id="restart-banner"`) {
		t.Error("there is no restart banner in the markup")
	}

	if !strings.Contains(html, `id="restart-card"`) {
		t.Error("there is no restart card under Settings, so an installation with " +
			"nothing pending offers no way to restart on purpose")
	}

	// The list of what is waiting appears once, and it is on the card: that is
	// where somebody acts on it, and a card's worth of detail stuck to the top of
	// every screen is what the banner used to be.
	if !strings.Contains(html, `id="restart-card-pending"`) {
		t.Error("the card does not list what is waiting, so the banner sends people " +
			"to a card that does not answer the question")
	}

	if strings.Contains(html, `id="restart-pending"`) {
		t.Error("the banner still lists the pending changes as well as the card")
	}

	// The banner points rather than acts, and points with a word in its own
	// sentence rather than a control beside it - what somebody wants to press is
	// the thing being talked about, and a button reading "go to the restart"
	// next to a sentence about a restart is the same word twice.
	//
	// Drawn rather than written into the markup, because it sits inside a
	// sentence whose shape belongs to whoever translated it.
	if !strings.Contains(js, `id: 'restart-open'`) {
		t.Error("the banner offers no way to the card it is a notice about")
	}

	if !strings.Contains(js, "').split('{1}')") {
		t.Error("the link is put beside the sentence rather than into it, so a " +
			"translation cannot decide where in the sentence it falls")
	}

	if strings.Contains(html, `id="restart-now"`) {
		t.Error("the banner still carries a restart button of its own")
	}

	// Not dismissable, and nothing to dismiss it with. It is a state rather than
	// news: it goes when the settings are put back or the application restarts.
	if strings.Contains(html, "restart-dismiss") {
		t.Error("the restart banner can be dismissed; it reports a state, and a " +
			"state that has been clicked away is still the state")
	}

	if !strings.Contains(js, "loadRestart") || !strings.Contains(js,
		`const banner = $('#restart-banner');`) {
		t.Error("the loader does not drive the banner")
	}

	// The permission, decided where the banner is filled rather than inherited
	// from a screen it no longer lives on.
	loader := js[strings.Index(js, "async function loadRestart()"):]
	loader = loader[:strings.Index(loader, "\n}\n")]

	if !strings.Contains(loader, "can('settings:manage')") {
		t.Error("the restart banner is filled without asking whether the reader may " +
			"administer this installation")
	}
}

// The footer says which platform the build is running on, beside the version.
//
// The same version is published for four platforms and they do not all behave
// alike, so "v1.0" alone does not say what somebody is looking at - which is the
// first question of a support conversation.
func TestTheFooterNamesThePlatformBesideTheVersion(t *testing.T) {
	js := asset(t, "/app.js")

	if !strings.Contains(js, "branding.os") {
		t.Error("the footer never reads the platform the server reports")
	}

	if !strings.Contains(js, "${version} (${platform})") {
		t.Error(`the footer does not render the platform as "version (platform)"`)
	}
}

// What a container is offered depends on whether anything can replace its image.
//
// Swapping the binary inside a container works and does not last: it changes
// that container and not the image it was made from, so the next recreate
// brings the old version back - and a recreate is how a container deployment
// applies anything at all. An update that reverts on a day nobody connects to
// the button they pressed is worse than no button, so there is none, and the
// card says which command to run instead.
//
// With deploy/compose.update.yaml beside it there is something that can replace
// the image, and then the button is the whole update: a new image, a container
// recreated from it, the old image removed.
func TestAContainerIsOfferedTheImageOrToldToUpdateByHand(t *testing.T) {
	js := asset(t, "/app.js")

	if !strings.Contains(js, "if (state.byImage) {") {
		t.Error("the card does not offer a container with an updater the image")
	}

	if !strings.Contains(js, "if (!state.installable) {") {
		t.Error("a container with nothing to replace its image is offered a button " +
			"whose effect the next recreate undoes")
	}

	// And it names the overlay, because "update it by hand" is only half an
	// answer when the other half is a file in this repository.
	if !strings.Contains(js, "deploy/compose.update.yaml") {
		t.Error("the card tells a container to update by hand without mentioning " +
			"what would let it do so from here")
	}
}

// The image update waits rather than asking for a restart it is not performing.
//
// This is the one that made an update look like nothing happening. The press
// used to fall through to the restart below it: the POST stopped the
// application, its restart policy started the same container again from the
// same old image, and the updater's recreate arrived into the middle of that.
// Whichever won came back, which was usually the version that was already
// there.
func TestTheImageUpdateDoesNotAlsoAskForARestart(t *testing.T) {
	js := asset(t, "/app.js")

	press := js[strings.Index(js, "function wireUpdate()"):]
	press = press[:strings.Index(press, "\n}\n")]

	byImage := strings.Index(press, "if (state?.byImage) {")
	restart := strings.Index(press, "'/settings/restart'")

	if byImage < 0 {
		t.Fatal("the press does not tell an image update from a restart")
	}

	if restart >= 0 && restart < byImage {
		t.Error("the press asks for a restart before it has decided whether " +
			"something else is already replacing this container")
	}

	if !strings.Contains(press, "IMAGE_UPDATE_TIMEOUT_MS") {
		t.Error("the image update is given the same patience as a restart, and it " +
			"has a download in front of it")
	}
}
