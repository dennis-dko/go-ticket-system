package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/i18n"
)

// isDocument reports whether this request is for the single page itself rather
// than for one of the files it pulls in.
//
// The single-page fallback has already rewritten anything unknown to "/", so by
// the time this is asked the only paths that mean the document are these two.
func isDocument(p string) bool {
	return p == "/" || p == "/index.html"
}

// document is index.html taken apart once, so a request only has to put it back
// together with this installation's own name in it.
type document struct {
	// The two pieces around the title. Empty when the markup did not contain what
	// was expected, in which case the page is not served at all - a missing title
	// is a worse failure than a generic one.
	before, after string
	usable        bool

	// languages is the meta tags naming the languages the page may speak, written
	// after the title. Fixed for the life of the process, like the dictionaries
	// it is made from.
	languages string
}

// titleMarkup is the exact string replaced. Kept as a constant beside the markup
// it matches, because a change to index.html that misses this file should fail
// loudly here rather than quietly serve a document with nothing substituted.
const titleMarkup = "<title>TimePulse</title>"

// newDocument splits index.html around the title.
//
// The assets are compiled in, so this is decided once at start-up and is the
// same for every request.
func newDocument(sub fs.FS) document {
	body, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return document{}
	}

	markup := string(body)

	titleAt := strings.Index(markup, titleMarkup)
	if titleAt < 0 {
		return document{}
	}

	return document{
		before:    markup[:titleAt],
		after:     markup[titleAt+len(titleMarkup):],
		usable:    true,
		languages: languagesMeta(i18n.Languages(), i18n.Name),
	}
}

// serve writes the page with this installation's name already in it.
func (d document) serve(w http.ResponseWriter, r *http.Request, branding Branding) {
	if !d.usable {
		http.Error(w, "the interface could not be assembled", http.StatusInternalServerError)

		return
	}

	title := strings.TrimSpace(branding.Title)
	if title == "" {
		title = model.Brand
	}

	page := d.before + "<title>" + escape(title) + "</title>\n  " + d.languages + d.after

	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")

	// Revalidated rather than cached: the document now depends on a setting, so
	// yesterday's copy can be wrong in a way the file's own hash cannot express.
	header.Set("Cache-Control", "no-cache")

	tag := `"` + fingerprint(page) + `"`
	header.Set("ETag", tag)

	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, tag) {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	_, _ = w.Write([]byte(page))
}

// escape makes a string safe to put between two tags.
//
// The title is written by an administrator and rendered into markup, which is the
// one place in this application where configured text becomes part of the
// document rather than the text of a node. Everything else goes through
// textContent in the browser, where this cannot arise.
func escape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	).Replace(s)
}

// fingerprint is a short stable digest, used for cache validators rather than
// for anything that needs to resist an adversary.
func fingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))

	return hex.EncodeToString(sum[:8])
}
