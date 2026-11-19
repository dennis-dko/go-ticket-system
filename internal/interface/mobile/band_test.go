package mobile

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The connect card's band draws the mark as the web interface's sign-in card
// does: the same mark on the same white field, with the same room around it and
// the same size of heading beside it.
//
// The maintainer asked for exactly that on 2026-11-19, when the app's mark was a
// third of the size of the sign-in card's on a band that otherwise looked the
// same. Look.kt keeps the band's measures together as SignInBand, and they are
// app.css's: .login-band, .login-emblem, .login-mark and the band's heading.
func TestTheAppsBandDrawsTheMarkAsTheSignInCardDoes(t *testing.T) {
	css := read(t, "../web/asset/app.css")
	look, _ := lexed(source(t, "Look.kt"))

	block := func(selector string) string {
		at := strings.Index(css, "\n"+selector+" {")
		if at < 0 {
			t.Fatalf("app.css has no %s block", selector)
		}

		rest := css[at:]

		return rest[:strings.Index(rest, "}")]
	}

	pixels := func(selector, property string) []int {
		m := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(property) + `:\s*([^;]+);`).FindStringSubmatch(block(selector))
		if m == nil {
			t.Fatalf("app.css's %s sets no %s", selector, property)
		}

		var out []int

		for _, part := range strings.Fields(m[1]) {
			n, err := strconv.Atoi(strings.TrimSuffix(part, "px"))
			if err != nil {
				t.Fatalf("app.css's %s %s is %q, which is not in pixels", selector, property, m[1])
			}

			out = append(out, n)
		}

		return out
	}

	padding := pixels(".login-band", "padding")
	gap := pixels(".login-band", "gap")

	if len(padding) != 4 || len(gap) != 2 {
		t.Fatalf("the band's padding %v and gap %v are not in the shape this reads", padding, gap)
	}

	want := map[string]int{
		"top":           padding[0],
		"end":           padding[1],
		"bottom":        padding[2],
		"start":         padding[3],
		"rowGap":        gap[0],
		"gap":           gap[1],
		"emblemPadding": pixels(".login-emblem", "padding")[0],
		"emblemCorner":  pixels(".login-emblem", "border-radius")[0],
		"mark":          pixels(".login-mark", "width")[0],
		"heading":       pixels(".login-band h2", "font-size")[0],
	}

	object := regexp.MustCompile(`object SignInBand \{([^}]*)\}`).FindStringSubmatch(look)
	if object == nil {
		t.Fatal("Look.kt keeps no SignInBand, so nothing holds the band to the sign-in card's")
	}

	mine := map[string]int{}

	for _, m := range regexp.MustCompile(`val (\w+) = (\d+)\.(?:dp|sp)`).FindAllStringSubmatch(object[1], -1) {
		mine[m[1]], _ = strconv.Atoi(m[2])
	}

	for name, value := range want {
		if got, ok := mine[name]; !ok || got != value {
			t.Errorf("the app's band has %s = %d, where the sign-in card's is %d", name, got, value)
		}
	}
}
