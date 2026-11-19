package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/domain/repository"
	"github.com/dennis-dko/timepulse/internal/support/apperror"
	"github.com/dennis-dko/timepulse/internal/support/backup"
)

// Dumper writes every table of the installation's database to w, answering with
// the number of rows. It is the persistence layer's, handed in so this service
// knows nothing of SQL.
type Dumper func(ctx context.Context, w io.Writer) (int64, error)

// BackupService makes backups of the installation, keeps them in a folder on the
// server, and removes the oldest past the number its plan keeps.
//
// Every backup is sealed with BACKUP_PASSPHRASE, which the server holds and no
// file does - the maintainer's choice, because a backup is downloaded and a
// download ends up on a laptop. Without one, none is made at all: an
// unencrypted backup is not the fallback, it is the thing that was ruled out.
type BackupService struct {
	settings   repository.SettingsRepository
	dump       Dumper
	folder     backup.Directory
	passphrase string
	zone       func(ctx context.Context) *time.Location
	now        func() time.Time

	// making is held for the length of one backup, so a scheduled one and one
	// asked for by hand in the same second do not both read the database and race
	// for one name.
	making sync.Mutex
}

// NewBackupService keeps backups in folder, sealed with passphrase, and reads
// schedules in the zone zone answers with.
func NewBackupService(
	settings repository.SettingsRepository,
	dump Dumper,
	folder backup.Directory,
	passphrase string,
	zone func(ctx context.Context) *time.Location,
) *BackupService {
	return &BackupService{
		settings: settings, dump: dump, folder: folder, passphrase: passphrase, zone: zone, now: time.Now,
	}
}

// WithClock makes backups at the moments now answers with, for a test making
// several in one second - which, named to the second, would be one.
func (s *BackupService) WithClock(now func() time.Time) *BackupService {
	s.now = now

	return s
}

// BackupState is what the backup card shows.
type BackupState struct {
	// Enabled says a backup can be made: BACKUP_PASSPHRASE is set, and long
	// enough to seal one.
	Enabled bool
	Folder  string
	Plan    model.BackupPlan
	Stored  []backup.Stored
}

// State reads the plan and what the folder holds.
func (s *BackupService) State(ctx context.Context) (BackupState, error) {
	plan, err := s.Plan(ctx)
	if err != nil {
		return BackupState{}, err
	}

	stored, err := s.folder.List()
	if err != nil {
		return BackupState{}, apperror.Internal(err)
	}

	return BackupState{Enabled: s.enabled(), Folder: s.folder.Path(), Plan: plan, Stored: stored}, nil
}

// Plan is the stored plan, or the default one - no schedule, the default number
// kept - where none has been saved.
func (s *BackupService) Plan(ctx context.Context) (model.BackupPlan, error) {
	raw, err := s.settings.Get(ctx, model.SettingBackupPlan)
	if err != nil {
		return model.BackupPlan{}, err
	}

	var plan model.BackupPlan

	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &plan); err != nil {
			// The reading the other settings take of a stored value gone bad: the
			// default, rather than a card that will not open.
			plan = model.BackupPlan{}
		}
	}

	return plan.Normalized(), nil
}

// SavePlan stores when backups are made on their own and how many are kept.
//
// A schedule the scheduler could not read is refused rather than stored, for
// the reason the directory's is: stored, it would never run and nothing would
// say so - and a backup that silently never happens is found on the day it is
// needed.
func (s *BackupService) SavePlan(ctx context.Context, plan model.BackupPlan) (model.BackupPlan, error) {
	plan = plan.Normalized()

	if !model.ValidSchedule(plan.Schedule) || model.TooLong(plan.Schedule, model.MaxScheduleLength) {
		return plan, apperror.InvalidFields("schedule")
	}

	if !plan.Valid() {
		return plan, apperror.InvalidFields("keep")
	}

	raw, err := json.Marshal(plan)
	if err != nil {
		return plan, apperror.Internal(err)
	}

	return plan, s.settings.Set(ctx, model.SettingBackupPlan, string(raw))
}

// Make makes a backup now, keeps it in the folder, and removes the oldest past
// the number the plan keeps.
func (s *BackupService) Make(ctx context.Context) (backup.Stored, error) {
	if !s.enabled() {
		return backup.Stored{}, apperror.Conflictf(
			"backups are sealed with BACKUP_PASSPHRASE, and the server has none of at least %d characters",
			backup.MinPassphraseLength).WithCode("backupNeedsPassphrase", fmt.Sprint(backup.MinPassphraseLength))
	}

	plan, err := s.Plan(ctx)
	if err != nil {
		return backup.Stored{}, err
	}

	s.making.Lock()
	defer s.making.Unlock()

	file, err := backup.Seal(s.passphrase, func(w io.Writer) error {
		_, err := s.dump(ctx, w)

		return err
	})
	if err != nil {
		return backup.Stored{}, apperror.Internal(fmt.Errorf("make the backup: %w", err))
	}

	made := s.now()
	name := backup.Name(made)

	if err := s.folder.Keep(name, file); err != nil {
		return backup.Stored{}, apperror.Internal(fmt.Errorf("keep the backup: %w", err))
	}

	if err := s.prune(plan.Keep); err != nil {
		return backup.Stored{}, apperror.Internal(fmt.Errorf("the backup %s was kept, and removing the "+
			"oldest past %d failed: %w", name, plan.Keep, err))
	}

	return backup.Stored{Name: name, Size: int64(len(file)), Made: made.UTC().Truncate(time.Second)}, nil
}

// Read is the backup of that name, for downloading.
func (s *BackupService) Read(name string) ([]byte, error) {
	file, err := s.folder.Read(name)
	if errors.Is(err, backup.ErrNoSuchBackup) {
		return nil, apperror.NotFound("backup", name)
	}

	if err != nil {
		return nil, apperror.Internal(err)
	}

	return file, nil
}

// Delete removes the backup of that name.
func (s *BackupService) Delete(name string) error {
	err := s.folder.Remove(name)
	if errors.Is(err, backup.ErrNoSuchBackup) {
		return apperror.NotFound("backup", name)
	}

	if err != nil {
		return apperror.Internal(err)
	}

	return nil
}

// RunDue makes a backup if the plan's schedule names the minute at falls in, in
// the installation's timezone, and says whether it did. It is asked every
// minute, which is what lets a schedule saved on the card apply at once: the
// framework's scheduler takes its jobs once, when the process starts.
func (s *BackupService) RunDue(ctx context.Context, at time.Time) (bool, error) {
	plan, err := s.Plan(ctx)
	if err != nil {
		return false, err
	}

	if !model.ScheduleMatches(plan.Schedule, at.In(s.zone(ctx))) {
		return false, nil
	}

	if _, err := s.Make(ctx); err != nil {
		return false, err
	}

	return true, nil
}

// prune removes the oldest backups until keep are left.
func (s *BackupService) prune(keep int) error {
	stored, err := s.folder.List()
	if err != nil {
		return err
	}

	for _, old := range stored[min(keep, len(stored)):] {
		if err := s.folder.Remove(old.Name); err != nil && !errors.Is(err, backup.ErrNoSuchBackup) {
			return err
		}
	}

	return nil
}

func (s *BackupService) enabled() bool {
	return utf8.RuneCountInString(s.passphrase) >= backup.MinPassphraseLength
}
