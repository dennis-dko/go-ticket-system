//go:build browser

package browser

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/timepulse/test/harness"
)

// The backup file is chosen on a field of the installer's own rather than with
// the browser's button, whose words are the browser's - "Durchsuchen…, Keine
// Datei ausgewählt." - and in the browser's language, whatever the page speaks.
// The field says what to do in the page's words, names the file once one is
// chosen, with its size, and takes a file dropped on it as if it had been
// chosen.
func TestTheInstallerSaysWhichBackupFileWasChosen(t *testing.T) {
	t.Parallel()

	app := harness.StartUnconfigured(t)

	ctx, done := installerInGerman(t, app)
	defer done()

	path := filepath.Join(t.TempDir(), "timepulse-20261120-001922.tpbak")
	if err := os.WriteFile(path, bytes.Repeat([]byte{1}, 3290), 0o600); err != nil {
		t.Fatalf("writing a file to choose: %v", err)
	}

	field := `(document.querySelector('#restore-drop')?.textContent ?? '').replace(/\s+/g, ' ').trim()`

	var before, chosen, dropped string

	if err := chromedp.Run(ctx,
		chromedp.Click("#restore", chromedp.ByQuery),
		chromedp.Evaluate(field, &before),
		chromedp.SetUploadFiles("#restore-file", []string{path}, chromedp.ByQuery),
		chromedp.Evaluate(field, &chosen),
		chromedp.Evaluate(`(() => {
			const data = new DataTransfer();
			data.items.add(new File([new Uint8Array(2 * 1024 * 1024)], 'dropped.tpbak'));
			document.querySelector('#restore-drop')?.dispatchEvent(
				new DragEvent('drop', { dataTransfer: data, bubbles: true, cancelable: true }));

			const file = document.querySelector('#restore-file').files[0];
			return (file ? file.name + ' | ' : '') +
				(document.querySelector('#restore-drop')?.textContent ?? '').replace(/\s+/g, ' ').trim();
		})()`, &dropped),
	); err != nil {
		t.Fatalf("choosing a backup file: %v", err)
	}

	if !strings.Contains(before, "Sicherungsdatei wählen") || strings.Contains(before, "timepulse-") {
		t.Errorf("before a file is chosen the field says %q, want what to do, in German", before)
	}

	if !strings.Contains(chosen, "timepulse-20261120-001922.tpbak") || !strings.Contains(chosen, "3 kB") {
		t.Errorf("with a file chosen the field says %q, want its name and 3 kB", chosen)
	}

	if !strings.HasPrefix(dropped, "dropped.tpbak | ") || !strings.Contains(dropped, "2 MB") {
		t.Errorf("after a file was dropped on the field it reads %q, want the dropped file chosen "+
			"and named with 2 MB", dropped)
	}
}
