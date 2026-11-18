//go:build integration

package integration

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Every installation is called TimePulse, and what an administrator writes under
// Settings is added after a bar. The rule is the model's; this is the wiring in
// main that hands it to the document the server writes and to an authenticator
// app's entry. Started under an APP_NAME of its own, which named the installation
// in both places until the name was fixed.
func TestTheInstallationIsCalledTimePulseWhateverItsProcessIsCalled(t *testing.T) {
	t.Parallel()

	a := start(t, "APP_NAME=Zeiterfassung")
	admin := a.signInAsAdmin("a-much-better-password")

	if got := servedTitle(t, admin); got != "TimePulse" {
		t.Errorf("an installation that added nothing serves the title %q, want TimePulse", got)
	}

	admin.must(admin.api(http.MethodPut, "/settings/branding", map[string]any{"title": "htp.net"}),
		http.StatusOK)

	if got := servedTitle(t, admin); got != "TimePulse | htp.net" {
		t.Errorf("the served title is %q, want TimePulse | htp.net", got)
	}

	enrolled, err := url.Parse(beginEnrolment(t, admin).URI)
	if err != nil {
		t.Fatalf("the enrolment URI does not parse: %v", err)
	}

	if issuer := enrolled.Query().Get("issuer"); issuer != "TimePulse | htp.net" {
		t.Errorf("an authenticator app is told the installation is called %q", issuer)
	}
}

// servedTitle is the title of the document the server writes, before any script
// has run.
func servedTitle(t *testing.T, c *client) string {
	t.Helper()

	body := string(c.must(c.do(http.MethodGet, "/", nil), http.StatusOK).Body)

	at := strings.Index(body, "<title>")
	end := strings.Index(body, "</title>")

	if at < 0 || end < at {
		t.Fatalf("the document has no title: %.200s", body)
	}

	return html.UnescapeString(body[at+len("<title>") : end])
}
