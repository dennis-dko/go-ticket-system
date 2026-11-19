package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/dennis-dko/timepulse/internal/interface/web"
	"github.com/dennis-dko/timepulse/internal/support/imaging"
)

// The installed app: what a telephone's home screen or a desktop's start menu is
// handed to show this application as one of its own - a name, icons at the sizes
// those places ask for, and the screens a long press leads to.

type manifest struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ShortName       string `json:"short_name"`
	Lang            string `json:"lang"`
	StartURL        string `json:"start_url"`
	Scope           string `json:"scope"`
	Display         string `json:"display"`
	ThemeColor      string `json:"theme_color"`
	BackgroundColor string `json:"background_color"`
	Icons           []struct {
		Src   string `json:"src"`
		Sizes string `json:"sizes"`
		Type  string `json:"type"`
	} `json:"icons"`
	Shortcuts []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"shortcuts"`
}

func fetchAsking(t *testing.T, server http.Handler, path, language string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.Header.Set("Accept-Language", language)

	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	return res
}

func readManifest(t *testing.T, server http.Handler, language string) manifest {
	t.Helper()

	res := fetchAsking(t, server, "/manifest.webmanifest", language)

	if res.Code != http.StatusOK {
		t.Fatalf("the manifest answered %d", res.Code)
	}

	if got := res.Header().Get("Content-Type"); got != "application/manifest+json" {
		t.Errorf("the manifest is served as %q, want application/manifest+json", got)
	}

	var m manifest
	if err := json.Unmarshal(res.Body.Bytes(), &m); err != nil {
		t.Fatalf("the manifest is not JSON: %v\n%s", err, res.Body.String())
	}

	return m
}

// The installed app is called what the installation calls itself, opens on the
// page, and stands on its own rather than inside a browser frame. Under its icon,
// where a dozen characters fit, it is called TimePulse.
func TestTheManifestNamesTheInstallation(t *testing.T) {
	m := readManifest(t, brandedServer(web.Branding{Title: "TimePulse | htp.net"}), "en")

	if m.Name != "TimePulse | htp.net" || m.ShortName != "TimePulse" {
		t.Errorf("the app is called %q (%q), want the installation's own name and TimePulse", m.Name, m.ShortName)
	}

	if m.StartURL != "/" || m.Scope != "/" || m.ID != "/" {
		t.Errorf("the app starts at %q in scope %q with id %q, want the page", m.StartURL, m.Scope, m.ID)
	}

	if m.Display != "standalone" {
		t.Errorf("the app is displayed %q, want standalone", m.Display)
	}

	sizes := map[string]string{}

	for _, icon := range m.Icons {
		if icon.Type != "image/png" {
			t.Errorf("an icon is %q, want PNG", icon.Type)
		}

		sizes[icon.Sizes] = icon.Src
	}

	for _, want := range []string{"192x192", "512x512"} {
		src, ok := sizes[want]
		if !ok {
			t.Errorf("the manifest has no %s icon; the home screen and the install dialog ask for it", want)

			continue
		}

		if src != "/app-icon/"+strings.Split(want, "x")[0] {
			t.Errorf("the %s icon is at %q, want the address the size is drawn at", want, src)
		}
	}
}

// An installation without a name of its own is the application's name.
func TestAnUnnamedInstallationIsTheApplicationsName(t *testing.T) {
	if m := readManifest(t, brandedServer(web.Branding{}), "en"); m.Name != "TimePulse" {
		t.Errorf("an unnamed installation is called %q, want TimePulse", m.Name)
	}
}

// What a long press offers is in the language the device asks for, and the
// German is the interface's own: read from the dictionary the page uses rather
// than written down a second time.
func TestTheManifestSpeaksTheDevicesLanguage(t *testing.T) {
	server := brandedServer(web.Branding{})

	german := readManifest(t, server, "de-DE,de;q=0.9,en;q=0.8")
	english := readManifest(t, server, "en-GB,en;q=0.9")

	names := func(m manifest) string {
		var all []string
		for _, s := range m.Shortcuts {
			all = append(all, s.Name)
		}

		return strings.Join(all, ", ")
	}

	if !strings.Contains(names(german), "Zeiteinträge") || german.Lang != "de" {
		t.Errorf("a German device is offered %q (lang %q)", names(german), german.Lang)
	}

	if !strings.Contains(names(english), "Time entries") || english.Lang != "en" {
		t.Errorf("an English device is offered %q (lang %q)", names(english), english.Lang)
	}

	for _, s := range english.Shortcuts {
		if !strings.HasPrefix(s.URL, "/#") {
			t.Errorf("the shortcut %q leads to %q, want a screen of the page", s.Name, s.URL)
		}
	}
}

// The icons are TimePulse's mark, drawn at the sizes they are announced at - a
// 192 scaled up to 512 by the device is the blur this exists to avoid.
func TestTheAppIconsAreDrawnAtTheirSizes(t *testing.T) {
	server := brandedServer(web.Branding{})
	m := readManifest(t, server, "en")

	for _, icon := range m.Icons {
		res := fetch(t, server, icon.Src)

		if res.Code != http.StatusOK {
			t.Fatalf("the %s icon answered %d", icon.Sizes, res.Code)
		}

		img, err := png.Decode(bytes.NewReader(res.Body.Bytes()))
		if err != nil {
			t.Fatalf("the %s icon is not a PNG: %v", icon.Sizes, err)
		}

		bounds := img.Bounds()
		if got := strconv.Itoa(bounds.Dx()) + "x" + strconv.Itoa(bounds.Dy()); got != icon.Sizes {
			t.Errorf("the icon announced as %s is %s", icon.Sizes, got)
		}

		mark, err := imaging.MarkPNG(bounds.Dx())
		if err != nil {
			t.Fatalf("drawing the mark at %d: %v", bounds.Dx(), err)
		}

		if !bytes.Equal(res.Body.Bytes(), mark) {
			t.Errorf("the %s icon is not the mark as imaging draws it", icon.Sizes)
		}
	}
}

// An icon is asked about again rather than kept for good.
//
// Its address carries no version, so a year of caching would keep a device on
// the mark it first saw through every release that draws it differently.
func TestAnAppIconIsRevalidatedRatherThanKept(t *testing.T) {
	server := brandedServer(web.Branding{})

	first := fetch(t, server, "/app-icon/192")

	if got := first.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("the icon is served with Cache-Control %q, want no-cache", got)
	}

	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("the icon carries no ETag, so asking again costs the whole picture")
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/app-icon/192", nil)
	req.Header.Set("If-None-Match", tag)

	again := httptest.NewRecorder()
	server.ServeHTTP(again, req)

	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("asking again with the tag answered %d and %d bytes, want 304 and none",
			again.Code, again.Body.Len())
	}
}

// Only the sizes the manifest announces are drawn: an address taking any number
// would draw a picture of any size somebody asks for.
func TestAnAppIconOfAnotherSizeIsNotServed(t *testing.T) {
	for _, path := range []string{"/app-icon/100", "/app-icon/99999", "/app-icon/x", "/app-icon/"} {
		if res := fetch(t, brandedServer(web.Branding{}), path); res.Code != http.StatusNotFound {
			t.Errorf("%s answered %d, want 404", path, res.Code)
		}
	}
}

// The document points at the manifest, and gives the devices that read no
// manifest an icon of their own.
func TestTheDocumentLinksTheManifestAndATouchIcon(t *testing.T) {
	body := head(fetch(t, brandedServer(web.Branding{Title: "x"}), "/").Body.String())

	if !strings.Contains(body, `<link rel="manifest" href="/manifest.webmanifest">`) {
		t.Errorf("the document does not link the manifest: %.400s", body)
	}

	if !strings.Contains(body, `<link rel="apple-touch-icon" href="/app-icon/192">`) {
		t.Errorf("the document has no touch icon: %.400s", body)
	}
}
