//go:build browser

package browser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

// TestABrowserLeavesNoProfileBehind closes a browser while something still
// holds a file in its profile, lets go once the browser is gone, and asks
// whether the profile went with it.
//
// A full run produces that order by itself on Windows: a helper process of
// Chrome's still writing to the profile when chromedp removed it, which chromedp
// does once, ten milliseconds after the browser's own process has exited. One
// green run of the suite left seventeen profiles behind, mostly empty
// directories and the odd cache journal. The order is forced here rather than
// waited for, because a race proves nothing on a day it is not lost.
//
// On Linux an open file does not stop a removal, so this passes there either
// way; as with everything test/tempdir is for, only a Windows run shows it.
func TestABrowserLeavesNoProfileBehind(t *testing.T) {
	t.Parallel()

	var profile string

	t.Run("a browser", func(t *testing.T) {
		alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), launch(t, "en-US")...)
		defer cancelAlloc()

		ctx, cancel := chromedp.NewContext(alloc)
		defer cancel()

		ctx, cancelTimeout := context.WithTimeout(ctx, interactionTimeout)
		defer cancelTimeout()

		dir, err := profileDirectoryOf(ctx)
		if err != nil {
			t.Fatal(err)
		}

		profile = dir

		// Held the way Chrome's helper held it: open past the moment the browser
		// is closed.
		held, err := os.Create(filepath.Join(dir, "held"))
		if err != nil {
			t.Fatal(err)
		}

		// Closed the way a case's cleanups close it, the allocator last: it
		// returns once the browser has exited and chromedp has had its one try
		// at whatever profile it made.
		cancelTimeout()
		cancel()
		cancelAlloc()

		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if profile == "" {
		return
	}

	if _, err := os.Stat(profile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the profile outlived its browser: %s still holds %q", profile, leftIn(profile))
	}
}

// profileDirectoryOf asks the browser behind ctx where it keeps its profile.
//
// Asked of the browser rather than worked out from the options, so the answer is
// the same whoever chose the directory - chromedp or this suite.
func profileDirectoryOf(ctx context.Context) (string, error) {
	var arguments []string

	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		arguments, err = cdpbrowser.GetBrowserCommandLine().Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Browser))

		return err
	}))
	if err != nil {
		return "", fmt.Errorf("ask the browser for its command line: %w", err)
	}

	for _, argument := range arguments {
		if dir, ok := strings.CutPrefix(argument, "--user-data-dir="); ok {
			return dir, nil
		}
	}

	return "", fmt.Errorf("the browser names no profile directory among %q", arguments)
}

// leftIn lists the files still under dir, for a failure to name.
func leftIn(dir string) []string {
	var left []string

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() {
			name, _ := filepath.Rel(dir, path)
			left = append(left, name)
		}

		return nil
	})
	if err != nil {
		left = append(left, err.Error())
	}

	return left
}
