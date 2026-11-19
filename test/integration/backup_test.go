//go:build integration

package integration

import (
	"io"
	"net/http"
	"testing"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/backup"
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
