package model_test

import (
	"testing"

	"github.com/dennis-dko/timepulse/internal/domain/model"
)

// What an installation calls itself in the language that is reading: TimePulse,
// and the addition written for that language.
func TestTheInstanceIsNamedInTheLanguageThatAsked(t *testing.T) {
	t.Parallel()

	branding := model.Branding{
		Title: "htp.net",
		Translations: map[string]model.BrandingText{
			"de": {Title: "htp.net Deutschland"},
			"en": {Title: "htp.net"},
		},
	}

	if got := branding.NameIn("de"); got != "TimePulse | htp.net Deutschland" {
		t.Errorf("a German reader is shown %q", got)
	}

	// A language with no addition of its own falls back to the untranslated one
	// rather than to nothing.
	if got := branding.NameIn("fr"); got != "TimePulse | htp.net" {
		t.Errorf("an untranslated language is shown %q", got)
	}

	// And a translation that exists but is empty is not an addition.
	empty := model.Branding{
		Title:        "htp.net",
		Translations: map[string]model.BrandingText{"de": {Title: "   "}},
	}

	if got := empty.NameIn("de"); got != "TimePulse | htp.net" {
		t.Errorf("a blank translation was used as an addition: %q", got)
	}

	// Nothing written anywhere is TimePulse alone.
	if got := (model.Branding{}).NameIn("de"); got != "TimePulse" {
		t.Errorf("an installation that added nothing is called %q", got)
	}
}
