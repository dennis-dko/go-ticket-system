package web_test

import (
	"html"
	"regexp"
	"strings"
	"testing"
)

var (
	// fallbackCall is a t() call with its English written out, as one string or
	// as several joined with +, the shape every longer sentence in the script has.
	fallbackCall = regexp.MustCompile(`(?:^|[^A-Za-z0-9_$.])t\(\s*'([^']+)',\s*` +
		`('(?:[^'\\]|\\.)*'(?:\s*\+\s*'(?:[^'\\]|\\.)*')*)\s*[,)]`)
	fallbackPart = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)

	// markupSentence is an element of the markup whose text is its English and
	// whose data-i18n names its translation.
	markupSentence = regexp.MustCompile(`data-i18n="([^"]+)"[^>]*>([^<]*)<`)

	// namedThing is a name a sentence hands somebody to type or to look for: a
	// file of the deployment, a compose file, a docker command, an environment
	// variable or an address of the API. None of them is translated.
	namedThing = regexp.MustCompile(`deploy/[\w./-]+` +
		`|\bcompose(?:\.[\w-]+)*\.ya?ml\b` +
		`|\bdocker(?: compose)? [a-z]+(?: -[a-z])?` +
		`|\b[A-Z][A-Z0-9]*_[A-Z0-9_]+\b` +
		`|/api/v1/[\w/{}-]+` +
		`|\.well-known/[\w-]+`)
)

// A translation names every file, command and setting its English names.
//
// Such a name is the part of a sentence somebody acts on, and the one part no
// language changes. The card for an update in a container gained "or add
// deploy/compose.update.yaml to the deployment and this card will do it" in
// English when the updater arrived, and the German went on without it: an
// administrator reading German was told how to update by hand and never that
// the card could do it. TestEveryTranslationCoversTheCodeLookups could not
// notice, because the key was there; only what it said had fallen behind.
//
// The markup is read as well as the script: the cases the version card's
// command does not take are written there, and they are made of such names.
func TestATranslationNamesEveryFileCommandAndSettingItsEnglishDoes(t *testing.T) {
	type source struct{ key, english string }

	var fromScript, fromMarkup []source

	for _, call := range fallbackCall.FindAllStringSubmatch(withoutLineComments(asset(t, "/app.js")), -1) {
		var english strings.Builder
		for _, part := range fallbackPart.FindAllStringSubmatch(call[2], -1) {
			english.WriteString(part[1])
		}

		fromScript = append(fromScript, source{call[1], english.String()})
	}

	for _, element := range markupSentence.FindAllStringSubmatch(markupComment.ReplaceAllString(asset(t, "/"), ""), -1) {
		fromMarkup = append(fromMarkup, source{element[1], html.UnescapeString(element[2])})
	}

	// Each scan has to have read the hundreds of sentences there are, or one
	// that reads nothing would pass for one that found nothing wrong.
	if len(fromScript) < 100 || len(fromMarkup) < 100 {
		t.Fatalf("read %d sentences from the script and %d from the markup; the scan is not reading them",
			len(fromScript), len(fromMarkup))
	}

	languages := dictionaries(t)

	for _, s := range append(fromScript, fromMarkup...) {
		names := namedThing.FindAllString(s.english, -1)

		for language, words := range languages {
			sentence, ok := words[s.key]
			if !ok {
				continue // TestEveryTranslationCoversTheCodeLookups and ...TheMarkup say so.
			}

			for _, name := range names {
				if !strings.Contains(sentence, name) {
					t.Errorf("%s in %s does not name %s, which its English does:\n\t%s",
						s.key, language, name, sentence)
				}
			}
		}
	}
}
