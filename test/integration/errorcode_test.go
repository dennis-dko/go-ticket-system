//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// A refusal has to say which rule was broken, not only say so in English.
//
// The server writes its messages where the rule is enforced, which is right for
// the log and wrong for the person who tripped over it: the interface showed them
// the rule's English whatever language they had chosen. The reason now travels as a code with the values the sentence
// interpolated, and the interface looks the sentence up in the reader's language.
//
// Proved on the wire rather than in the browser, because the wire is the contract:
// the code has to survive GoFr's error rendering, which is where a field that is
// not part of its own error types would be dropped.

// refusal is the error object as it reaches a client.
type refusal struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Values  []any  `json:"values"`
}

func refusalOf(t *testing.T, r response) refusal {
	t.Helper()

	var body struct {
		Error refusal `json:"error"`
	}

	if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
		t.Fatalf("the error body is not JSON: %v\n%s", err, r.Body)
	}

	return body.Error
}

// A conflict, which GoFr renders through a local error type.
func TestAConflictCarriesItsReasonAndValues(t *testing.T) {
	t.Parallel()

	_, admin, _ := startWithWorker(t)

	// A role's name is taken once, and the refusal names the name that is taken -
	// the value the German sentence needs.
	role := map[string]any{"name": "twice", "description": "made twice", "permissions": []string{"tickets:read"}}

	admin.must(admin.api(http.MethodPost, "/roles", role), http.StatusCreated, http.StatusOK)

	refused := admin.api(http.MethodPost, "/roles", role)
	if refused.Status != http.StatusConflict {
		t.Fatalf("a second role of the same name answered %d, want 409: %s",
			refused.Status, refused.Body)
	}

	reason := refusalOf(t, refused)

	if reason.Code != "roleNameTaken" {
		t.Errorf("the refusal is coded %q, want roleNameTaken", reason.Code)
	}

	if len(reason.Values) != 1 || reason.Values[0] != "twice" {
		t.Fatalf("the refusal carries %v, want the name that is taken", reason.Values)
	}

	// The English message stays: it is what the log records, and what a client
	// without the sentence falls back to.
	if !strings.Contains(reason.Message, "already exists") {
		t.Errorf("the message no longer explains itself: %q", reason.Message)
	}
}

// Invalid input, which is the other rendering path - and the one that used to
// wrap the sentence in GoFr's "'1' invalid parameter(s):".
func TestInvalidInputCarriesItsReasonAndValues(t *testing.T) {
	t.Parallel()

	_, admin, _ := startWithWorker(t)

	// A password shorter than the rule, which is refused with the length it asks for.
	refused := admin.api(http.MethodPost, "/users", map[string]any{
		"name": "Kurt", "email": "kurt@example.com", "role": "agent", "password": "short",
	})

	if refused.Status != http.StatusBadRequest {
		t.Fatalf("a short password answered %d, want 400: %s", refused.Status, refused.Body)
	}

	reason := refusalOf(t, refused)

	if reason.Code != "passwordTooShort" {
		t.Errorf("the refusal is coded %q, want passwordTooShort", reason.Code)
	}

	// One value, the length. JSON has one number type, so 12 arrives as 12.0.
	if len(reason.Values) != 1 {
		t.Fatalf("the refusal carries %d value(s), want the length: %v", len(reason.Values), reason.Values)
	}

	if length, ok := reason.Values[0].(float64); !ok || length < 1 {
		t.Errorf("the refusal asks for %v characters", reason.Values[0])
	}

	// The sentence itself, no longer behind a wrapper that counted parameters.
	if strings.Contains(reason.Message, "invalid parameter") {
		t.Errorf("the reason is still wrapped in a parameter count: %q", reason.Message)
	}

	if !strings.Contains(reason.Message, "at least") {
		t.Errorf("the message no longer explains itself: %q", reason.Message)
	}
}

// A rejected field says which field, and says so with a name beside it.
//
// This used to assert the opposite - that a field rejection carried no code at
// all - and it was right at the time: the names were the whole of what it said,
// and the sentence around them came from GoFr in English. That was the last
// refusal in the API with nothing to look a translation up by.
//
// Both halves are now checked, because both are needed and they fail
// independently: the code without the names gives a sentence that cannot say
// which field, and the names without the code give a list nobody can put a
// sentence around.
func TestAFieldRejectionIsNamedAndSaysWhichField(t *testing.T) {
	t.Parallel()

	_, admin, _ := startWithWorker(t)

	refused := admin.api(http.MethodPost, "/users", map[string]any{
		"name": "", "email": "nameless@example.com", "role": "agent", "password": "a-password-long-enough",
	})

	if refused.Status != http.StatusBadRequest {
		t.Fatalf("an account without a name answered %d, want 400: %s", refused.Status, refused.Body)
	}

	if reason := refusalOf(t, refused); reason.Code != apperror.CodeInvalidFields {
		t.Errorf("a field rejection is named %q, want %q",
			reason.Code, apperror.CodeInvalidFields)
	}

	var body struct {
		Error struct {
			Param []string `json:"param"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(refused.Body), &body); err != nil {
		t.Fatalf("the error body is not JSON: %v", err)
	}

	if len(body.Error.Param) == 0 {
		t.Errorf("the rejection no longer says which field: %s", refused.Body)
	}
}
