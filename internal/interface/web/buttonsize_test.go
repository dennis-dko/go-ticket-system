package web_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The buttons that keep their own size are named alike wherever the pills are
// sized, and every button with a class of its own is either named there or acts.
//
// A button that acts is a pill as high as a field; the small ones - a link in a
// row, the cross on a notice, the eye inside a password field, the calendar
// inside a date field - keep the sizes their own rules give them. CSS has no way
// to name that list once, so it is written three times: for the height, for the
// shadow under the form's own answer, and again for a touch screen, where a
// rule lowering everything to 44 pixels would otherwise win. A button left out
// of one copy is a 48-pixel pill, or a shadowed one, in that copy's case only -
// a cross the height of a field on a telephone, and nowhere else.
func TestTheButtonsThatKeepTheirOwnSizeAreNamedAlike(t *testing.T) {
	css := asset(t, "/app.css")

	lists := regexp.MustCompile(`button:where\(:not\(([^)]*)\)\)`).FindAllStringSubmatch(css, -1)
	if len(lists) != 3 {
		t.Fatalf("the stylesheet sizes buttons with %d :where lists, want three", len(lists))
	}

	names := func(list string) []string {
		var out []string

		for _, name := range strings.Split(list, ",") {
			out = append(out, strings.TrimSpace(name))
		}

		slices.Sort(out)

		return out
	}

	height, shadow, touch := names(lists[0][1]), names(lists[1][1]), names(lists[2][1])

	if !slices.Equal(height, touch) {
		t.Errorf("a touch screen keeps %v at their own size, and every other screen %v", touch, height)
	}

	for _, name := range height {
		if !slices.Contains(shadow, name) {
			t.Errorf("%s keeps its own size but is given the shadow of the form's own answer", name)
		}
	}

	// And every class a button carries in the markup or the script is either one
	// of those or one that acts.
	acting := map[string]bool{".secondary": true, ".danger": true, ".row-apart": true, ".login-submit": true,
		".bulk-assign": true}
	markup := asset(t, "/") + asset(t, "/app.js")

	for _, m := range regexp.MustCompile(`(?:<button[^>]*\sclass="|el\('button', \{[^}]*class: ')([^"']+)`).
		FindAllStringSubmatch(markup, -1) {
		for _, class := range strings.Fields(m[1]) {
			name := "." + class

			if !acting[name] && !slices.Contains(height, name) && !strings.HasPrefix(class, "week-") {
				t.Errorf("a button carries the class %s, which is neither named among the small ones nor "+
					"known to act", name)
			}
		}
	}
}
