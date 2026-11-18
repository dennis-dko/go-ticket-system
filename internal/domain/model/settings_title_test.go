package model_test

import (
	"testing"

	"github.com/dennis-dko/timepulse/internal/domain/model"
)

// The tab carries TimePulse and an addition of its own where an installation
// wrote one, and the title's addition where it did not.
//
// The fallback is what every installation that has never opened this setting
// relies on: the field arrived after they were all configured, so it is empty
// everywhere, and an empty field that meant no addition would take the title's
// off every tab.
func TestTheTabAddsItsOwnWordsOrTheTitles(t *testing.T) {
	for name, tc := range map[string]struct {
		branding model.Branding
		want     string
	}{
		"nothing added": {
			branding: model.Branding{},
			want:     "TimePulse",
		},
		"the title's addition": {
			branding: model.Branding{Title: "Beispiel GmbH & Co. KG"},
			want:     "TimePulse | Beispiel GmbH & Co. KG",
		},
		"its own addition": {
			branding: model.Branding{Title: "Beispiel GmbH & Co. KG", TabTitle: "Beispiel"},
			want:     "TimePulse | Beispiel",
		},
		// Spaces are not an addition. Somebody clearing the field leaves whatever
		// the keyboard left behind, and a tab with " " added has nothing added.
		"blank": {
			branding: model.Branding{Title: "htp.net", TabTitle: "   "},
			want:     "TimePulse | htp.net",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.branding.TabName(); got != tc.want {
				t.Errorf("the tab is called %q, want %q", got, tc.want)
			}
		})
	}
}
