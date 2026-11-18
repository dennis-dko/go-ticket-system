package mobile

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The app's icons are favicon.svg's drawing.
//
// The launcher's tile and figures, the status bar's shape and the mark on the
// app's own screens are vector drawables, and the hourglass they showed before
// was written into each by hand, in coordinates worked out from the SVG's - and
// once more as drawing code in Look.kt. Each drawable carries the SVG's own path
// data and colours now, placed by a group, so each is held to the file here: a
// figure the SVG gains, loses or moves that a drawable does not is reported
// rather than found on somebody's home screen.
func TestTheAppsIconsAreTheMark(t *testing.T) {
	svg := read(t, "../web/asset/favicon.svg")

	var figures []string

	for _, found := range regexp.MustCompile(`<path d="([^"]+)"`).FindAllStringSubmatch(svg, -1) {
		figures = append(figures, found[1])
	}

	hub := regexp.MustCompile(`<circle cx="([^"]+)" cy="([^"]+)" r="([^"]+)"/>`).FindStringSubmatch(svg)
	if hub == nil || len(figures) == 0 {
		t.Fatalf("favicon.svg has %d figures and a hub of %v, so this compares nothing", len(figures), hub)
	}

	radius, err := strconv.ParseFloat(hub[3], 64)
	if err != nil {
		t.Fatalf("the hub's radius: %v", err)
	}

	// A drawable has no circles, so the hub is two arcs there: from its centre
	// out to its left edge, then round and back.
	across := strconv.FormatFloat(2*radius, 'f', -1, 64)
	figures = append(figures, "M"+hub[1]+" "+hub[2]+"m-"+hub[3]+" 0a"+hub[3]+" "+hub[3]+" 0 1 0 "+across+
		" 0a"+hub[3]+" "+hub[3]+" 0 1 0 -"+across+" 0Z")

	var colours []string

	for _, found := range regexp.MustCompile(`stop-color="#([0-9a-f]{6})"`).FindAllStringSubmatch(svg, -1) {
		colours = append(colours, `android:color="#FF`+strings.ToUpper(found[1])+`"`)
	}

	drawable := func(name string) string {
		return read(t, "src/main/res/drawable/"+name)
	}

	for _, name := range []string{"ic_launcher_foreground.xml", "ic_notification.xml", "ic_mark.xml"} {
		var drawn []string

		for _, found := range regexp.MustCompile(`android:pathData="([^"]+)"`).FindAllStringSubmatch(drawable(name), -1) {
			drawn = append(drawn, found[1])
		}

		for _, figure := range figures {
			if !slices.Contains(drawn, figure) {
				t.Errorf("%s does not draw favicon.svg's figure %.48q", name, figure)
			}
		}
	}

	for _, name := range []string{"ic_launcher_background.xml", "ic_mark.xml"} {
		body := drawable(name)

		for _, colour := range colours {
			if !strings.Contains(body, colour) {
				t.Errorf("%s's tile does not pass through %s, a colour of favicon.svg's", name, colour)
			}
		}

		if stops := strings.Count(body, "<item "); stops != len(colours) {
			t.Errorf("%s's tile passes through %d colours and favicon.svg's through %d", name, stops, len(colours))
		}
	}

	if !strings.Contains(source(t, "Look.kt"), "painterResource(R.drawable.ic_mark)") {
		t.Error("Look.kt does not take the mark from ic_mark, so the app's own screens draw a copy of their own")
	}
}
