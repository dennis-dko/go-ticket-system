//go:build browser

package browser

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/backup"
	"github.com/dennis-dko/timepulse/internal/support/security"
	"github.com/dennis-dko/timepulse/test/harness"
)

// backupSealedWith writes a backup whose installation had key, holding only that
// installation's key check, and answers where it lies.
func backupSealedWith(t *testing.T, key, passphrase string) string {
	t.Helper()

	sealer, err := security.NewSealer(key)
	if err != nil {
		t.Fatalf("build a sealer: %v", err)
	}

	check, err := sealer.Seal("timepulse")
	if err != nil {
		t.Fatalf("seal the key check: %v", err)
	}

	file, err := backup.Seal(passphrase, func(w io.Writer) error {
		encoder := json.NewEncoder(w)

		for _, line := range []any{
			sqldb.BackupHeader{Format: "timepulse-backup", Version: 1, Application: "TimePulse",
				Dialect: "sqlite", Schema: 20260730120000, CreatedAt: time.Now().UTC()},
			map[string]any{"table": "settings", "columns": []string{"key_name", "value", "updated_at"},
				"kinds": []string{"text", "text", "time"}},
			[]any{model.SettingSecretKeyCheck, check, "2026-11-20T00:00:00Z"},
			map[string]any{"end": true, "rows": 1},
		} {
			if err := encoder.Encode(line); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("seal the backup: %v", err)
	}

	path := filepath.Join(t.TempDir(), "timepulse-20261120-001922.tpbak")
	if err := os.WriteFile(path, file, 0o600); err != nil {
		t.Fatalf("write the backup: %v", err)
	}

	return path
}

// A backup sealed with a SECRET_KEY this installation does not have is answered
// on the installer's page with what the installer asks for: the key that
// backup's installation had, or the backup without its secrets. The part is out
// of sight until the backup has been read, the key travels with the next try - a
// wrong one is said to be wrong - and so does "without its secrets", which puts
// the key's field away.
func TestTheInstallerAsksForTheKeyOfTheBackupsInstallation(t *testing.T) {
	t.Parallel()

	const passphrase = "the installation's own passphrase"

	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("k"), 32))
	other := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("o"), 32))
	file := backupSealedWith(t, key, passphrase)

	app := harness.StartUnconfigured(t)

	ctx, done := installerInGerman(t, app)
	defer done()

	// The page's uploads are kept, and its save is answered here rather than by
	// the installer, so the backup is read on every try and restored on none.
	record := `(() => {
		window.sent = [];
		const fetchOf = window.fetch.bind(window);
		window.fetch = (url, options) => {
			if (String(url).includes('/install/backup')) window.sent.push({ ...options.headers });
			if (String(url).includes('/install/save')) {
				return Promise.resolve(new Response(JSON.stringify({ error: 'stopped by the test' }), { status: 400 }));
			}
			return fetchOf(url, options);
		};
	})()`

	var (
		hiddenAtFirst, shownWhenAsked, fieldWithout bool
		asked, wrong, right, without                string
		sent                                        []map[string]string
	)

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(record, nil),
		chromedp.SetValue("#token", setupToken(t, app), chromedp.ByQuery),
		chromedp.Click("#restore", chromedp.ByQuery),
		chromedp.SetUploadFiles("#restore-file", []string{file}, chromedp.ByQuery),
		chromedp.SetValue("#restore-passphrase", passphrase, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('#restore-key').hidden`, &hiddenAtFirst),
		whatTheInstallerSays("save", &asked),
		chromedp.Evaluate(`!document.querySelector('#restore-key').hidden`, &shownWhenAsked),
	); err != nil {
		t.Fatalf("offering the backup: %v", err)
	}

	if !hiddenAtFirst {
		t.Error("the part about the backup's secrets is shown before any backup has been read")
	}

	if !shownWhenAsked || !strings.Contains(asked, "Gib den SECRET_KEY jener Installation an") {
		t.Fatalf("after reading the backup the page shows the part %v and says %q, want it shown and the "+
			"installer's question in German", shownWhenAsked, asked)
	}

	// The page as the question leaves it, kept for a person to look at.
	var shot []byte

	if err := chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond), chromedp.FullScreenshot(&shot, 100)); err == nil {
		_ = os.WriteFile("installersecretkey.png", shot, 0o600)
	}

	if err := chromedp.Run(ctx,
		chromedp.SetValue("#restore-secret-key", other, chromedp.ByQuery),
		whatTheInstallerSays("save", &wrong),
		chromedp.SetValue("#restore-secret-key", key, chromedp.ByQuery),
		whatTheInstallerSays("save", &right),
		chromedp.Click("#secrets-without", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('#restore-secret-key-field').hidden`, &fieldWithout),
		whatTheInstallerSays("save", &without),
		chromedp.Evaluate(`window.sent`, &sent),
	); err != nil {
		t.Fatalf("answering the installer about the backup's secrets: %v", err)
	}

	if !strings.Contains(wrong, "öffnet die Geheimnisse dieser Sicherung nicht") {
		t.Errorf("with another key the page says %q, want that it does not open the backup's secrets", wrong)
	}

	if !strings.Contains(right, "stopped by the test") || !strings.Contains(without, "stopped by the test") {
		t.Errorf("with the right key the page says %q and without the secrets %q, want both past the "+
			"backup and on to saving", right, without)
	}

	if !fieldWithout {
		t.Error("the key's field stays out when the backup is to be restored without its secrets")
	}

	encoded := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }

	if len(sent) != 4 ||
		sent[0]["X-Backup-Secret-Key"] != "" || sent[0]["X-Backup-Without-Secrets"] != "" ||
		sent[1]["X-Backup-Secret-Key"] != encoded(other) ||
		sent[2]["X-Backup-Secret-Key"] != encoded(key) ||
		sent[3]["X-Backup-Without-Secrets"] != "true" || sent[3]["X-Backup-Secret-Key"] != "" {
		t.Errorf("the page sent %v, want nothing about the secrets first, then each key typed, then "+
			"without its secrets", sent)
	}
}
