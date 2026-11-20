//go:build browser

package browser

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/timepulse/test/harness"
)

// The installer asks what a new database starts with as one choice - empty, the
// demo data or a backup - and lets anybody go from any answer to any other. It
// was two tick boxes, and ticking the backup switched the demo off: going from
// the backup to the demo meant unticking it first, and nothing on the screen
// said that the two excluded each other.
func TestTheInstallerOffersWhatADatabaseStartsWithAsOneChoice(t *testing.T) {
	t.Parallel()

	app := harness.StartUnconfigured(t)

	ctx, done := installerInGerman(t, app)
	defer done()

	type choice struct {
		Chosen []string `json:"chosen"`
		Kinds  []string `json:"kinds"`
		Fields bool     `json:"fields"`
	}

	read := func() choice {
		var c choice

		if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const boxes = [...document.querySelectorAll('#empty, #demo, #restore')];

			return {
				chosen: boxes.filter((box) => box.checked).map((box) => box.id),
				kinds: boxes.map((box) => box.type + ':' + box.name),
				fields: !document.querySelector('#restore-fields').hidden,
			};
		})()`, &c)); err != nil {
			t.Fatalf("reading the choice: %v", err)
		}

		return c
	}

	if c := read(); !slices.Equal(c.Chosen, []string{"empty"}) ||
		!slices.Equal(c.Kinds, []string{"radio:start", "radio:start", "radio:start"}) || c.Fields {
		t.Errorf("a new installer offers %+v, want empty, the demo and a backup as one choice, empty chosen", c)
	}

	for _, step := range []struct {
		press  string
		fields bool
	}{
		{"restore", true},
		{"demo", false},
		{"restore", true},
		{"empty", false},
	} {
		if err := chromedp.Run(ctx, chromedp.Click("#"+step.press, chromedp.ByQuery)); err != nil {
			t.Fatalf("choosing %s: %v", step.press, err)
		}

		if c := read(); !slices.Equal(c.Chosen, []string{step.press}) || c.Fields != step.fields {
			t.Errorf("after choosing %s the installer has %+v, want only that chosen and the backup's "+
				"fields shown %v", step.press, c, step.fields)
		}
	}

	// The page with the backup chosen, kept for a person to look at - once the
	// fields' colours have finished moving to the new answer.
	var shot []byte

	if err := chromedp.Run(ctx,
		chromedp.Click("#restore", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.FullScreenshot(&shot, 100),
	); err == nil {
		_ = os.WriteFile("installerchoice.png", shot, 0o600)
	}
}
