package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/internal/interface/web"
)

// The page arrives already carrying the instance's own name.
//
// It used to arrive with "TimePulse" and a script that corrected it once
// /branding had answered, so every reload showed a name nobody chose for as long
// as that request took. Reported twice, and both times the fix was on the wrong
// side of the round trip: there is no arrangement of client code that wins a race
// against a request it has to make first.
func TestTheDocumentCarriesTheConfiguredTitle(t *testing.T) {
	server := brandedServer(web.Branding{Title: "Zeiterfassung htp"})

	res := fetch(t, server, "/")

	if !strings.Contains(res.Body.String(), "<title>Zeiterfassung htp</title>") {
		t.Errorf("the page does not name the instance: %.200s", head(res.Body.String()))
	}

	if strings.Contains(res.Body.String(), "<title>TimePulse</title>") {
		t.Error("the shipped title is still in the document somebody is sent")
	}
}

// A title is text, and it lands between two tags.
//
// The one place in this application where configured text becomes part of the
// document rather than the text of a node - everything else goes through
// textContent in the browser, where this cannot arise. So it is the one place
// that has to escape, and it is written by whoever holds settings:manage.
func TestAConfiguredTitleCannotCloseItsOwnTag(t *testing.T) {
	server := brandedServer(web.Branding{
		Title: `</title><script>window.owned=1</script>`,
	})

	body := fetch(t, server, "/").Body.String()

	if strings.Contains(body, "<script>window.owned") {
		t.Error("a configured title escaped its tag and became markup")
	}

	if !strings.Contains(body, "&lt;/title&gt;") {
		t.Errorf("the title was not escaped: %.200s", head(body))
	}
}

// Nothing is left with no title at all.
func TestAnEmptyTitleFallsBackToTheShippedOne(t *testing.T) {
	server := brandedServer(web.Branding{Title: "   "})

	if !strings.Contains(fetch(t, server, "/").Body.String(), "<title>TimePulse</title>") {
		t.Error("an instance that has named itself nothing has no name at all")
	}
}

// The page declares TimePulse's mark as it ships, whatever the installation is
// called.
//
// These are index.html's own two links, served as written. The SVG and the .ico
// are the same mark, so which of them an engine takes does not change the
// picture - and nothing configured stands in for either.
func TestTheDocumentDeclaresTheShippedMark(t *testing.T) {
	body := head(fetch(t, brandedServer(web.Branding{Title: "x"}), "/").Body.String())

	for _, link := range []string{
		`<link rel="icon" type="image/svg+xml" href="/favicon.svg">`,
		`<link rel="alternate icon" href="/favicon.ico">`,
	} {
		if !strings.Contains(body, link) {
			t.Errorf("the document does not declare %s: %.400s", link, body)
		}
	}

	if got := strings.Count(body, `rel="icon"`); got != 1 || strings.Contains(body, "data:image/") {
		t.Errorf("the document declares an icon besides the shipped mark: %.400s", body)
	}
}

// fetch performs one request against a handler.
func fetch(t *testing.T, server http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	return res
}

func head(body string) string {
	if at := strings.Index(body, "<body"); at > 0 {
		return body[:at]
	}

	return body
}
