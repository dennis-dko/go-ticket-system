package model_test

import (
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
)

// What an installation calls itself, in the language that is reading.
func TestTheInstanceIsNamedInTheLanguageThatAsked(t *testing.T) {
	t.Parallel()

	branding := model.Branding{
		Title: "Ticket System",
		Translations: map[string]model.BrandingText{
			"de": {Title: "Ticketsystem"},
			"en": {Title: "Ticket System"},
		},
	}

	if got := branding.TitleIn("de"); got != "Ticketsystem" {
		t.Errorf("a German reader is shown %q", got)
	}

	// A language with no name of its own falls back to the untranslated one
	// rather than to nothing.
	if got := branding.TitleIn("fr"); got != "Ticket System" {
		t.Errorf("an untranslated language is shown %q", got)
	}

	// And a translation that exists but is empty is not a name.
	empty := model.Branding{
		Title:        "Ticket System",
		Translations: map[string]model.BrandingText{"de": {Title: "   "}},
	}

	if got := empty.TitleIn("de"); got != "Ticket System" {
		t.Errorf("a blank translation was used as a name: %q", got)
	}
}
