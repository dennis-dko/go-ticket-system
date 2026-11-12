package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The address put into a code for another device is the one the page shows: the
// Origin its browser sent, where that names the host the request reached - so a
// proxy in front that does not say which scheme the browser used cannot turn the
// page's https into http - and the address as reached otherwise.
func TestTheAddressACodeCarriesIsTheOneThePageShows(t *testing.T) {
	for _, tc := range []struct {
		name    string
		host    string
		headers map[string]string
		want    string
	}{
		{"the page's own origin", "tickets.example.com", map[string]string{"Origin": "https://tickets.example.com"},
			"https://tickets.example.com"},
		{"an origin naming another host", "tickets.example.com", map[string]string{"Origin": "https://elsewhere.example"},
			"http://tickets.example.com"},
		{"an origin of null", "tickets.example.com", map[string]string{"Origin": "null"}, "http://tickets.example.com"},
		{"no origin, behind a proxy that says https", "tickets.example.com",
			map[string]string{"X-Forwarded-Proto": "https"}, "https://tickets.example.com"},
		{"no origin, a port", "tickets.example.com:8000", nil, "http://tickets.example.com:8000"},
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", nil)
		request.Host = tc.host

		for name, value := range tc.headers {
			request.Header.Set(name, value)
		}

		if got := addressAsSeen(request); got != tc.want {
			t.Errorf("%s: the address is %q, want %q", tc.name, got, tc.want)
		}
	}
}
