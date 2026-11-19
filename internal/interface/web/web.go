package web

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
)

// Branding is what an installation calls itself, as far as the served document
// is concerned.
//
// A name and nothing else. The tab and the installed app show TimePulse's mark
// whatever logo an installation uploads - the logo is drawn at the left of the
// bar, by the page, once somebody has signed in.
type Branding struct {
	// Title names the browser tab and the installed app.
	Title string
}

// BrandingFunc answers what this installation is called right now.
type BrandingFunc func(context.Context) Branding

// assets holds the UI. all: is not needed here because no file starts with
// '_' or '.', but the pattern must keep matching every file added later.
//
//go:embed asset
var assets embed.FS

// apiPrefixes are the paths the UI must never shadow. Anything under them is
// passed to the next handler even if a file of that name happened to exist.
var apiPrefixes = []string{
	"/api/",
	"/.well-known/",
	"/health",
	"/alive",
	"/metrics",
	"/swagger",
}

// /favicon.ico used to be in that list, which meant the one path browsers ask
// for by name fell through to the API and answered 404.
//
// The markup carries an icon, so a browser that reads the page was fine - but a
// bookmark, a restored tab, a feed reader and a pinned shortcut all ask for the
// file directly and got nothing. There is a real one in the assets now, so
// removing the entry is all it takes: it is served like any other file, and no
// API route was ever registered under that name for it to shadow.

// Handler returns middleware that serves the embedded UI.
//
// GoFr's AddStaticFiles only serves a directory from disk, which would defeat
// the single-binary goal, so the assets are served here instead. The signature
// matches gofr.dev/pkg/gofr/http.Middleware.
//
// The branding function is what makes the served document say the instance's own
// name. Optional: nil serves the assets exactly as they are embedded, which is
// what the tests that only care about caching want.
//
// It has to be the server that does this. The title is configured, so it lives in
// the database, so the page cannot know it until it has asked - and asking
// finishes after the document has been parsed and shown. Patching it afterwards
// from script is what this used to do, and it leaves a reload showing a name
// nobody chose for as long as the request takes.
// There is no arrangement of client-side code that fixes that, because the
// problem is the order of two things and one of them is a round trip.
func Handler(branding BrandingFunc) func(http.Handler) http.Handler {
	sub, err := fs.Sub(assets, "asset")
	if err != nil {
		// Only reachable if the embed directive and this path disagree,
		// which is a build-time mistake rather than a runtime condition.
		panic("web: embedded assets missing: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))
	tags := buildETags(sub)
	document := newDocument(sub)
	install := newInstallable()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !servesUI(r) {
				next.ServeHTTP(w, r)

				return
			}

			// What makes the page installable, before the single-page fallback
			// below, which rewrites anything that is not a file to "/": neither of
			// these is a file, and both would otherwise be answered with the page.
			if path(r) == manifestPath {
				var current Branding
				if branding != nil {
					current = branding(r.Context())
				}

				install.serveManifest(w, r, current)

				return
			}

			if strings.HasPrefix(path(r), appIconPath) {
				install.serveAppIcon(w, r)

				return
			}

			// A language's dictionary, from the ones the binary carries rather than
			// from these assets: the installer and the Android app read the same
			// files. Before the fallback too, because it is not a file here.
			if file, ok := strings.CutPrefix(path(r), dictionaryPath); ok {
				serveDictionary(w, r, file)

				return
			}

			// /api-docs is a second page, not part of the single-page app, so
			// it is resolved to its own file rather than the SPA fallback.
			if p := path(r); p == "/api-docs" || p == "/api-docs/" {
				r = r.Clone(r.Context())
				r.URL.Path = "/api-docs.html"
			}

			// The UI is a single page: unknown paths must render the app
			// rather than 404, so deep links work on reload.
			if _, statErr := fs.Stat(sub, strings.TrimPrefix(path(r), "/")); statErr != nil {
				r = r.Clone(r.Context())
				r.URL.Path = "/"
			}

			// The document, with the title written into it before it is sent.
			if branding != nil && isDocument(path(r)) {
				document.serve(w, r, branding(r.Context()))

				return
			}

			if servedFromCache(w, r, tags) {
				return
			}

			fileServer.ServeHTTP(w, r)
		})
	}
}

// etags is the content hash of every embedded asset, by request path.
//
// Built once at start-up, which is possible precisely because the assets are
// compiled in: they cannot change while the process runs, so there is nothing to
// invalidate and nothing to stat per request.
type etags map[string]string

// buildETags hashes every embedded file.
func buildETags(sub fs.FS) etags {
	tags := etags{}

	_ = fs.WalkDir(sub, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // an asset that cannot be read is simply not tagged
		}

		content, readErr := fs.ReadFile(sub, name)
		if readErr != nil {
			return nil //nolint:nilerr // same: no tag rather than no start-up
		}

		sum := sha256.Sum256(content)
		tags["/"+name] = `"` + hex.EncodeToString(sum[:16]) + `"`

		return nil
	})

	// The single-page fallback answers "/" with index.html, so it needs the tag
	// under the path it is actually requested by.
	if tag, ok := tags["/index.html"]; ok {
		tags["/"] = tag
	}

	return tags
}

// servedFromCache answers a repeat request with 304 where the browser already has
// the file, and reports whether it did.
//
// Every asset used to be sent in full on every page load. http.FileServer would
// have handled this by itself, but it works from the modification time, and a file
// compiled into the binary has none - embed reports the zero time, so no
// Last-Modified and no ETag were ever emitted and nothing could be revalidated.
//
// no-cache rather than a long max-age, and that is the honest choice here: the
// asset URLs are not fingerprinted - /app.js is /app.js in every release - so a
// browser told to hold one for a year would still be running last release's
// interface against this release's API, with no way to tell it otherwise. no-cache
// does not mean "do not cache": it means "ask first", and asking is one request
// that ends in 304 with no body.
func servedFromCache(w http.ResponseWriter, r *http.Request, tags etags) bool {
	tag, known := tags[path(r)]
	if !known {
		return false
	}

	w.Header().Set("ETag", tag)
	w.Header().Set("Cache-Control", "no-cache")

	// Any of the tags the browser offers, which is what the header allows even
	// though this only ever sends one.
	for offered := range strings.SplitSeq(r.Header.Get("If-None-Match"), ",") {
		if strings.TrimSpace(offered) == tag {
			w.WriteHeader(http.StatusNotModified)

			return true
		}
	}

	return false
}

// servesUI reports whether the request should be answered from the embedded
// assets rather than passed down to the API routes.
func servesUI(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	p := path(r)
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}

	return true
}

func path(r *http.Request) string {
	p := r.URL.Path
	if p == "" {
		return "/"
	}

	return p
}
