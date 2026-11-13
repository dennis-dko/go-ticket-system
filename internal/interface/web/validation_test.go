package web_test

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/support/security"
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
	"theme-picker":    "applies the appearance as it is chosen; nothing is submitted",
	"language-picker": "applies the language as it is chosen",
	"log-search":      "searches the log as it is typed",
	"log-delay":       "how often the log is asked again, read within its bounds by logDelaySeconds",
}

// markupFields is every control a person types into or chooses from, by its id or
// else its name, with its attributes and whether a form holds it.
func markupFields(t *testing.T) []markupField {
	t.Helper()

	var (
		fields []markupField
		forms  []string
	)

	for _, tag := range markupTag.FindAllStringSubmatch(markupComment.ReplaceAllString(asset(t, "/"), ""), -1) {
		closing, name := tag[1] == "/", strings.ToLower(tag[2])

		if name == "form" {
			if closing && len(forms) > 0 {
				forms = forms[:len(forms)-1]
			} else if !closing {
				id := "(a form with no id)"
				if match := markupID.FindStringSubmatch(tag[3]); match != nil {
					id = match[1]
				}

				forms = append(forms, id)
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

		field := markupField{key: key, attributes: attributes, inForm: len(forms) > 0}
		if field.inForm {
			field.form = forms[len(forms)-1]
		}

		fields = append(fields, field)
	}

	if len(fields) < 60 {
		t.Fatalf("read %d controls from the page; the reading has stopped finding them", len(fields))
	}

	return fields
}

type markupField struct {
	key        string
	attributes map[string]string
	inForm     bool

	// form is the id of the form holding the field, which tells apart two fields
	// of the same name on two cards.
	form string
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

// numberRule is what the server takes in a number field: its bounds, empty where
// there is none, and the step - "any" where it takes any value between them, "1"
// for a whole number.
type numberRule struct{ min, max, step string }

// figure writes a bound the way the markup writes it.
func figure(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

// numberRules are every number field on the screen, by its form and name, with
// the server's rule for it. A field the markup holds to more than this refuses a
// value the server takes - an idle time of 7 minutes, refused by steps of five
// while the server took it - and one held to less lets somebody type what the
// server refuses.
var numberRules = map[string]numberRule{
	"form-operational/sessionLifetimeHours": {
		figure(model.MinSessionLifetimeHours), figure(model.MaxSessionLifetimeHours), "any",
	},
	"form-operational/rateLimit":              {figure(model.MinRateLimit), "", "1"},
	"form-operational/rateLimitWindowSeconds": {"1", figure(model.MaxRateWindowSeconds), "1"},
	"form-operational/ldapSyncMaxDeleteRatio": {"0", "1", "any"},
	"form-telemetry/tracerRatio":              {"0", "1", "any"},
	"form-ldap/port":                          {"1", figure(model.MaxPort), "1"},
	"form-datasource/port":                    {"1", figure(model.MaxPort), "1"},

	// Zero switches the timeout off; between it and the floor the server refuses
	// by the field's name, a rule of two parts the markup cannot state.
	"form-operational/sessionIdleMinutes": {"0", figure(model.MaxSessionIdleMinutes), "1"},

	// A bound the server keeps unexported, beside the code that checks it: the
	// days a token may last, in apitoken_application_service.go.
	"form-token/expiresInDays": {"0", "3650", "1"},

	// Not the server's: how often the log viewer asks again, which the page reads
	// within these bounds itself.
	"log-delay": {"0", "300", "1"},
}

// Every number field takes what the server takes, no more and no less, and a
// number field is not added without saying what that is.
func TestEveryNumberFieldTakesWhatTheServerTakes(t *testing.T) {
	seen := map[string]bool{}

	for _, field := range markupFields(t) {
		if field.attributes["type"] != "number" {
			continue
		}

		key := field.key
		if field.inForm {
			key = field.form + "/" + field.attributes["name"]
		}

		seen[key] = true

		rule, named := numberRules[key]
		if !named {
			t.Errorf("%s is a number field numberRules does not name, so nothing says what it takes", key)

			continue
		}

		// A number field with no step is held to whole numbers.
		step := cmp.Or(field.attributes["step"], "1")

		if field.attributes["min"] != rule.min || field.attributes["max"] != rule.max || step != rule.step {
			t.Errorf("%s takes min %q, max %q in steps of %q; the server takes min %q, max %q in steps of %q",
				key, field.attributes["min"], field.attributes["max"], step, rule.min, rule.max, rule.step)
		}
	}

	for key := range numberRules {
		if !seen[key] {
			t.Errorf("numberRules names %s, which is no longer a number field", key)
		}
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
