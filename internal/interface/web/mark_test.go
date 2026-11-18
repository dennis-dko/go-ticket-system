package web_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The mark beside the title, and the one before the installer's heading, are the
// tab icon, number for number.
//
// All of them are this application's mark: the page draws it from a symbol of its
// own, the installer - which loads none of the page - carries its own copy, and
// the tab shows favicon.svg. The hourglass that stood in two of those places
// before was written out by hand in each, and copies nothing compares are how one
// of them comes to show an old picture - so each copy's figures, hub, tile and
// colours are read and held to the file's.
func TestTheMarkBesideTheTitleIsTheTabIcon(t *testing.T) {
	icon := asset(t, "/favicon.svg")

	installer, err := os.ReadFile(filepath.Join("..", "installer", "asset", "install.html"))
	if err != nil {
		t.Fatalf("reading the installer's page: %v", err)
	}

	for _, copied := range []struct{ where, markup, opening, closing string }{
		{"the mark beside the title", asset(t, "/"), `<symbol id="icon-mark"`, "</symbol>"},
		{"the installer's mark", string(installer), `<svg class="mark"`, "</svg>"},
	} {
		at := strings.Index(copied.markup, copied.opening)
		if at < 0 {
			t.Errorf("%s is not where it was looked for (%s)", copied.where, copied.opening)

			continue
		}

		end := strings.Index(copied.markup[at:], copied.closing)
		if end < 0 {
			t.Errorf("%s is never closed", copied.where)

			continue
		}

		drawn := copied.markup[at : at+end]

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

			if got := find.FindAllString(drawn, -1); !slices.Equal(got, want) {
				t.Errorf("%s has %q where the tab icon has %q", copied.where, got, want)
			}
		}
	}
}
