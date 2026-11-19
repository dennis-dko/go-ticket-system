package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/i18n"
	"github.com/dennis-dko/timepulse/internal/support/imaging"
)

// manifestPath is where a browser reads what the installed app is called and
// which icons it has.
const manifestPath = "/manifest.webmanifest"

// appIconPath is where the installed app's icons are served, followed by the size.
const appIconPath = "/app-icon/"

// appIconSizes are the squares an installed app is given: 192 for the home screen
// and 512 for the install dialog and the splash screen. Only these are drawn - an
// address taking any number would draw a picture of any size anybody asked for.
var appIconSizes = []int{192, 512}

// The colours the device paints around the app before the page has: the
// interface's own ground and accent, so starting it does not flash white into a
// dark room or blue into nothing.
const (
	appBackground = "#f5f6f8"
	appTheme      = "#2f6feb"
)

// appShortcut is one screen a long press on the installed app leads to.
//
// Named by the interface's own words: the key is the page's, the English is its
// fallback, and every other language's name is read from the dictionaries the
// page reads rather than written down here a second time. The manifest is
// fetched by the device, not by the page, so it cannot go through t() - but it
// can use what t() uses.
type appShortcut struct {
	key, english, view string
}

var appShortcuts = []appShortcut{
	{key: "nav.timesheets", english: "Time entries", view: "timesheets"},
	{key: "nav.calendar", english: "Calendar", view: "calendar"},
	{key: "nav.overtime", english: "Overtime", view: "overtime"},
}

// installable serves what makes the page installable as an app.
//
// Both halves are read or drawn once, because neither can change while the
// process runs: the dictionaries are compiled in, and so is the mark.
type installable struct {
	// words names the shortcuts, by language.
	words map[string]i18n.Translator

	// icons is the mark at each of appIconSizes. The installed app shows
	// TimePulse's mark whatever logo an installation uploads, so there is one
	// picture per size for the life of the process.
	icons map[int][]byte
}

// newInstallable reads the shortcuts' names out of the dictionaries the page
// reads, so the two cannot come to disagree, and draws the icons.
func newInstallable() *installable {
	words := map[string]i18n.Translator{}

	for _, language := range i18n.Languages() {
		words[language] = i18n.For(language)
	}

	icons := map[int][]byte{}

	for _, size := range appIconSizes {
		// A size that cannot be drawn is left out, and asking for it is then
		// answered 404 - which cannot happen with the mark compiled in, and would
		// say so rather than serve an empty picture if it ever did.
		if drawn, err := imaging.MarkPNG(size); err == nil {
			icons[size] = drawn
		}
	}

	return &installable{words: words, icons: icons}
}

// appManifest is the Web App Manifest, the fields a device reads of it.
type appManifest struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	ShortName       string           `json:"short_name"`
	Lang            string           `json:"lang"`
	StartURL        string           `json:"start_url"`
	Scope           string           `json:"scope"`
	Display         string           `json:"display"`
	BackgroundColor string           `json:"background_color"`
	ThemeColor      string           `json:"theme_color"`
	Icons           []manifestIcon   `json:"icons"`
	Shortcuts       []manifestTarget `json:"shortcuts"`
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

type manifestTarget struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// serveManifest writes the manifest for this installation, in the language the
// device asked for.
//
// Built per request rather than shipped as a file, because the name is the
// installation's own and the shortcuts are named in the language asked for.
func (s *installable) serveManifest(w http.ResponseWriter, r *http.Request, branding Branding) {
	name := strings.TrimSpace(branding.Title)
	if name == "" {
		name = model.Brand
	}

	lang := preferredLanguage(r.Header.Get("Accept-Language"))

	manifest := appManifest{
		ID:   "/",
		Name: name,
		// What a home screen writes under the icon, where a dozen characters fit:
		// the name every installation shares, while what this one adds goes in
		// the full name above.
		ShortName:       model.Brand,
		Lang:            lang,
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: appBackground,
		ThemeColor:      appTheme,
	}

	for _, size := range appIconSizes {
		side := strconv.Itoa(size)

		manifest.Icons = append(manifest.Icons, manifestIcon{
			Src:     appIconPath + side,
			Sizes:   side + "x" + side,
			Type:    "image/png",
			Purpose: "any",
		})
	}

	for _, shortcut := range appShortcuts {
		manifest.Shortcuts = append(manifest.Shortcuts, manifestTarget{
			Name: s.words[lang].Text(shortcut.key, shortcut.english),
			URL:  "/#" + shortcut.view,
		})
	}

	body, err := json.Marshal(manifest)
	if err != nil {
		http.Error(w, "the manifest could not be written", http.StatusInternalServerError)

		return
	}

	header := w.Header()
	header.Set("Content-Type", "application/manifest+json")
	// Revalidated, like the document: what it says depends on settings and on the
	// language asked for, which no address here encodes.
	header.Set("Cache-Control", "no-cache")
	header.Set("Vary", "Accept-Language")

	tag := `"` + fingerprint(string(body)) + `"`
	header.Set("ETag", tag)

	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, tag) {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	_, _ = w.Write(body)
}

// serveAppIcon writes one of the installed app's icons.
func (s *installable) serveAppIcon(w http.ResponseWriter, r *http.Request) {
	size, ok := appIconSize(strings.TrimPrefix(path(r), appIconPath))
	if !ok {
		http.NotFound(w, r)

		return
	}

	body := s.icons[size]
	if len(body) == 0 {
		http.NotFound(w, r)

		return
	}

	header := w.Header()
	header.Set("Content-Type", "image/png")

	// Revalidated like every other asset rather than cached for good: the address
	// carries no version, and the next release may draw the mark differently.
	header.Set("Cache-Control", "no-cache")

	tag := `"` + fingerprint(string(body)) + `"`
	header.Set("ETag", tag)

	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, tag) {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	_, _ = w.Write(body)
}

// appIconSize reads the size from the address, and only the announced ones.
func appIconSize(raw string) (int, bool) {
	size, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}

	for _, announced := range appIconSizes {
		if size == announced {
			return size, true
		}
	}

	return 0, false
}

// preferredLanguage is the language a device asking with this Accept-Language
// would rather read, of the languages the interface speaks.
//
// The highest weight wins, and the order breaks a tie, which is what the header
// means. A device asking for none of them gets English, the language the
// interface is written in.
func preferredLanguage(header string) string {
	best, bestWeight := i18n.English, -1.0
	spoken := i18n.Languages()

	for part := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")

		if !slices.Contains(spoken, language) {
			continue
		}

		weight := 1.0

		if q, found := strings.CutPrefix(strings.TrimSpace(params), "q="); found {
			if parsed, err := strconv.ParseFloat(q, 64); err == nil {
				weight = parsed
			}
		}

		if weight > bestWeight {
			best, bestWeight = language, weight
		}
	}

	return best
}
