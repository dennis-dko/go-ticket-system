package test

import (
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// Every column that points at an account is cleared when the account is erased.
//
// PurgeUser is the erasure path, used by directory synchronisation as well as by
// an administrator deleting somebody deliberately, and irreversible either way.
// What is the account's own is deleted with it; what is the organisation's - a
// ticket it owned, the contact it reached the portal as - stays and has the
// account taken off it, the column set to NULL.
//
// A column missing from its list does not fail loudly, which is the whole reason
// for this. On PostgreSQL the final DELETE is refused while anything still
// references the account - inside a transaction that then rolls back, so the
// erasure simply never succeeds. On SQLite and MySQL, neither of which enforces
// the keys this schema declares, it succeeds and leaves rows pointing at an
// account that no longer exists.
//
// This has already happened once, with passkeys. CLAUDE.md records it, and
// records the remedy as "add it to the list in the same commit" - which is a rule
// held up by whoever remembers it at the time.
//
// It also records why the obvious scan is not enough: `grep 'REFERENCES
// users(id)'` missed a column added by an ALTER, which appears in no CREATE
// TABLE. So this reads the whole chain for both shapes, and follows RENAME TO -
// a SQLite rebuild creates a table under a new name and renames it, and a check
// that did not follow that would be looking for a table that does not exist
// under a name nothing uses. And it keeps every such column of a table rather
// than one: tickets point at an account twice, as owner and as responsible, and
// a check that remembered one of them would never ask about the other.
func TestEveryTableReferencingAnAccountIsPurgedWithIt(t *testing.T) {
	root := ".."
	chain := read(t, filepath.Join(root,
		"internal", "infrastructure", "persistence", "migration", "migration.go"))
	purge := read(t, filepath.Join(root,
		"internal", "infrastructure", "persistence", "sqldb", "purge.go"))

	referencing := referencesToUsers(chain)
	if len(referencing) == 0 {
		t.Fatal("no references to users(id) found; this test is reading nothing")
	}

	// The account's own table is not one of the things cleared before it.
	delete(referencing, "users")

	var missing []string

	for table, columns := range referencing {
		for column := range columns {
			if !clears(purge, table, column) {
				missing = append(missing, table+"."+column)
			}
		}
	}

	sort.Strings(missing)

	for _, one := range missing {
		t.Errorf("%s points at an account and PurgeUser neither deletes its rows nor "+
			"sets it to NULL; on PostgreSQL that makes the erasure impossible, and on "+
			"SQLite and MySQL it leaves rows behind pointing at nobody", one)
	}
}

// clears reports whether purge deletes the rows of table that point at the
// account through column, or sets column to NULL on them.
func clears(purge, table, column string) bool {
	quotedTable, quotedColumn := regexp.QuoteMeta(table), regexp.QuoteMeta(column)

	deletion := regexp.MustCompile(
		`DELETE FROM ` + quotedTable + `\s+WHERE\s+` + quotedColumn + `\s*=`)

	release := regexp.MustCompile(
		`UPDATE ` + quotedTable + `\s+SET\s+[^"]*\b` + quotedColumn + `\s*=\s*NULL\b[^"]*\bWHERE\s+` +
			quotedColumn + `\s*=`)

	return deletion.MatchString(purge) || release.MatchString(purge)
}

// referencesToUsers maps each table with a foreign key to users onto every
// column that holds one, with renames applied.
func referencesToUsers(chain string) map[string]map[string]bool {
	found := map[string]map[string]bool{}

	add := func(table, column string) {
		if found[table] == nil {
			found[table] = map[string]bool{}
		}

		found[table][column] = true
	}

	// Inside a CREATE TABLE body: the column line names the column, and the
	// nearest CREATE TABLE above it names the table.
	created := regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?([a-z_]+)`)
	inTable := regexp.MustCompile(`(?m)^\s*([a-z_]+)\s+\S+.*REFERENCES users\(id\)`)

	for _, at := range inTable.FindAllStringSubmatchIndex(chain, -1) {
		column := chain[at[2]:at[3]]

		tables := created.FindAllStringSubmatchIndex(chain[:at[0]], -1)
		if len(tables) == 0 {
			continue
		}

		last := tables[len(tables)-1]
		add(chain[last[2]:last[3]], column)
	}

	// And added later by an ALTER, which reading only the CREATE statements
	// would miss.
	altered := regexp.MustCompile(
		`ALTER TABLE ([a-z_]+) ADD COLUMN ([a-z_]+)[^"` + "`" + `]*REFERENCES users\(id\)`)

	for _, m := range altered.FindAllStringSubmatch(chain, -1) {
		add(m[1], m[2])
	}

	return withRenames(found, chain)
}

// withRenames follows ALTER TABLE ... RENAME TO, so a table is looked for under
// the name it ends up with.
func withRenames(found map[string]map[string]bool, chain string) map[string]map[string]bool {
	renamed := regexp.MustCompile(`ALTER TABLE ([a-z_]+) RENAME TO ([a-z_]+)`)

	for _, m := range renamed.FindAllStringSubmatch(chain, -1) {
		if columns, ok := found[m[1]]; ok {
			delete(found, m[1])

			if found[m[2]] == nil {
				found[m[2]] = map[string]bool{}
			}

			for column := range columns {
				found[m[2]][column] = true
			}
		}
	}

	return found
}
