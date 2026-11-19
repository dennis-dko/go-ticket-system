//go:build firefox

package firefox

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/test/harness"
)

// Firefox adopts TimePulse's mark as the tab's icon, whatever logo is saved.
//
// Asked of what Firefox decided rather than of the document alone: what a tab
// draws is not in the document and not in the response, and Firefox writes its
// decision down in its own icon store. Firefox is also where the tab went wrong
// while a logo stood in for the mark - from the same markup, it kept the shipped
// .ico where Chrome took the logo. The logo stands at the left of the bar now and
// nowhere else, so with one saved the page still declares the shipped pair, and
// the mark is what Firefox keeps.
func TestFirefoxAdoptsTimePulsesMarkWhateverTheLogo(t *testing.T) {
	app := harness.Start(t)
	b := openBrowser(t)

	b.goTo(app.BaseURL())
	signIn(t, b)

	// One red pixel: plainly an uploaded logo and plainly not the mark.
	const logo = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

	post(t, b, "/api/v1/settings/branding",
		fmt.Sprintf(`{"title":"Zeiterfassung","logo":%q}`, logo), "PUT")

	b.goTo(app.BaseURL())
	b.settle()

	if icons := strings.Join(declaredIcons(t, b), " "); icons != "/favicon.svg /favicon.ico" {
		t.Errorf("the page declares %q, want the shipped mark's SVG and .ico", icons)
	}

	// Adopting is not instant: the icon is fetched after the document, and written
	// to the store after that. Either file is the mark.
	adopted := waitForAdoptedIcon(t, b, "favicon.")
	if adopted == "" {
		t.Fatal("Firefox recorded no icon for this page, so the tab stays empty")
	}

	// Only this installation's rows: a new profile's store also holds the icons of
	// Firefox's own bookmarks. Firefox keeps the touch icon as well, and that is
	// the mark too, drawn at the app's size.
	origin, err := url.Parse(app.BaseURL())
	if err != nil {
		t.Fatalf("reading the instance's address: %v", err)
	}

	for _, row := range recordedIcons(t, b.profile) {
		if !strings.Contains(row, origin.Host) {
			continue
		}

		if !strings.Contains(row, "/favicon.") && !strings.Contains(row, "/app-icon/") {
			t.Errorf("Firefox adopted %s, which is not TimePulse's mark", row)
		}
	}

	t.Logf("Firefox adopted %s", adopted)
}

// declaredIcons is every icon link on the page, in document order.
func declaredIcons(t *testing.T, b *browser) []string {
	t.Helper()

	var icons []string

	b.evalJSON(`JSON.stringify([...document.querySelectorAll('link[rel~="icon"]')]
		.map(link => link.getAttribute('href')))`, &icons)

	return icons
}
