//go:build firefox

package firefox

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/test/harness"
)

// The tab keeps TimePulse's mark in Firefox, whatever logo is saved.
//
// Firefox is where the tab went wrong while a logo stood in for the mark: from
// the same markup, it kept the shipped .ico where Chrome took the logo. The logo
// stands at the left of the bar now and nowhere else, so the page declares the
// shipped pair whatever is uploaded - and this asks Firefox what it makes of the
// first, by the bytes rather than by the address.
func TestFirefoxKeepsTimePulsesMarkInTheTab(t *testing.T) {
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

	served := b.evalString(`(async () => {
		const link = document.querySelector('link[rel~="icon"]');
		if (!link) return 'no icon';

		const answer = await fetch(link.href);

		return (await answer.text()).includes('aria-label="TimePulse"') ? 'the mark' : 'something else';
	})()`)

	if served != "the mark" {
		t.Errorf("the tab's icon answers with %s rather than TimePulse's mark", served)
	}

	if width := iconWidth(t, b); width == 0 {
		t.Error("Firefox could not draw what the tab is pointed at")
	}
}

// iconWidth fetches whatever the page points its icon at and measures it.
func iconWidth(t *testing.T, b *browser) int {
	t.Helper()

	raw := b.evalString(`(async () => {
		const link = document.querySelector('link[rel~="icon"]');
		if (!link) return '0';

		return await new Promise((resolve) => {
			const probe = new Image();
			probe.onload = () => resolve(String(probe.naturalWidth));
			probe.onerror = () => resolve('0');
			probe.src = link.href;
		});
	})()`)

	width, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}

	return width
}

// declaredIcons is every icon link on the page, in document order.
func declaredIcons(t *testing.T, b *browser) []string {
	t.Helper()

	var icons []string

	b.evalJSON(`JSON.stringify([...document.querySelectorAll('link[rel~="icon"]')]
		.map(link => link.getAttribute('href')))`, &icons)

	return icons
}
