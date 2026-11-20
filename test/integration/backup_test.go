//go:build integration

package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/backup"
	"github.com/dennis-dko/timepulse/test/harness"
)

const backupPassphrase = "the installation's own passphrase"

// backupCard is what GET /settings/backups answers.
type backupCard struct {
	Enabled  bool   `json:"enabled"`
	Folder   string `json:"folder"`
	Schedule string `json:"schedule"`
	Keep     int    `json:"keep"`
	Backups  []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"backups"`
}

func readBackupCard(t *testing.T, admin *client) backupCard {
	t.Helper()

	var card backupCard

	admin.must(admin.api(http.MethodGet, "/settings/backups", nil), http.StatusOK).Data(t, &card)

	return card
}

// newestAdminChange is the action of the newest entry in the administration log.
func newestAdminChange(t *testing.T, c *client) string {
	t.Helper()

	var page struct {
		Entries []struct {
			Action string `json:"action"`
		} `json:"entries"`
	}

	c.must(c.api(http.MethodGet, "/admin/changes", nil), http.StatusOK).Data(t, &page)

	if len(page.Entries) == 0 {
		t.Fatal("the administration log is empty")
	}

	return page.Entries[0].Action
}

// madeBackup makes a backup through the API and downloads it.
func madeBackup(t *testing.T, admin *client) (string, []byte) {
	t.Helper()

	var made struct {
		Name string `json:"name"`
	}

	admin.must(admin.api(http.MethodPost, "/settings/backups", nil), http.StatusOK, http.StatusCreated).Data(t, &made)

	if !backup.ValidName(made.Name) {
		t.Fatalf("a backup was made under the name %q", made.Name)
	}

	return made.Name, admin.must(admin.api(http.MethodGet, "/settings/backups/"+made.Name, nil), http.StatusOK).Body
}

// An administrator makes a backup, finds it listed, downloads a file that opens
// with the server's passphrase to a backup of this database, and deletes it -
// and the administration log holds every step, the download included, because
// a download is the whole installation leaving the server. An account that may
// not administer the installation sees none of it.
func TestAnAdministratorMakesDownloadsAndDeletesABackup(t *testing.T) {
	t.Parallel()

	a := start(t, "BACKUP_PASSPHRASE="+backupPassphrase)
	admin := a.signInAsAdmin("a-much-better-password")
	worker := a.signInAsUser(admin, "Wera", "wera@example.com")

	if card := readBackupCard(t, admin); !card.Enabled || len(card.Backups) != 0 || card.Keep < 1 {
		t.Fatalf("a new installation's card reads %+v, want backups possible and none kept", card)
	}

	if r := worker.api(http.MethodGet, "/settings/backups", nil); r.Status != http.StatusForbidden {
		t.Errorf("an account that does not administer the installation read its backups: %d", r.Status)
	}

	name, file := madeBackup(t, admin)

	if card := readBackupCard(t, admin); len(card.Backups) != 1 || card.Backups[0].Name != name ||
		card.Backups[0].Size != int64(len(file)) {
		t.Errorf("after one backup the card lists %+v, want %s of %d bytes", card.Backups, name, len(file))
	}

	var header sqldb.BackupHeader

	if err := backup.Open(file, backupPassphrase, func(content io.Reader) error {
		var err error
		header, err = sqldb.ReadBackupHeader(content)

		return err
	}); err != nil || header.Dialect == "" || header.CreatedAt.IsZero() {
		t.Errorf("the downloaded file opens to %+v (%v), want a backup naming its database and its moment",
			header, err)
	}

	if action := newestAdminChange(t, admin); action != "installation.backupDownloaded" {
		t.Errorf("the newest change in the administration log is %q, want the download", action)
	}

	admin.must(admin.api(http.MethodDelete, "/settings/backups/"+name, nil), http.StatusOK, http.StatusNoContent)

	if card := readBackupCard(t, admin); len(card.Backups) != 0 {
		t.Errorf("a deleted backup is still listed: %+v", card.Backups)
	}

	if r := admin.api(http.MethodGet, "/settings/backups/"+name, nil); r.Status != http.StatusNotFound {
		t.Errorf("a deleted backup still downloads: %d", r.Status)
	}

	if action := newestAdminChange(t, admin); action != "installation.backupDeleted" {
		t.Errorf("the newest change in the administration log is %q, want the deletion", action)
	}
}

// Without BACKUP_PASSPHRASE the card says so and no backup is made: an
// unencrypted backup is not the fallback, it is what was ruled out.
func TestNoBackupIsMadeWithoutAPassphrase(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	if card := readBackupCard(t, admin); card.Enabled {
		t.Error("an installation without BACKUP_PASSPHRASE says it can make backups")
	}

	refused := admin.api(http.MethodPost, "/settings/backups", nil)
	if got := refusalOf(t, refused); refused.Status != http.StatusConflict || got.Code != "backupNeedsPassphrase" {
		t.Errorf("making a backup without a passphrase answered %d %q, want 409 backupNeedsPassphrase",
			refused.Status, got.Code)
	}
}

// A plan the scheduler could not read is refused as a field rather than stored,
// and one it can is kept and read back.
func TestABackupPlanIsKeptOnlyIfItCanRun(t *testing.T) {
	t.Parallel()

	a := start(t, "BACKUP_PASSPHRASE="+backupPassphrase)
	admin := a.signInAsAdmin("a-much-better-password")

	never := admin.api(http.MethodPut, "/settings/backups/plan", map[string]any{"schedule": "0 25 * * *", "keep": 7})
	if never.Status != http.StatusBadRequest {
		t.Errorf("a schedule with a twenty-fifth hour was answered %d, want 400", never.Status)
	}

	admin.must(admin.api(http.MethodPut, "/settings/backups/plan", map[string]any{"schedule": "0 3 * * *", "keep": 7}),
		http.StatusOK)

	if card := readBackupCard(t, admin); card.Schedule != "0 3 * * *" || card.Keep != 7 {
		t.Errorf("the plan reads back as %q keeping %d, want 0 3 * * * keeping 7", card.Schedule, card.Keep)
	}
}

// uploadBackup sends a backup to the installer the way its page does: the file
// as the body, the passphrase as base64 of its UTF-8 in a header.
func uploadBackup(t *testing.T, c *installerClient, file []byte, passphrase string) (int, map[string]any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, c.app.BaseURL()+"/install/backup", bytes.NewReader(file))
	if err != nil {
		t.Fatalf("cannot build the upload: %v", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Setup-Token", c.token)
	req.Header.Set("X-Backup-Passphrase", base64.StdEncoding.EncodeToString([]byte(passphrase)))

	res, err := (&http.Client{Timeout: time.Minute}).Do(req)
	if err != nil {
		t.Fatalf("upload the backup: %v", err)
	}

	defer func() { _ = res.Body.Close() }()

	var body map[string]any

	_ = json.NewDecoder(res.Body).Decode(&body)

	return res.StatusCode, body
}

// A backup comes back through the installer of a new installation: everybody
// signs in with the password they had, and finds what they had booked - into a
// database of the dialect the suite runs on, so the CI runs on PostgreSQL and
// MySQL restore into their own as well as from them.
//
// The installer opens the file before a database is chosen for it, so a wrong
// passphrase is said on its page, and it refuses a restore into a database that
// already holds accounts.
func TestABackupIsRestoredThroughTheInstallerOfANewInstallation(t *testing.T) {
	t.Parallel()

	source := start(t, "BACKUP_PASSPHRASE="+backupPassphrase)
	admin := source.signInAsAdmin("a-much-better-password")
	worker := source.signInAsUser(admin, "Wera", "wera@example.com")

	worker.must(worker.api(http.MethodPost, "/timesheets", map[string]any{
		"date": "2026-11-19", "durationHours": 2.75, "description": "Before the restore",
	}), http.StatusCreated, http.StatusOK)

	admin.must(admin.api(http.MethodPut, "/settings/backups/plan", map[string]any{"schedule": "", "keep": 3}),
		http.StatusOK)

	_, file := madeBackup(t, admin)

	c := openInstaller(t)

	if status, body := uploadBackup(t, c, file, "not the passphrase at all"); status != http.StatusBadRequest ||
		body["code"] != "backupWrongPassphrase" {
		t.Errorf("a wrong passphrase was answered %d %v, want 400 backupWrongPassphrase", status, body)
	}

	if status, body := uploadBackup(t, c, file, backupPassphrase); status != http.StatusOK {
		t.Fatalf("the backup was not taken: %d %v", status, body)
	}

	answer := map[string]any{"restore": true, "language": "en"}
	for key, value := range harness.FreshDatasource(t) {
		answer[key] = value
	}

	if status, body := c.post("/install/save", c.token, answer); status != http.StatusOK {
		t.Fatalf("saving the connection with the restore asked for: %d %v", status, body)
	}

	waitForTheApplication(t, c)

	restored := &app{App: c.app, t: t}

	back := restored.newClient()
	back.signIn("wera@example.com", "another-password-1")

	var entries listOf[timesheetResponse]

	back.must(back.api(http.MethodGet, "/timesheets", nil), http.StatusOK).Data(t, &entries)

	if entries.TotalCount != 1 || entries.Items[0].DurationHours != 2.75 {
		t.Fatalf("after the restore Wera finds %+v, want her 2.75 hours\n%s", entries, c.app.Log())
	}

	again := restored.newClient()
	again.signIn(adminEmail, "a-much-better-password")

	if card := readBackupCard(t, again); card.Keep != 3 {
		t.Errorf("the restored installation keeps %d backups, want the 3 its plan said", card.Keep)
	}
}

// A backup moves between kinds of database, both ways: one made on the suite's
// database comes back into SQLite, and one made on SQLite comes back into the
// suite's database. The installer offers every kind for a restore and the file
// says nothing about the kind it was made on - trying a server's backup on one's
// own computer is the first thing somebody does with it.
//
// On the SQLite run both directions stay on SQLite, so the PostgreSQL and MySQL
// runs are the ones that cross.
func TestABackupMovesBetweenSQLiteAndTheSuitesDatabase(t *testing.T) {
	t.Parallel()

	sqlite := map[string]string{"dialect": "sqlite", "name": "chosen"}

	for _, direction := range []struct {
		name   string
		source []string
		madeOn string
		target map[string]string
	}{
		{"from the suite's database into SQLite", nil, "", sqlite},
		{"from SQLite into the suite's database", []string{"DB_DIALECT=sqlite", "DB_NAME=source"}, "sqlite", nil},
	} {
		t.Run(direction.name, func(t *testing.T) {
			t.Parallel()

			source := start(t, append([]string{"BACKUP_PASSPHRASE=" + backupPassphrase}, direction.source...)...)
			admin := source.signInAsAdmin("a-much-better-password")
			worker := source.signInAsUser(admin, "Wera", "wera@example.com")

			worker.must(worker.api(http.MethodPost, "/timesheets", map[string]any{
				"date": "2026-11-19", "durationHours": 2.75, "description": "Before the restore",
			}), http.StatusCreated, http.StatusOK)

			_, file := madeBackup(t, admin)

			c := openInstaller(t)

			status, body := uploadBackup(t, c, file, backupPassphrase)
			if status != http.StatusOK {
				t.Fatalf("the backup was not taken: %d %v", status, body)
			}

			// What the file says it was made on, so the case is seen to cross rather
			// than assumed to: the source's environment is what decides it.
			madeOn := direction.madeOn
			if madeOn == "" {
				madeOn = source.Dialect()
			}

			if body["dialect"] != madeOn {
				t.Fatalf("the backup says it was made on %v, want %s", body["dialect"], madeOn)
			}

			target := direction.target
			if target == nil {
				target = harness.FreshDatasource(t)
			}

			answer := map[string]any{"restore": true, "language": "en"}
			for key, value := range target {
				answer[key] = value
			}

			if status, body := c.post("/install/save", c.token, answer); status != http.StatusOK {
				t.Fatalf("saving the connection with the restore asked for: %d %v", status, body)
			}

			waitForTheApplication(t, c)

			back := (&app{App: c.app, t: t}).newClient()
			back.signIn("wera@example.com", "another-password-1")

			var entries listOf[timesheetResponse]

			back.must(back.api(http.MethodGet, "/timesheets", nil), http.StatusOK).Data(t, &entries)

			if entries.TotalCount != 1 || entries.Items[0].DurationHours != 2.75 {
				t.Fatalf("after the restore Wera finds %+v, want her 2.75 hours\n%s", entries, c.app.Log())
			}

			// And she books again: the ids a restore wrote are taken, so a database
			// that counts its own - PostgreSQL's sequences - has to count on from them.
			back.must(back.api(http.MethodPost, "/timesheets", map[string]any{
				"date": "2026-11-20", "durationHours": 1.5, "description": "After the restore",
			}), http.StatusCreated, http.StatusOK)
		})
	}
}
