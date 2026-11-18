package web_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The mark beside the title is the tab icon, number for number.
//
// Both are this application's mark: the page draws it from a symbol of its own
// and the tab shows favicon.svg. The hourglass that stood in both before was
// written out twice by hand, and two copies nothing compares are how one of them
// comes to show an old picture - so the symbol's figures, hub, tile and colours
// are read and held to the file's.
func TestTheMarkBesideTheTitleIsTheTabIcon(t *testing.T) {
	icon := asset(t, "/favicon.svg")
	page := asset(t, "/")

	at := strings.Index(page, `<symbol id="icon-mark"`)
	if at < 0 {
		t.Fatal("the page defines no symbol for the mark")
	}

	end := strings.Index(page[at:], "</symbol>")
	if end < 0 {
		t.Fatal("the mark's symbol is never closed")
	}

	symbol := page[at : at+end]

	for _, pattern := range []string{
		`<path d="[^"]+"`,
		`<circle [^>]+>`,
		`<stop [^>]+>`,
		`viewBox="[^"]+"`,
		`<rect width="[^"]+" height="[^"]+" rx="[^"]+"`,
		`x1="[^"]+" y1="[^"]+" x2="[^"]+" y2="[^"]+"`,
		`<g fill="[^"]+">`,
	} {
		find := regexp.MustCompile(pattern)

		want := find.FindAllString(icon, -1)
		if len(want) == 0 {
			t.Errorf("favicon.svg has nothing like %s, so this compares nothing", pattern)

			continue
		}

		if got := find.FindAllString(symbol, -1); !slices.Equal(got, want) {
			t.Errorf("the mark beside the title has %q where the tab icon has %q", got, want)
		}
	}
}
