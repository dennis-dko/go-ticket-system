package installer

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The installer opens on the sign-in card's band: TimePulse's mark on its white
// field and the heading in white beside it, on the same anthracite over the same
// accent line.
//
// The maintainer asked for the mark to look here as it does on the sign-in card,
// on 2026-11-19. The installer is a page of its own that cannot load app.css - it
// runs before there is anything else - so it carries the band's rules itself, and
// this holds them to app.css's, property for property.
func TestTheHeadingIsTheSignInCardsBand(t *testing.T) {
	page, err := assets.ReadFile("asset/install.html")
	if err != nil {
		t.Fatal(err)
	}

	css, err := os.ReadFile("../web/asset/app.css")
	if err != nil {
		t.Fatal(err)
	}

	for _, pair := range []struct {
		web, installer string
		properties     []string
	}{
		{".login-band", ".band", []string{"align-items", "background", "border-bottom", "display", "flex-wrap", "gap", "padding"}},
		{".login-emblem", ".emblem", []string{"background", "border-radius", "display", "flex", "padding"}},
		{".login-mark", ".band .mark", []string{"height", "width"}},
		{".login-band h2", ".band h1", []string{"color", "flex", "font-size", "font-weight", "line-height", "margin"}},
	} {
		theirs := declarations(t, string(css), pair.web)
		mine := declarations(t, string(page), pair.installer)

		for _, property := range pair.properties {
			if theirs[property] == "" {
				t.Fatalf("app.css's %s sets no %s, so there is nothing to hold the installer to", pair.web, property)
			}

			if mine[property] != theirs[property] {
				t.Errorf("the installer's %s has %s: %q, where the sign-in card's %s has %q",
					pair.installer, property, mine[property], pair.web, theirs[property])
			}
		}
	}
}

// declarations reads one rule's properties out of a stylesheet, its comments
// left out and its values with their spaces collapsed.
func declarations(t *testing.T, sheet, selector string) map[string]string {
	t.Helper()

	sheet = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(sheet, "")

	rule := regexp.MustCompile(`(?m)(?:^|[\s}])` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(sheet)
	if rule == nil {
		return map[string]string{}
	}

	found := map[string]string{}

	for _, m := range regexp.MustCompile(`([a-z-]+)\s*:\s*([^;]+);`).FindAllStringSubmatch(rule[1], -1) {
		found[m[1]] = strings.Join(strings.Fields(m[2]), " ")
	}

	return found
}
