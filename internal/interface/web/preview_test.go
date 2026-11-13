package web_test

import (
	"strings"
	"testing"
)

// A preview that describes a chosen file is forgotten when the file changes.
//
// Previews are drawn through redrawable, so a language change re-renders them
// without asking the server again, and stopRedrawing is how a card says the
// answer it was holding is gone. The card's reset says so when it is emptied.
//
// Choosing a different file makes that equally true, and the change handler
// once did not say so. The draw stayed registered against the previous result,
// and the next language change put that verdict back on screen above a file
// nobody had checked - with the Import button back beside it if the verdict was
// clean, which would then upload the new file unseen.
//
// Only this key needs it. Every other redrawable draws a card that is refilled
// rather than emptied; a preview describes a file somebody chose, which is the
// only content that stops being true while the card is still on screen - so
// this checks the card that holds one rather than demanding a stop for every
// key.
func TestAFileCardForgetsThePreviewItNoLongerDescribes(t *testing.T) {
	js := asset(t, "/app.js")

	const (
		what    = "the card buildSheetCard makes for accounts and roles"
		marker  = "file.addEventListener('change'"
		forgets = "stopRedrawing(`sheetPreview:${spec.key}`)"
	)

	at := strings.Index(js, marker)
	if at < 0 {
		t.Fatalf("app.js no longer contains %q, so %s is not being checked", marker, what)
	}

	// To the end of the handler, which is the first "});" after it - the body
	// contains no nested one.
	handler := js[at:]
	if end := strings.Index(handler, "});"); end > 0 {
		handler = handler[:end]
	}

	if !strings.Contains(handler, forgets) {
		t.Errorf("%s hides its preview when another file is chosen without calling "+
			"%s, so the draw stays registered against the previous file and the "+
			"next language change puts that verdict back on screen",
			what, forgets)
	}
}
