package web_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/internal/support/i18n"
)

// The Backups card says what a restore takes, and it said "the same SECRET_KEY"
// for as long as that was true - and on after the installer learned to take the
// key a backup was made under, or to restore it without the secrets that key
// seals. Somebody whose old key was lost read that their backups were no use. So
// the sentence says the installer asks for that key, and that a restore can go
// on without those secrets, in both languages.
func TestTheBackupCardSaysARestoreCanTakeTheKeyItWasMadeUnder(t *testing.T) {
	declared := regexp.MustCompile(`data-i18n="backup\.hint"[^>]*>([^<]*)<`).FindStringSubmatch(asset(t, "/"))
	if declared == nil {
		t.Fatal("the Backups card says nothing under backup.hint")
	}

	for _, language := range []struct{ code, sentence, asks, without, same string }{
		{"en", declared[1], "the installer asks for", "without the secrets", "the same SECRET_KEY"},
		{"de", i18n.Dictionary("de")["backup.hint"], "nach dem der Installer fragt", "ohne die Geheimnisse", "demselben SECRET_KEY"},
	} {
		if !strings.Contains(language.sentence, language.asks) || !strings.Contains(language.sentence, language.without) {
			t.Errorf("%s: the card does not say the installer asks for the backup's key, or that a restore can "+
				"leave its secrets behind: %q", language.code, language.sentence)
		}

		if strings.Contains(language.sentence, language.same) {
			t.Errorf("%s: the card still says a restore needs the same key: %q", language.code, language.sentence)
		}
	}
}
