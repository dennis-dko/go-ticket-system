package test

import (
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// Everything erased with an account, or taken off it, is handed over with it.
//
// Somebody asking for everything held about them (GDPR articles 15 and 20) is
// asking for what PurgeUser touches: the two are the same list read in two
// directions. A table added to the erasure and not to the export is data the
// person can lose without ever having been shown it; a table in the export and
// not in the erasure is data that outlives the account. Both lists are written
// out as statements, so they are compared as statements: the tables purge.go
// deletes from or sets the account's column to NULL in, and the tables
// export.go reads from.
//
// The export reads one kind of table the erasure leaves alone on purpose, named
// in writtenAndKept with the reason.
func TestEveryTableErasedWithAnAccountIsInItsExport(t *testing.T) {
	sqldb := filepath.Join("..", "internal", "infrastructure", "persistence", "sqldb")
	purge := read(t, filepath.Join(sqldb, "purge.go"))

	touched := append(tablesIn(purge, `DELETE FROM (\w+) WHERE`),
		tablesIn(purge, `UPDATE (\w+) SET [^"]*= NULL`)...)
	slices.Sort(touched)
	touched = slices.Compact(touched)

	exported := tablesIn(read(t, filepath.Join(sqldb, "export.go")),
		`FROM (\w+)(?: \w+)? (?:LEFT JOIN \w+ \w+ ON [\w.]+ = [\w.]+ )?WHERE`)

	if len(touched) < 10 {
		t.Fatalf("only %d tables were found in purge.go, so the scan has stopped reading it: %v", len(touched), touched)
	}

	for _, table := range touched {
		if !slices.Contains(exported, table) {
			t.Errorf("PurgeUser erases %s or takes the account off it, and the export does not hand it over", table)
		}
	}

	for _, table := range exported {
		if _, kept := writtenAndKept[table]; kept {
			continue
		}

		if !slices.Contains(touched, table) {
			t.Errorf("the export hands over %s, and PurgeUser does not erase it or take the account off it", table)
		}
	}

	for table := range writtenAndKept {
		if !slices.Contains(exported, table) {
			t.Errorf("writtenAndKept names %s, which the export no longer reads; drop the row", table)
		}

		if slices.Contains(touched, table) {
			t.Errorf("writtenAndKept names %s, which PurgeUser touches after all; drop the row", table)
		}
	}
}

// writtenAndKept are the tables the export reads and the erasure leaves as they
// are, each with the reason.
var writtenAndKept = map[string]string{
	"articles": "what an agent wrote in a ticket is the ticket's record and stays, signed by an id " +
		"that names nobody once the account is gone; the export says where and when it was written",
}

// tablesIn returns every table a pattern's first group names in a source, sorted
// and once each.
func tablesIn(source, pattern string) []string {
	var tables []string

	for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatch(source, -1) {
		if !slices.Contains(tables, match[1]) {
			tables = append(tables, match[1])
		}
	}

	slices.Sort(tables)

	return tables
}
