//go:build firefox

package firefox

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// What Firefox decided about a page's icon, read from its own store.
//
// What a tab draws is not in the document and not in the response; it is what
// the browser decided, and the browser is the only one who knows. Asking what the
// document declares, or whether the address answers with an image, was true
// through two versions of a bug where the tab stayed empty. Firefox writes the
// decision down: every icon it adopts goes into favicons.sqlite in its profile,
// so the question can be asked directly rather than inferred.

// adoptionPatience is how long Firefox is given to fetch an icon and write it to
// its store.
//
// Twenty seconds, until a run took 37 and failed - "Firefox recorded no icon for
// this page" - on a machine that was busy with another suite at the time. Nothing
// was wrong with the application; the number was the outlier. Everything else
// here waits 45 or 60 seconds, and the browser suite next door says why: these
// were written against a quiet machine running one case at a time.
//
// It costs nothing when the icon arrives, which is the ordinary case at well
// under a second.
const adoptionPatience = 45 * time.Second

// waitForAdoptedIcon polls Firefox's own store until it has recorded an icon
// whose address holds href, and returns what it recorded.
func waitForAdoptedIcon(t *testing.T, b *browser, href string) string {
	t.Helper()

	// The address without its origin, which is what the store's rows end with.
	wanted := strings.TrimPrefix(href, "/")

	deadline := time.Now().Add(adoptionPatience)

	for {
		for _, row := range recordedIcons(t, b.profile) {
			if strings.Contains(row, wanted) {
				return row
			}
		}

		if time.Now().After(deadline) {
			return ""
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// recordedIcons reads what this Firefox has decided about icons.
//
// Copied before reading: Firefox holds its store open, and the write-ahead log
// beside it holds anything recent - so the main file alone is a view from some
// minutes ago.
func recordedIcons(t *testing.T, profile string) []string {
	t.Helper()

	source := filepath.Join(profile, "favicons.sqlite")
	if _, err := os.Stat(source); err != nil {
		return nil
	}

	copied := filepath.Join(t.TempDir(), fmt.Sprintf("favicons-%d.sqlite", time.Now().UnixNano()))

	for _, suffix := range []string{"", "-wal", "-shm"} {
		body, err := os.ReadFile(source + suffix)
		if err != nil {
			continue
		}

		if err := os.WriteFile(copied+suffix, body, 0o600); err != nil {
			t.Fatalf("copying Firefox's icon store: %v", err)
		}
	}

	db, err := sql.Open("sqlite", copied)
	if err != nil {
		t.Fatalf("opening Firefox's icon store: %v", err)
	}

	defer func() { _ = db.Close() }()

	rows, err := db.Query(`SELECT icon_url, width, length(data) FROM moz_icons ORDER BY id`)
	if err != nil {
		// The store exists but has no table yet, which is simply "nothing decided".
		return nil
	}

	defer func() { _ = rows.Close() }()

	var out []string

	for rows.Next() {
		var (
			url    string
			width  int
			length int
		)

		if err := rows.Scan(&url, &width, &length); err != nil {
			t.Fatalf("reading Firefox's icon store: %v", err)
		}

		// An icon with no bytes is one Firefox looked at and did not keep.
		if length == 0 {
			continue
		}

		out = append(out, fmt.Sprintf("%s (width %d, %d bytes)", url, width, length))
	}

	return out
}
