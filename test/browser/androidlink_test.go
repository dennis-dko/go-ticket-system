//go:build browser

package browser

import (
	"fmt"
	"testing"

	"github.com/chromedp/chromedp"
)

// The card that offers to install the page as an app names the Android app as
// well, and leads to the release it is downloaded from: the release this
// installation runs, because the app belongs to the release it was built with,
// and the newest one for a build that is no release's - the suite's own binary
// is built without a version and says "dev".
func TestTheInstallCardLeadsToTheAndroidAppsRelease(t *testing.T) {
	t.Parallel()

	const releases = "https://github.com/dennis-dko/go-time-recording/releases"

	p := open(t)
	p.readyWorker()

	p.run("open my account", p.click(`.tab[data-view="settings"]`),
		chromedp.WaitVisible("#install-card", chromedp.ByID))

	// The link is narrowed once the version is known, so it is read once the
	// footer shows the version rather than whenever the card happens to appear.
	p.waitEvaluates("the version in the footer",
		`document.querySelector('#footer-version')?.textContent.startsWith('dev') ? 'drawn' : ''`, "drawn")

	if !p.visible("#install-android-link") || !p.visible("#install-android") {
		t.Fatalf("the card shows the link to the Android app %v and the sentence about it %v, want both",
			p.visible("#install-android-link"), p.visible("#install-android"))
	}

	if href := p.attr("#install-android-link", "href"); href != releases+"/latest" {
		t.Errorf("on a build that is no release the card leads to %q, want the newest release", href)
	}

	if target, rel := p.attr("#install-android-link", "target"), p.attr("#install-android-link", "rel"); target != "_blank" || rel != "noopener noreferrer" {
		t.Errorf("the link opens with target %q and rel %q, want a new tab that is not handed this one", target, rel)
	}

	// An installation that runs a release: the page drawn as it is for one.
	p.run("draw the page as a release draws it",
		chromedp.Evaluate(`drawBranding({ ...lastBranding, version: 'v0.4.6' })`, nil))

	if href := p.attr("#install-android-link", "href"); href != releases+"/tag/v0.4.6" {
		t.Errorf("on release v0.4.6 the card leads to %q, want that release", href)
	}

	// Every shape a version arrives in: a release's own, and what a build from
	// source says - how far it is from the last tag, a changed tree, no tag at all.
	for version, want := range map[string]string{
		"v0.4.6":                  releases + "/tag/v0.4.6",
		"v1.0.0":                  releases + "/tag/v1.0.0",
		"v0.4.6-3-g30c92cb":       releases + "/latest",
		"v0.4.6-3-g30c92cb-dirty": releases + "/latest",
		"v0.4.6-dirty":            releases + "/latest",
		"30c92cb":                 releases + "/latest",
		"0.0.0-dev":               releases + "/latest",
		"dev":                     releases + "/latest",
		"":                        releases + "/latest",
	} {
		var got string

		p.run("ask where the app of "+version+" is", chromedp.Evaluate(fmt.Sprintf(`releaseOfTheApp(%q)`, version), &got))

		if got != want {
			t.Errorf("the app of version %q is looked for at %q, want %q", version, got, want)
		}
	}
}
