package test

import (
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
)

// Every table the migration chain leaves is in a backup, or named as left out
// with the reason; and a backup lists each table after every table it points at.
//
// A table missing from the list does not fail loudly: the backup is made, the
// restore succeeds, and whatever the table held is simply not there afterwards -
// noticed on the day somebody needs it. And the order is what a restore inserts
// in, so a table listed before one it points at is refused by PostgreSQL's
// foreign key on the first row, and only there: SQLite and MySQL enforce none of
// these, so a restore that works on two dialects fails on the third.
func TestEveryTableIsInABackupInTheOrderARestoreNeeds(t *testing.T) {
	chain := read(t, filepath.Join("..", "internal", "infrastructure", "persistence", "migration", "migration.go"))

	tables, parents := tablesAndParents(chain)
	if len(tables) < 15 {
		t.Fatalf("read %d tables from the migration chain, so the scan has stopped reading", len(tables))
	}

	for table := range tables {
		_, leftOut := sqldb.LeftOutOfBackups[table]
		if !leftOut && !slices.Contains(sqldb.BackupTables, table) {
			t.Errorf("the chain creates %s, and a backup neither carries it nor says why not", table)
		}
	}

	for table := range sqldb.LeftOutOfBackups {
		if !tables[table] {
			t.Errorf("%s is named as left out of a backup, and the chain does not create it", table)
		}
	}

	for at, table := range sqldb.BackupTables {
		if !tables[table] {
			t.Errorf("a backup carries %s, which the chain does not create", table)

			continue
		}

		for parent := range parents[table] {
			if parent == table {
				continue
			}

			if before := slices.Index(sqldb.BackupTables, parent); before < 0 || before > at {
				t.Errorf("%s points at %s, which a restore would insert after it", table, parent)
			}
		}
	}
}

// tablesAndParents reads the tables the chain leaves - created, less those
// dropped or renamed away, plus what they were renamed to - and the tables each
// one points at.
func tablesAndParents(chain string) (map[string]bool, map[string]map[string]bool) {
	created := regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?([a-z_]+)`)
	referenced := regexp.MustCompile(`REFERENCES ([a-z_]+)\(`)
	added := regexp.MustCompile("ALTER TABLE ([a-z_]+) ADD COLUMN [a-z_]+[^\"`]*REFERENCES ([a-z_]+)\\(")
	renamed := regexp.MustCompile(`ALTER TABLE ([a-z_]+) RENAME TO ([a-z_]+)`)
	dropped := regexp.MustCompile(`DROP TABLE (?:IF EXISTS )?([a-z_]+)`)

	tables := map[string]bool{}
	parents := map[string]map[string]bool{}

	point := func(child, parent string) {
		if parents[child] == nil {
			parents[child] = map[string]bool{}
		}

		parents[child][parent] = true
	}

	for _, m := range created.FindAllStringSubmatch(chain, -1) {
		tables[m[1]] = true
	}

	// A reference inside a CREATE TABLE belongs to the nearest one above it.
	for _, at := range referenced.FindAllStringSubmatchIndex(chain, -1) {
		above := created.FindAllStringSubmatch(chain[:at[0]], -1)
		if len(above) > 0 {
			point(above[len(above)-1][1], chain[at[2]:at[3]])
		}
	}

	for _, m := range added.FindAllStringSubmatch(chain, -1) {
		point(m[1], m[2])
	}

	for _, m := range dropped.FindAllStringSubmatch(chain, -1) {
		delete(tables, m[1])
	}

	for _, m := range renamed.FindAllStringSubmatch(chain, -1) {
		delete(tables, m[1])
		tables[m[2]] = true

		for parent := range parents[m[1]] {
			point(m[2], parent)
		}

		delete(parents, m[1])
	}

	return tables, parents
}
