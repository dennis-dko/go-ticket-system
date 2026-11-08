//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// signedInAs is the account a sign-in's answer names.
func signedInAs(t *testing.T, answer response) string {
	t.Helper()

	var envelope struct {
		Data struct {
			User *struct {
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
	}

	if err := json.Unmarshal(answer.Body, &envelope); err != nil || envelope.Data.User == nil {
		t.Fatalf("the sign-in answered %d %s", answer.Status, answer.Body)
	}

	return envelope.Data.User.Email
}

// sessionCookieOf is the session cookie an answer set, empty where it set none.
func sessionCookieOf(answer response) string {
	for _, cookie := range (&http.Response{Header: answer.Header}).Cookies() {
		if cookie.Name == "gtr_session" && cookie.Value != "" {
			return cookie.Value
		}
	}

	return ""
}

// sessionExpiryOf is when the session cookie an answer set runs out.
func sessionExpiryOf(t *testing.T, answer response) time.Time {
	t.Helper()

	for _, cookie := range (&http.Response{Header: answer.Header}).Cookies() {
		if cookie.Name == "gtr_session" && cookie.Value != "" {
			return cookie.Expires
		}
	}

	t.Fatalf("the answer set no session cookie: %v", answer.Header.Values("Set-Cookie"))

	return time.Time{}
}

// madeToken is a personal token its owner made, with its id and secret.
func madeToken(t *testing.T, owner *client, name string) (uint, string) {
	t.Helper()

	var token struct {
		ID     uint   `json:"id"`
		Secret string `json:"secret"`
	}

	owner.must(owner.api(http.MethodPost, "/me/tokens", map[string]any{"name": name}),
		http.StatusCreated, http.StatusOK).Data(t, &token)

	return token.ID, token.Secret
}

// readingMe is how the installation answers a session cookie alone.
func readingMe(t *testing.T, baseURL, session string) int {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/api/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.AddCookie(&http.Cookie{Name: "gtr_session", Value: session})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_ = resp.Body.Close()

	return resp.StatusCode
}

// A personal token is exchanged for a session of its owner, which is how the
// Android app signs in to the web interface it shows: the cookie alone then
// reads what the owner may.
func TestATokenSignsItsOwnerInToTheWebInterface(t *testing.T) {
	t.Parallel()

	a, _, worker := startWithWorker(t)
	_, secret := madeToken(t, worker, "phone")

	answer := asTheClients(t, a.BaseURL(), secret, http.MethodPost, "/auth/token", nil)
	if answer.Status != http.StatusCreated || signedInAs(t, answer) != "wera@example.com" {
		t.Fatalf("the token's sign-in answered %d %s", answer.Status, answer.Body)
	}

	session := sessionCookieOf(answer)
	if session == "" {
		t.Fatalf("the token's sign-in set no session cookie: %v", answer.Header.Values("Set-Cookie"))
	}

	if status := readingMe(t, a.BaseURL(), session); status != http.StatusOK {
		t.Errorf("the session the token opened was answered %d", status)
	}
}

// Taking a token back ends the session it was exchanged for, so the phone it was
// scanned into is cut off at once - as it was while the app sent the token with
// every request - rather than for as long as the session would have lasted. Only
// its owner can take it back, and nothing else of theirs ends with it.
func TestTakingATokenBackEndsTheSessionItOpened(t *testing.T) {
	t.Parallel()

	a, admin, worker := startWithWorker(t)
	id, secret := madeToken(t, worker, "phone")

	phone := sessionCookieOf(asTheClients(t, a.BaseURL(), secret, http.MethodPost, "/auth/token", nil))
	if phone == "" {
		t.Fatal("the token opened no session to take back")
	}

	admin.must(admin.api(http.MethodDelete, fmt.Sprintf("/me/tokens/%d", id), nil), http.StatusNotFound)

	if status := readingMe(t, a.BaseURL(), phone); status != http.StatusOK {
		t.Fatalf("after somebody else tried to take the token back, its session was answered %d", status)
	}

	worker.must(worker.api(http.MethodDelete, fmt.Sprintf("/me/tokens/%d", id), nil), http.StatusOK, http.StatusNoContent)

	if status := readingMe(t, a.BaseURL(), phone); status != http.StatusUnauthorized {
		t.Errorf("after the token was taken back, the session it opened was answered %d", status)
	}

	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK)
}

// A session a token opened lasts no longer than the token does, however long an
// installation lets a sign-in last.
func TestASessionATokenOpenedEndsNoLaterThanTheToken(t *testing.T) {
	t.Parallel()

	a, admin, worker := startWithWorker(t)
	admin.must(admin.api(http.MethodPut, "/settings/operational", map[string]any{"sessionLifetimeHours": 72}),
		http.StatusOK)

	var token struct {
		Secret    string     `json:"secret"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}

	worker.must(worker.api(http.MethodPost, "/me/tokens", map[string]any{"name": "phone", "expiresInDays": 1}),
		http.StatusCreated, http.StatusOK).Data(t, &token)

	if token.ExpiresAt == nil {
		t.Fatal("a token made for a day carries no expiry")
	}

	// A minute's slack, for a database that keeps a moment to the second: what is
	// being caught is a session outliving its token by two days.
	expires := sessionExpiryOf(t, asTheClients(t, a.BaseURL(), token.Secret, http.MethodPost, "/auth/token", nil))
	if expires.After(token.ExpiresAt.Add(time.Minute)) {
		t.Errorf("the session runs until %s, the token only until %s", expires, token.ExpiresAt)
	}
}

// Only the token decides who is signed in: a session cookie of somebody else on
// the same request is not asked, so a stray cookie cannot turn another person's
// token into its own session.
func TestATokensSignInIsTheTokensOwnerWhateverCookieComesWithIt(t *testing.T) {
	t.Parallel()

	a, admin, worker := startWithWorker(t)
	_, secret := madeToken(t, worker, "phone")

	base, err := url.Parse(a.BaseURL())
	if err != nil {
		t.Fatal(err)
	}

	var adminSession string

	for _, cookie := range admin.http.Jar.Cookies(base) {
		if cookie.Name == "gtr_session" {
			adminSession = cookie.Value
		}
	}

	if adminSession == "" {
		t.Fatal("the administrator's client holds no session cookie to send")
	}

	answer := asTheClients(t, a.BaseURL(), secret, http.MethodPost, "/auth/token", nil)
	if signedInAs(t, answer) != "wera@example.com" {
		t.Fatalf("the token signed in somebody else: %s", answer.Body)
	}

	withCookie := admin.withHeader("Authorization", "Bearer "+secret).api(http.MethodPost, "/auth/token", nil)
	if withCookie.Status != http.StatusCreated || signedInAs(t, withCookie) != "wera@example.com" {
		t.Errorf("with the administrator's cookie beside it, the token signed in %s", withCookie.Body)
	}
}

// What is not a usable token opens no session: one the installation does not
// know and one taken back are refused as a wrong password is, and one whose
// owner has to replace a password an administrator gave them says so. A request
// with no token at all is a POST like any other and meets the CSRF check first.
func TestWhatIsNoUsableTokenOpensNoSession(t *testing.T) {
	t.Parallel()

	a, admin, worker := startWithWorker(t)
	id, secret := madeToken(t, worker, "phone")
	_, other := madeToken(t, worker, "laptop")

	for _, tc := range []struct {
		name, token string
		status      int
	}{
		{"no token at all", "", http.StatusForbidden},
		{"a token nobody made", "gtr_" + "0123456789abcdef0123456789abcdef", http.StatusUnauthorized},
	} {
		answer := asTheClients(t, a.BaseURL(), tc.token, http.MethodPost, "/auth/token", nil)
		if answer.Status != tc.status || sessionCookieOf(answer) != "" {
			t.Errorf("%s was answered %d %s, want %d and no session", tc.name, answer.Status, answer.Body, tc.status)
		}
	}

	worker.must(worker.api(http.MethodDelete, fmt.Sprintf("/me/tokens/%d", id), nil), http.StatusOK, http.StatusNoContent)

	if answer := asTheClients(t, a.BaseURL(), secret, http.MethodPost, "/auth/token", nil); answer.Status != http.StatusUnauthorized ||
		sessionCookieOf(answer) != "" {
		t.Errorf("a token taken back was answered %d %s", answer.Status, answer.Body)
	}

	var me struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}

	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)
	admin.must(admin.api(http.MethodPut, fmt.Sprintf("/users/%d/password", me.User.ID),
		map[string]any{"password": "a-chosen-reset-password"}), http.StatusOK)

	answer := asTheClients(t, a.BaseURL(), other, http.MethodPost, "/auth/token", nil)
	if answer.Status == http.StatusCreated || sessionCookieOf(answer) != "" {
		t.Errorf("a token whose owner was given a password to replace opened a session: %d %s", answer.Status, answer.Body)
	}
}

// During maintenance a token opens a session only for an account that may end
// it, as every other sign-in does.
func TestATokensSignInKeepsToMaintenance(t *testing.T) {
	t.Parallel()

	a, admin, worker := startWithWorker(t)
	_, workers := madeToken(t, worker, "phone")
	_, admins := madeToken(t, admin, "phone")

	setMaintenance(t, admin, true, "")

	if answer := asTheClients(t, a.BaseURL(), workers, http.MethodPost, "/auth/token", nil); answer.Status == http.StatusCreated ||
		sessionCookieOf(answer) != "" {
		t.Errorf("during maintenance the worker's token opened a session: %d %s", answer.Status, answer.Body)
	}

	if answer := asTheClients(t, a.BaseURL(), admins, http.MethodPost, "/auth/token", nil); answer.Status != http.StatusCreated ||
		sessionCookieOf(answer) == "" {
		t.Errorf("during maintenance the administrator's token was answered %d %s", answer.Status, answer.Body)
	}
}
