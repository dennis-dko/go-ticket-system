package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
)

// A passkey's name is held to the characters the form allows, and cut by
// characters where it is longer: the form counts characters, and a cut by bytes
// split a letter like ü in two and stored a name that was no longer text.
func TestAPasskeysNameIsCutByCharacters(t *testing.T) {
	limit := model.PasskeyMaxNameLength

	for name, want := range map[string]string{
		"":                                 model.DefaultPasskeyName,
		"   ":                              model.DefaultPasskeyName,
		"  Work laptop  ":                  "Work laptop",
		strings.Repeat("a", limit-1) + "ü": strings.Repeat("a", limit-1) + "ü",
		strings.Repeat("ü", limit+5):       strings.Repeat("ü", limit),
		strings.Repeat("a", limit) + "and beyond": strings.Repeat("a", limit),
	} {
		got := passkeyLabel(name)

		if got != want || !utf8.ValidString(got) {
			t.Errorf("a passkey called %q (%d characters) is listed as %q (%d characters, text: %v), want %q",
				name, utf8.RuneCountInString(name), got, utf8.RuneCountInString(got), utf8.ValidString(got), want)
		}
	}
}
