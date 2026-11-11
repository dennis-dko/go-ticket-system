package web_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/go-time-recording/internal/support/security"
)

var (
	markupAttribute = regexp.MustCompile(`([a-zA-Z-]+)(?:="([^"]*)")?`)
	fieldTag        = map[string]bool{"input": true, "select": true, "textarea": true}

	// notTypedInto are the inputs nobody types into: what they hold is ticked,
	// chosen as a file, or pressed.
	notTypedInto = map[string]bool{
		"hidden": true, "checkbox": true, "radio": true, "submit": true, "button": true,
		"reset": true, "file": true, "image": true, "color": true,
	}
)

// outsideAForm are the controls that sit in no form, each with why that is
// right. A control in no form is held to no rule before what it holds is used,
// which is how the second factor's code went to the server unchecked - so one
// that submits what somebody typed belongs in a form, and the rest are named.
var outsideAForm = map[string]string{
	"theme-picker":       "applies the appearance as it is chosen; nothing is submitted",
	"language-picker":    "applies the language as it is chosen",
	"viewing-for-picker": "chooses whose time the screens show, as it is chosen",
	"timer-project":      "the stopwatch's project, chosen from what may be booked",
	"timer-description":  "the stopwatch's optional description, whose one rule - its length - holds while typing",
	"filter-ts-project":  "filters the entries as it is chosen",
	"week-add-project":   "adds a row to the week as it is chosen",
	"statistics-from":    "the statistics' range, asked for as it is changed",
	"statistics-to":      "the statistics' range, asked for as it is changed",
	"log-search":         "searches the log as it is typed",
	"log-delay":          "how often the log is asked again, read within its bounds by logDelaySeconds",
}

// markupFields is every control a person types into or chooses from, by its id or
// else its name, with its attributes and whether a form holds it.
func markupFields(t *testing.T) []markupField {
	t.Helper()

	var (
		fields []markupField
		depth  int
	)

	for _, tag := range markupTag.FindAllStringSubmatch(markupComment.ReplaceAllString(asset(t, "/"), ""), -1) {
		closing, name := tag[1] == "/", strings.ToLower(tag[2])

		if name == "form" {
			if closing {
				depth--
			} else {
				depth++
			}

			continue
		}

		if closing || !fieldTag[name] {
			continue
		}

		attributes := map[string]string{}
		for _, attribute := range markupAttribute.FindAllStringSubmatch(tag[3], -1) {
			attributes[strings.ToLower(attribute[1])] = attribute[2]
		}

		if name == "input" && notTypedInto[strings.ToLower(attributes["type"])] {
			continue
		}

		key := attributes["id"]
		if key == "" {
			key = attributes["name"]
		}

		fields = append(fields, markupField{key: key, attributes: attributes, inForm: depth > 0})
	}

	if len(fields) < 100 {
		t.Fatalf("read %d controls from the page; the reading has stopped finding them", len(fields))
	}

	return fields
}

type markupField struct {
	key        string
	attributes map[string]string
	inForm     bool
}

// Everything a person types and submits is held to its rules by the browser
// before anything is sent, which a form's submission does and a button beside a
// loose field does not. The second factor's card was such a field, and five
// digits went to the server to be called a wrong code.
func TestEveryControlOutsideAFormIsOneThatNeedsNone(t *testing.T) {
	seen := map[string]bool{}

	for _, field := range markupFields(t) {
		if field.inForm {
			continue
		}

		seen[field.key] = true

		if _, named := outsideAForm[field.key]; !named {
			t.Errorf("%s sits in no form, so nothing it holds is checked before it is used: put it in one, "+
				"or name it in outsideAForm with why it needs none", field.key)
		}
	}

	for key := range outsideAForm {
		if !seen[key] {
			t.Errorf("outsideAForm names %s, which is no longer a control outside a form", key)
		}
	}
}

// Every pattern a field is held to says in the screen's language what it asks
// for: the browser can only call it "the requested format", in its own language.
func TestEveryPatternSaysWhatItAsksFor(t *testing.T) {
	block := regexp.MustCompile(`(?s)const PATTERN_MESSAGES = \{(.*?)\n\};`).FindStringSubmatch(asset(t, "/app.js"))
	if block == nil {
		t.Fatal("app.js has no PATTERN_MESSAGES, so no pattern a field is held to is put into words")
	}

	worded := map[string]bool{}
	for _, entry := range regexp.MustCompile(`'([^']+)':\s*\(\)\s*=>\s*t\(`).FindAllStringSubmatch(block[1], -1) {
		worded[entry[1]] = true
	}

	patterns := 0

	for _, field := range markupFields(t) {
		pattern, held := field.attributes["pattern"]
		if !held {
			continue
		}

		patterns++

		if !worded[pattern] {
			t.Errorf("%s is held to the pattern %s, which PATTERN_MESSAGES does not put into words", field.key, pattern)
		}
	}

	if patterns == 0 {
		t.Fatal("no field on the page is held to a pattern; the reading has stopped finding them")
	}
}

// A second factor's code is held to the server's rule wherever it is typed: as
// many digits as the server checks for, nothing else, and within a form - at the
// sign-in, and on the card that switches the second factor on and off.
func TestEverySecondFactorCodeIsHeldToTheServersRule(t *testing.T) {
	secret, err := security.NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}

	code, err := security.TOTPCodeAt(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"pattern":   fmt.Sprintf("[0-9]{%d}", len(code)),
		"maxlength": fmt.Sprint(len(code)),
		"inputmode": "numeric",
	}

	found := 0

	for _, field := range markupFields(t) {
		if field.attributes["autocomplete"] != "one-time-code" {
			continue
		}

		found++

		if !field.inForm {
			t.Errorf("the code field %s sits in no form, so a code is sent unchecked", field.key)
		}

		for attribute, value := range want {
			if field.attributes[attribute] != value {
				t.Errorf("the code field %s has %s %q, want %q", field.key, attribute, field.attributes[attribute], value)
			}
		}
	}

	if found < 2 {
		t.Errorf("found %d fields for a second factor's code, want the sign-in's and the card's", found)
	}
}
