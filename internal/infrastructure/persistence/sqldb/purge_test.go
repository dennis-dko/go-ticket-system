package sqldb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"

	gofrmigration "gofr.dev/pkg/gofr/migration"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/migration"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/sqldb"

	_ "modernc.org/sqlite"
)

// newTestDB opens a throwaway SQLite database and applies the real migrations,
// so the cascade is exercised against the schema that actually ships rather
// than a hand-written copy of it.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "purge-test.db")

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	all := migration.All(sqldb.DialectSQLite)

	versions := make([]int64, 0, len(all))
	for version := range all {
		versions = append(versions, version)
	}

	// Migrations must run in their declared order.
	slices.Sort(versions)

	for _, version := range versions {
		if err := all[version].UP(gofrmigration.Datasource{SQL: db}); err != nil {
			t.Fatalf("migration %d: %v", version, err)
		}
	}

	return db
}

// twoAccounts creates the account a case removes and one that has to survive it.
func twoAccounts(t *testing.T, db *sql.DB) (leaving, staying *model.User) {
	t.Helper()

	ctx := context.Background()
	users := sqldb.NewUserRepository(db, sqldb.DialectSQLite)

	// The migrations seed the roles, so an existing one can be used.
	role, err := sqldb.NewRoleRepository(db, sqldb.DialectSQLite).GetByName(ctx, model.RoleAgent)
	if err != nil {
		t.Fatalf("read seeded role: %v", err)
	}

	leaving, err = users.Save(ctx, &model.User{
		Name: "Leaving", Email: "leaving@example.com", RoleID: role.ID, IsExternal: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	staying, err = users.Save(ctx, &model.User{
		Name: "Staying", Email: "staying@example.com", RoleID: role.ID,
	})
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	return leaving, staying
}

// ticket writes a ticket straight into the table, in the seeded queue, and
// returns its id. The purge is about what points at an account, so the cases
// below need rows that do and nothing of how a ticket is otherwise made.
func ticket(t *testing.T, db *sql.DB, number string, state model.TicketState, owner, responsible *uint) uint {
	t.Helper()

	now := time.Now().UTC()

	result, err := db.Exec(`INSERT INTO tickets
		(number, title, queue_id, state, priority, owner_id, responsible_id, locked, locked_at,
		 channel, created_at, updated_at)
		VALUES (?, ?, (SELECT id FROM queues ORDER BY id LIMIT 1), ?, 3, ?, ?, ?, ?, 'phone', ?, ?)`,
		number, "Ticket "+number, string(state), owner, responsible, owner != nil, now, now, now)
	if err != nil {
		t.Fatalf("create ticket %s: %v", number, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read the id of ticket %s: %v", number, err)
	}

	return uint(id)
}

// count is how many rows a statement with one parameter counts.
func count(t *testing.T, db *sql.DB, query string, arg any) int {
	t.Helper()

	var n int
	if err := db.QueryRow(query, arg).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}

	return n
}

// PurgeUser removes the account and what is its own, and takes its name off
// what is not: a ticket it owned stays, without an owner and without a lock,
// and a contact it signed in as stays without the account.
func TestPurgeUserRemovesWhatIsTheAccountsAndReleasesWhatIsNot(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	leaving, staying := twoAccounts(t, db)
	users := sqldb.NewUserRepository(db, sqldb.DialectSQLite)

	owned := ticket(t, db, "2026111200000001", model.StateOpen, &leaving.ID, nil)
	answeredFor := ticket(t, db, "2026111200000002", model.StateOpen, &staying.ID, &leaving.ID)
	watched := ticket(t, db, "2026111200000003", model.StateNew, nil, nil)

	for _, statement := range []string{
		"INSERT INTO ticket_watchers (ticket_id, user_id) VALUES (?, ?)",
		"INSERT INTO notifications (user_id, ticket_id, kind, created_at) VALUES (?2, ?1, 'answered', CURRENT_TIMESTAMP)",
	} {
		if _, err := db.Exec(statement, watched, leaving.ID); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	if _, err := db.Exec("INSERT INTO user_queues (user_id, queue_id) "+
		"VALUES (?, (SELECT id FROM queues ORDER BY id LIMIT 1))", leaving.ID); err != nil {
		t.Fatalf("subscribe to a queue: %v", err)
	}

	if _, err := db.Exec("INSERT INTO contacts (name, email, user_id, created_at) "+
		"VALUES ('Leaving', 'leaving@example.com', ?, CURRENT_TIMESTAMP)", leaving.ID); err != nil {
		t.Fatalf("create the contact: %v", err)
	}

	if _, err := sqldb.NewAPITokenRepository(db, sqldb.DialectSQLite).Save(ctx, &model.APIToken{
		UserID: leaving.ID, Name: "ci", TokenHash: "hash", Prefix: "gts_x", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("create token: %v", err)
	}

	err := sqldb.NewSessionRepository(db, sqldb.DialectSQLite).Save(ctx, &model.Session{
		TokenHash: "session-hash", UserID: leaving.ID,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// A passkey, which the purge once forgot. Its table has a foreign key to
	// users like the rest, so forgetting it does not fail quietly: on PostgreSQL
	// the final delete is refused and the account can never be removed, while on
	// SQLite and MySQL the credential outlives the person it belonged to.
	if _, err := sqldb.NewPasskeyRepository(db, sqldb.DialectSQLite).Save(ctx, &model.Passkey{
		UserID: leaving.ID, Name: "work laptop",
		CredentialID: []byte("credential-id"), PublicKey: []byte("public-key"),
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("create passkey: %v", err)
	}

	if err := users.PurgeUser(ctx, leaving.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}

	// Nothing that is the departed account's own may remain, and nothing may
	// still point at it.
	for _, check := range []struct {
		what  string
		query string
	}{
		{"users", "SELECT COUNT(*) FROM users WHERE id = ?"},
		{"tickets owned", "SELECT COUNT(*) FROM tickets WHERE owner_id = ?"},
		{"tickets answered for", "SELECT COUNT(*) FROM tickets WHERE responsible_id = ?"},
		{"contacts signed in as", "SELECT COUNT(*) FROM contacts WHERE user_id = ?"},
		{"watched tickets", "SELECT COUNT(*) FROM ticket_watchers WHERE user_id = ?"},
		{"notifications", "SELECT COUNT(*) FROM notifications WHERE user_id = ?"},
		{"queues to be told about", "SELECT COUNT(*) FROM user_queues WHERE user_id = ?"},
		{"api tokens", "SELECT COUNT(*) FROM api_tokens WHERE user_id = ?"},
		{"sessions", "SELECT COUNT(*) FROM sessions WHERE user_id = ?"},
		{"passkeys", "SELECT COUNT(*) FROM passkeys WHERE user_id = ?"},
	} {
		if n := count(t, db, check.query, leaving.ID); n != 0 {
			t.Errorf("%d %s remained after the purge", n, check.what)
		}
	}

	// The tickets are the organisation's and stay, all three of them.
	if n := count(t, db, "SELECT COUNT(*) FROM tickets WHERE id IN (?1, ?1 + 1, ?1 + 2)", owned); n != 3 {
		t.Errorf("%d of the three tickets remain; a ticket outlives the account that owned it", n)
	}

	// The one the account owned is free for somebody else: no owner, no lock.
	var (
		locked   bool
		lockedAt sql.NullString
	)

	if err := db.QueryRow("SELECT locked, locked_at FROM tickets WHERE id = ?", owned).
		Scan(&locked, &lockedAt); err != nil {
		t.Fatalf("read the released ticket: %v", err)
	}

	if locked || lockedAt.Valid {
		t.Errorf("the released ticket is locked=%v since %v; nobody could take it", locked, lockedAt.String)
	}

	// The one it only answered for keeps its owner and its lock.
	if n := count(t, db, "SELECT COUNT(*) FROM tickets WHERE locked AND owner_id = ?", staying.ID); n != 1 {
		t.Errorf("the other account's ticket %d lost its owner or its lock", answeredFor)
	}

	// And the contact is still a contact.
	if n := count(t, db, "SELECT COUNT(*) FROM contacts WHERE email = ?", "leaving@example.com"); n != 1 {
		t.Error("the contact went with the account it signed in as")
	}

	if _, err := users.GetByID(ctx, staying.ID); err != nil {
		t.Errorf("the remaining user must survive: %v", err)
	}
}

// What an account would leave behind is the tickets it owns that somebody still
// has to answer: a finished one loses its owner's name the same way, and nobody
// is waiting on it.
func TestAnAccountLeavesBehindTheTicketsItOwnsThatAreNotFinished(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	leaving, staying := twoAccounts(t, db)
	users := sqldb.NewUserRepository(db, sqldb.DialectSQLite)

	ticket(t, db, "2026111200000001", model.StateOpen, &leaving.ID, nil)
	ticket(t, db, "2026111200000002", model.StatePendingReminder, &leaving.ID, nil)
	ticket(t, db, "2026111200000003", model.StateClosedSuccessful, &leaving.ID, nil)
	ticket(t, db, "2026111200000004", model.StateMerged, &leaving.ID, nil)
	ticket(t, db, "2026111200000005", model.StateOpen, &staying.ID, &leaving.ID)

	got, err := users.CountUnfinishedOwnedBy(ctx, leaving.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if got != 2 {
		t.Errorf("the account is said to leave %d tickets behind, want the 2 it owns that are not finished", got)
	}
}

// Purging twice must not fail differently the second time: the sync repeats a
// failed run, so the operation has to be safe to re-enter.
func TestPurgeUserIsSafeToRepeat(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	users := sqldb.NewUserRepository(db, sqldb.DialectSQLite)
	roles := sqldb.NewRoleRepository(db, sqldb.DialectSQLite)

	role, err := roles.GetByName(ctx, model.RoleAgent)
	if err != nil {
		t.Fatalf("read seeded role: %v", err)
	}

	user, err := users.Save(ctx, &model.User{
		Name: "Gone", Email: "gone@example.com", RoleID: role.ID, IsExternal: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := users.PurgeUser(ctx, user.ID); err != nil {
		t.Fatalf("first purge: %v", err)
	}

	// The second run finds nothing left and reports that, rather than
	// corrupting anything.
	if err := users.PurgeUser(ctx, user.ID); err == nil {
		t.Error("purging an already-removed account should report it as missing")
	}
}
