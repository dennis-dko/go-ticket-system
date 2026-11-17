//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// A token's sign-in answers with the session under the name it had before the
// project was called TimePulse as well as under today's, carrying the same
// session. An Android app from before the rename opens its session by looking for
// exactly that name in the answer, and one that does not find it tells its owner
// the installation is not theirs - so without it every phone stops working the
// day its installation is updated.
func TestATokensSignInStillHandsOutTheCookieOlderAppsLookFor(t *testing.T) {
	t.Parallel()

	a, _, worker := startWithWorker(t)
	_, secret := madeToken(t, worker, "phone")

	answer := tokenSignIn(t, a.BaseURL(), secret)
	if answer.Status != http.StatusCreated {
		t.Fatalf("the token's sign-in answered %d %s", answer.Status, answer.Body)
	}

	today := cookieOf(answer, "timepulse_session")
	before := cookieOf(answer, "gtr_session")

	if today == "" || before != today {
		t.Errorf("the token's sign-in set timepulse_session=%q and gtr_session=%q, want one session under both",
			today, before)
	}
}

// A browser that signed in before the update holds its session under the old
// name alone, and is still signed in by it.
func TestASessionCookieFromBeforeTheRenameStillSignsIn(t *testing.T) {
	t.Parallel()

	a, _, worker := startWithWorker(t)
	_, secret := madeToken(t, worker, "phone")

	session := cookieOf(tokenSignIn(t, a.BaseURL(), secret), "timepulse_session")
	if session == "" {
		t.Fatal("the token's sign-in set no session cookie")
	}

	if status := readingMeWith(t, a.BaseURL(), "gtr_session", session); status != http.StatusOK {
		t.Errorf("the session under the name from before the rename was answered %d", status)
	}
}

// cookieOf is the value of the cookie an answer set under name, empty where it
// set none.
func cookieOf(answer response, name string) string {
	for _, cookie := range (&http.Response{Header: answer.Header}).Cookies() {
		if cookie.Name == name && cookie.Value != "" {
			return cookie.Value
		}
	}

	return ""
}

// readingMeWith is how the installation answers a session presented under name
// and nothing else.
func readingMeWith(t *testing.T, baseURL, name, session string) int {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/api/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.AddCookie(&http.Cookie{Name: name, Value: session})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_ = resp.Body.Close()

	return resp.StatusCode
}
