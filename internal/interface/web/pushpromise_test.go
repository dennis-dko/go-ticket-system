package web_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/support/i18n"
)

// The notification card says what pushing to this browser does, and it promised the
// reminders "even while it is closed". No push reaches a browser on a computer
// that is closed: the push service holds the reminder and hands it over when the
// browser is started again, which is when they arrived on the computer this was
// first tried on, and somebody who believed the card waited for them in vain. So
// the sentence says the browser has to be running, in both languages, and
// promises a closed one nothing.
func TestThePushSentenceSaysTheBrowserHasToBeRunning(t *testing.T) {
	declared := regexp.MustCompile(`data-i18n="push\.pushed"[^>]*>([^<]*)<`).FindStringSubmatch(asset(t, "/"))
	if declared == nil {
		t.Fatal("the notification card says nothing under push.pushed")
	}

	for _, language := range []struct{ code, sentence, running, closed string }{
		{"en", declared[1], "has to be running", "while it is closed"},
		{"de", i18n.Dictionary("de")["push.pushed"], "muss der Browser dafür laufen", "wenn er geschlossen ist"},
	} {
		if !strings.Contains(language.sentence, language.running) {
			t.Errorf("%s: the card does not say the browser has to be running to be pushed to: %q", language.code, language.sentence)
		}

		if strings.Contains(language.sentence, language.closed) {
			t.Errorf("%s: the card promises the reminders to a browser that is closed: %q", language.code, language.sentence)
		}
	}
}
