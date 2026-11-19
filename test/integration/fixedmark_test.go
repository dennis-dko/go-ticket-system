//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/internal/support/imaging"
)

// aLogo is a one-pixel PNG: an uploaded logo that is plainly not TimePulse's mark.
const aLogo = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// The tab and the installed app show TimePulse's mark, whatever logo an
// installation has uploaded.
//
// An uploaded logo used to become the tab's icon and the installed app's: the
// document pointed the tab at an address that served the logo, and the manifest's
// icons were fitted from it. The maintainer decided the mark is the application's
// and stays: the logo belongs at the left of the bar once somebody has signed in,
// and nowhere else. So with a logo saved, the document still names favicon.svg,
// and the app's icon at the size a home screen asks for is the mark as
// imaging.Mark draws it, byte for byte.
func TestTheTabAndTheAppKeepTimePulsesMarkWhateverTheLogo(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	admin.must(admin.api(http.MethodPut, "/settings/branding", map[string]any{"title": "htp.net", "logo": aLogo}),
		http.StatusOK)

	page := string(admin.must(admin.do(http.MethodGet, "/", nil), http.StatusOK).Body)

	if !strings.Contains(page, `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`) ||
		strings.Contains(page, `href="/favicon?`) {
		at := strings.Index(page, `rel="icon"`)
		t.Errorf("with a logo saved, the tab is not pointed at TimePulse's mark: %.200s", page[max(0, at-20):])
	}

	var manifest struct {
		Icons []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
		} `json:"icons"`
	}

	if err := json.Unmarshal(admin.must(admin.do(http.MethodGet, "/manifest.webmanifest", nil), http.StatusOK).Body,
		&manifest); err != nil {
		t.Fatalf("the manifest does not parse: %v", err)
	}

	mark, err := imaging.MarkPNG(192)
	if err != nil {
		t.Fatalf("drawing the mark: %v", err)
	}

	served := false

	for _, icon := range manifest.Icons {
		if icon.Sizes != "192x192" {
			continue
		}

		served = true

		if body := admin.must(admin.do(http.MethodGet, icon.Src, nil), http.StatusOK).Body; !bytes.Equal(body, mark) {
			t.Errorf("with a logo saved, the installed app's icon at 192 is not TimePulse's mark (%d bytes)", len(body))
		}
	}

	if !served {
		t.Fatalf("the manifest names no icon at 192: %+v", manifest.Icons)
	}
}
