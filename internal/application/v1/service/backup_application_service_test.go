package service_test

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/application/v1/service"
	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/apperror"
	"github.com/dennis-dko/timepulse/internal/support/backup"
)

const backupPassphrase = "the server's own passphrase"

// backups is a service over a folder of its own, dumping a database of one line,
// on a clock the test moves.
func backups(t *testing.T, passphrase string, zone *time.Location) (*service.BackupService, *time.Time) {
	t.Helper()

	clock := time.Date(2026, 11, 19, 2, 0, 0, 0, time.UTC)

	dump := func(_ context.Context, w io.Writer) (int64, error) {
		_, err := io.WriteString(w, `{"format":"timepulse-backup"}`+"\n")

		return 1, err
	}

	s := service.NewBackupService(newStubSettings(), dump, backup.NewDirectory(filepath.Join(t.TempDir(), "backups")),
		passphrase, func(context.Context) *time.Location { return zone }).
		WithClock(func() time.Time { return clock })

	return s, &clock
}

// No backup is made without BACKUP_PASSPHRASE, or with one too short to seal a
// file that leaves the server: an unencrypted backup is the thing the
// maintainer ruled out, not a fallback.
func TestNoBackupIsMadeWithoutThePassphrase(t *testing.T) {
	ctx := context.Background()

	for _, passphrase := range []string{"", "short"} {
		s, _ := backups(t, passphrase, time.UTC)

		if state, err := s.State(ctx); err != nil || state.Enabled {
			t.Errorf("passphrase %q: the card says backups can be made (%v)", passphrase, err)
		}

		_, err := s.Make(ctx)

		var refused *apperror.Error
		if !errors.As(err, &refused) || refused.Code != "backupNeedsPassphrase" {
			t.Errorf("passphrase %q: making a backup answered %v, want backupNeedsPassphrase", passphrase, err)
		}
	}
}

// A backup is made sealed, kept in the folder, and the oldest go once more are
// there than the plan keeps - those made by hand included, since they lie in the
// same folder.
func TestABackupIsKeptAndTheOldestGoPastThePlan(t *testing.T) {
	ctx := context.Background()
	s, clock := backups(t, backupPassphrase, time.UTC)

	if _, err := s.SavePlan(ctx, model.BackupPlan{Keep: 2}); err != nil {
		t.Fatalf("save the plan: %v", err)
	}

	var made []string

	for range 3 {
		stored, err := s.Make(ctx)
		if err != nil {
			t.Fatalf("make a backup: %v", err)
		}

		made = append(made, stored.Name)
		*clock = clock.Add(time.Hour)
	}

	state, err := s.State(ctx)
	if err != nil || len(state.Stored) != 2 || state.Stored[0].Name != made[2] || state.Stored[1].Name != made[1] {
		t.Fatalf("the folder holds %+v (%v); want the two newest, %v", state.Stored, err, made[1:])
	}

	file, err := s.Read(made[2])
	if err != nil {
		t.Fatalf("read the newest: %v", err)
	}

	var inside string

	if err := backup.Open(file, backupPassphrase, func(r io.Reader) error {
		read, err := io.ReadAll(r)
		inside = string(read)

		return err
	}); err != nil || !strings.Contains(inside, "timepulse-backup") {
		t.Errorf("the kept file opens to %q, %v", inside, err)
	}

	if _, err := s.Read(made[0]); err == nil {
		t.Error("the oldest backup, one past the plan, is still there")
	}
}

// A plan the scheduler could not read is refused rather than stored - stored, it
// would never run and nothing would say so - and so is a number kept that is
// none or a second database's worth.
func TestAPlanThatWouldNeverRunIsRefused(t *testing.T) {
	ctx := context.Background()
	s, _ := backups(t, backupPassphrase, time.UTC)

	for _, plan := range []model.BackupPlan{
		{Schedule: "0 25 * * *", Keep: 7},
		{Schedule: "every night", Keep: 7},
		{Schedule: "0 3 * * *", Keep: model.MaxBackupsKept + 1},
		{Schedule: "0 3 * * *", Keep: -1},
	} {
		if _, err := s.SavePlan(ctx, plan); err == nil {
			t.Errorf("the plan %+v was stored", plan)
		}
	}

	saved, err := s.SavePlan(ctx, model.BackupPlan{Schedule: " 0 3 * * * "})
	if err != nil || saved.Schedule != "0 3 * * *" || saved.Keep != model.DefaultBackupsKept {
		t.Errorf("a plan naming no number was saved as %+v (%v), want the default number", saved, err)
	}
}

// A scheduled backup runs at the minute its schedule names in the installation's
// timezone, which is the clock the administrator who typed it reads - not the
// server's, which in the shipped image is UTC.
func TestAScheduledBackupRunsInTheInstallationsZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no zone database here: %v", err)
	}

	ctx := context.Background()
	s, _ := backups(t, backupPassphrase, berlin)

	if _, err := s.SavePlan(ctx, model.BackupPlan{Schedule: "0 3 * * *", Keep: 7}); err != nil {
		t.Fatalf("save the plan: %v", err)
	}

	// Three in the morning in Berlin, in November, is two in UTC.
	if made, err := s.RunDue(ctx, time.Date(2026, 11, 19, 3, 0, 0, 0, time.UTC)); err != nil || made {
		t.Errorf("at three UTC: made %v (%v), want nothing", made, err)
	}

	if made, err := s.RunDue(ctx, time.Date(2026, 11, 19, 2, 0, 0, 0, time.UTC)); err != nil || !made {
		t.Errorf("at three in Berlin: made %v (%v), want a backup", made, err)
	}
}
