package spreadsheet

import (
	"strings"

	"github.com/dennis-dko/go-ticket-system/internal/support/i18n"
)

// translations is what a workbook calls things, in each language it is written
// in: the workbook.* entries of the dictionaries every part of the application
// reads, read once, because they are compiled in.
//
// The keys are the English words, which are also the fallback: a word nobody has
// translated is still written, in English, rather than left blank. A dictionary
// keys them the same way, behind workbook. - workbook.Users - because the words
// travel through the code as English data, a table's own headings, and are looked
// up as they arrive.
//
// Values are translated as well as headings - "yes" in a German export is no
// better than a heading left in English - and the importer accepts both
// spellings, which is what keeps a German export readable by the same
// application that wrote it.
var translations = workbookWords()

// workbookWords reads the workbook's words out of every dictionary there is.
func workbookWords() map[string]map[string]string {
	all := map[string]map[string]string{}

	for _, language := range i18n.Languages()[1:] {
		words := map[string]string{}

		for key, word := range i18n.Dictionary(language) {
			if english, ok := strings.CutPrefix(key, "workbook."); ok {
				words[english] = word
			}
		}

		all[language] = words
	}

	return all
}

// Translate gives the word in the language asked for, or the word itself.
//
// Exported because a preview shows the same values the file holds, and the preview
// is built by the service layer: the words a German export puts in its status
// column are the words the preview of that import has to show, or the reader is
// comparing two different vocabularies.
func Translate(language, text string) string { return translate(language, text) }

// translate gives the word in the language asked for, or the word itself.
func translate(language, text string) string {
	dictionary, ok := translations[normalise(language)]
	if !ok {
		return text
	}

	if translated, found := dictionary[text]; found {
		return translated
	}

	return text
}

// valueWords are the words that appear inside cells, as opposed to above them,
// named so that a test can hold every dictionary to them. They are not looked up
// on the way back in: a flag is read by ticked, which knows the spellings people
// type, and a reverse lookup through the dictionary would be ambiguous wherever
// two English words share one translation.
var valueWords = []string{"yes", "no"}

// yesNo is how a flag is written, in words rather than as TRUE/FALSE: a
// spreadsheet is read by a person, and Excel has its own opinions about what a
// boolean cell looks like in each locale.
func yesNo(on bool) string {
	if on {
		return "yes"
	}

	return "no"
}

// names are every name a sheet can carry, so the reader can find it whichever
// language wrote it.
func names(key string) []string {
	all := []string{key}

	for _, dictionary := range translations {
		if translated, ok := dictionary[key]; ok {
			all = append(all, translated)
		}
	}

	return all
}

// headingsIn translates a table's headings.
func headingsIn(language string, table Table) []string {
	out := make([]string, len(table.Headings))
	for i, heading := range table.Headings {
		out[i] = translate(language, heading)
	}

	return out
}

// normalise reduces "de-DE" and "DE" to "de", because that is the part that
// decides which words to use and the interface sends whatever the browser gave
// it.
func normalise(language string) string {
	lower := strings.ToLower(strings.TrimSpace(language))

	if cut := strings.IndexAny(lower, "-_"); cut > 0 {
		return lower[:cut]
	}

	return lower
}

// Languages are the languages a workbook can be written in, English aside.
//
// Exported so a test can check every one of them round-trips rather than only
// the one somebody remembered to write a case for.
func Languages() []string {
	out := make([]string, 0, len(translations))
	for language := range translations {
		out = append(out, language)
	}

	return out
}
