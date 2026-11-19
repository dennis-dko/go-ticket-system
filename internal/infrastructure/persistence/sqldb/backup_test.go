package sqldb_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/migration"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
)

// newestMigration is the schema the test databases are at.
func newestMigration() int64 {
	var newest int64
	for version := range migration.All(sqldb.DialectSQLite) {
		newest = max(newest, version)
	}

	return newest
}

// filled is a database with something of each kind a backup has to carry: an
// id, text, a number of hours, a moment, a boolean, bytes and a NULL.
func filled(t *testing.T) (*sql.DB, *model.User) {
	t.Helper()

	db := newTestDB(t)
	ctx := context.Background()

	role, err := sqldb.NewRoleRepository(db, sqldb.DialectSQLite).GetByName(ctx, model.RoleUser)
	if err != nil {
		t.Fatalf("read seeded role: %v", err)
	}

	wera, err := sqldb.NewUserRepository(db, sqldb.DialectSQLite).Save(ctx, &model.User{
		Name: "Wera", Email: "wera@example.com", RoleID: role.ID, MustChangePassword: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	project, err := sqldb.NewProjectRepository(db, sqldb.DialectSQLite).Save(ctx, &model.Project{
		Name: "Portal", StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Status: model.ProjectStatusActive,
		OwnerID: &wera.ID,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	ticket := "Ticket 4711"

	if _, err := sqldb.NewTimesheetRepository(db, sqldb.DialectSQLite).Save(ctx, &model.Timesheet{
		UserID: wera.ID, ProjectID: &project.ID, Date: time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC),
		DurationHours: 2.75, Description: &ticket,
	}); err != nil {
		t.Fatalf("book: %v", err)
	}

	if _, err := sqldb.NewPasskeyRepository(db, sqldb.DialectSQLite).Save(ctx, &model.Passkey{
		UserID: wera.ID, Name: "phone", CredentialID: []byte{0, 1, 2, 250}, PublicKey: []byte("key"),
		CreatedAt: time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("passkey: %v", err)
	}

	return db, wera
}

func dumped(t *testing.T, db *sql.DB) *bytes.Buffer {
	t.Helper()

	var out bytes.Buffer

	if _, err := sqldb.Dump(context.Background(), db, sqldb.BackupHeader{
		Application: "test", Dialect: sqldb.DialectSQLite, Schema: newestMigration(), CreatedAt: time.Now(),
	}, &out); err != nil {
		t.Fatalf("dump: %v", err)
	}

	return &out
}

// A backup restored into a database the migrations have just made holds what
// the original held: the accounts, the hours to the minute, the moments, the
// booleans, the bytes - and the ids, which everything else points at.
func TestABackupRestoresWhatWasBackedUp(t *testing.T) {
	original, wera := filled(t)
	backup := dumped(t, original)

	restored := newTestDB(t)
	ctx := context.Background()

	summary, err := sqldb.Restore(ctx, restored, sqldb.DialectSQLite, newestMigration(), backup)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	if summary.Tables != len(sqldb.BackupTables) {
		t.Errorf("restored %d tables, want all %d", summary.Tables, len(sqldb.BackupTables))
	}

	back, err := sqldb.NewUserRepository(restored, sqldb.DialectSQLite).GetByEmail(ctx, "wera@example.com")
	if err != nil {
		t.Fatalf("the account did not come back: %v", err)
	}

	if back.ID != wera.ID || !back.MustChangePassword || back.Name != "Wera" {
		t.Errorf("the account came back as %+v, want id %d, Wera, must change her password", back, wera.ID)
	}

	entries, err := sqldb.NewTimesheetRepository(restored, sqldb.DialectSQLite).GetAll(ctx)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the entries came back as %v, %v; want one", entries, err)
	}

	if e := entries[0]; e.UserID != wera.ID || e.DurationHours != 2.75 || e.Description == nil ||
		*e.Description != "Ticket 4711" ||
		!e.Date.Equal(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)) || e.ProjectID == nil {
		t.Errorf("the entry came back as %+v", e)
	}

	keys, err := sqldb.NewPasskeyRepository(restored, sqldb.DialectSQLite).GetByUser(ctx, wera.ID)
	if err != nil || len(keys) != 1 || !bytes.Equal(keys[0].CredentialID, []byte{0, 1, 2, 250}) ||
		keys[0].LastUsedAt != nil {
		t.Errorf("the passkey came back as %+v, %v", keys, err)
	}

	// And the next account gets an id of its own rather than one the restore wrote.
	role, _ := sqldb.NewRoleRepository(restored, sqldb.DialectSQLite).GetByName(ctx, model.RoleUser)

	next, err := sqldb.NewUserRepository(restored, sqldb.DialectSQLite).Save(ctx, &model.User{
		Name: "Next", Email: "next@example.com", RoleID: role.ID,
	})
	if err != nil || next.ID <= wera.ID {
		t.Errorf("the next account was given id %v (%v) after a restore that wrote %d", next, err, wera.ID)
	}
}

// A restore goes only into a database that holds no account, from a schema no
// newer than its own, and a backup cut short is refused whole: the database is
// left as the migrations made it, seeded roles and all.
func TestARestoreRefusesWhatItCannotFinish(t *testing.T) {
	original, _ := filled(t)
	backup := dumped(t, original).String()
	ctx := context.Background()

	if _, err := sqldb.Restore(ctx, original, sqldb.DialectSQLite, newestMigration(),
		strings.NewReader(backup)); !errors.Is(err, sqldb.ErrNotEmpty) {
		t.Errorf("into a database with accounts: %v, want ErrNotEmpty", err)
	}

	older := newTestDB(t)
	if _, err := sqldb.Restore(ctx, older, sqldb.DialectSQLite, newestMigration()-1,
		strings.NewReader(backup)); !errors.Is(err, sqldb.ErrNewerSchema) {
		t.Errorf("into an older schema: %v, want ErrNewerSchema", err)
	}

	cut := newTestDB(t)

	var seeded int

	if err := cut.QueryRow("SELECT COUNT(*) FROM roles").Scan(&seeded); err != nil || seeded == 0 {
		t.Fatalf("the migrations seeded %d roles (%v)", seeded, err)
	}

	lines := strings.Split(strings.TrimSpace(backup), "\n")
	short := strings.Join(lines[:len(lines)-1], "\n")

	if _, err := sqldb.Restore(ctx, cut, sqldb.DialectSQLite, newestMigration(),
		strings.NewReader(short)); !errors.Is(err, sqldb.ErrCutShort) {
		t.Errorf("a backup without its last line: %v, want ErrCutShort", err)
	}

	var users, roles int

	_ = cut.QueryRow("SELECT COUNT(*) FROM users").Scan(&users)
	_ = cut.QueryRow("SELECT COUNT(*) FROM roles").Scan(&roles)

	if users != 0 || roles != seeded {
		t.Errorf("a refused restore left %d accounts and %d roles, want none and the %d seeded",
			users, roles, seeded)
	}
}
