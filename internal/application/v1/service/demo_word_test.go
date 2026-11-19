package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/internal/support/i18n"
)

// Everything the demo writes is in every dictionary, under demo.*, and nothing
// there is a word the demo no longer writes.
//
// The demo is written in the language the installer was read in, from the one
// dictionary every part of the application reads; its English stands beside the
// code. A text missing from a dictionary would be written in English into an
// installation set up in another language, which nothing else would notice.
func TestTheDemosWordsAreInEveryDictionary(t *testing.T) {
	written := map[string]bool{"demo.standup": true}

	for _, p := range demoProjects {
		written["demo."+p.key+".name"] = true
		written["demo."+p.key+".description"] = true

		for i := range p.work {
			written[fmt.Sprintf("demo.%s.%d", p.key, i+1)] = true
		}
	}

	for _, language := range i18n.Languages()[1:] {
		words := i18n.Dictionary(language)

		for key := range written {
			if words[key] == "" {
				t.Errorf("the demo writes %s and %s has no words for it", key, language)
			}
		}

		for key := range words {
			if strings.HasPrefix(key, "demo.") && !written[key] {
				t.Errorf("%s has %s, which the demo no longer writes", language, key)
			}
		}
	}

	if got := demoText(i18n.For("de"), demoSupport, demoProjects[demoSupport].work[1]); got != "Rückruf beim Kunden" {
		t.Errorf("a German support booking reads %q", got)
	}
}
