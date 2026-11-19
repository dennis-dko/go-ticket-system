package oidctest_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/dennis-dko/timepulse/test/oidctest"
)

// The provider sends the browser back only to an address on this machine.
//
// A real provider sends it back only to an address registered for the client,
// and refuses any other: that is what keeps its sign-in from being a way to send
// somebody's browser, with a code in hand, wherever a link says. This one
// served every address it was given, which CodeQL reported as an open redirect.
// Every application it serves runs on this machine, so this machine is its
// registration.
func TestTheProviderSendsTheBrowserBackOnlyToThisMachine(t *testing.T) {
	t.Parallel()

	p := oidctest.Start(t)

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	ask := func(back string) *http.Response {
		t.Helper()

		query := url.Values{
			"client_id":             {oidctest.ClientID},
			"response_type":         {"code"},
			"code_challenge_method": {"S256"},
			"code_challenge":        {"challenge"},
			"state":                 {"state"},
			"redirect_uri":          {back},
		}

		resp, err := client.Get(p.URL + "/authorize?" + query.Encode()) //nolint:noctx // a test asking once
		if err != nil {
			t.Fatalf("ask the provider to send the browser back to %s: %v", back, err)
		}

		t.Cleanup(func() { _ = resp.Body.Close() })

		return resp
	}

	for _, here := range []string{
		"http://127.0.0.1:8000/sign-in/openid",
		"http://localhost:8000/sign-in/openid",
		"https://[::1]:8443/sign-in/openid",
	} {
		if resp := ask(here); resp.StatusCode != http.StatusFound {
			t.Errorf("the provider answered %d for %s, an address on this machine", resp.StatusCode, here)
		}
	}

	for _, elsewhere := range []string{
		"https://elsewhere.example/sign-in/openid",
		"http://127.0.0.1.elsewhere.example/sign-in/openid",
		"http://localhost.elsewhere.example/sign-in/openid",
		"//elsewhere.example/sign-in/openid",
		"javascript:alert(1)",
	} {
		resp := ask(elsewhere)
		if resp.StatusCode == http.StatusFound {
			t.Errorf("the provider sent the browser to %s, which is not on this machine", resp.Header.Get("Location"))
		}
	}
}
