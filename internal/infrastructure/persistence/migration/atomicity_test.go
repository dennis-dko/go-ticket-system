package migration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"slices"
	"sort"
	"testing"
)

// A migration that changes the schema and the data is not atomic on MySQL.
//
// GoFr runs each migration inside a transaction - migration.Datasource.SQL *is*
// the *sql.Tx - so every statement in one, a seed's row-at-a-time work included,
// commits or rolls back together. On PostgreSQL and SQLite that is the end of it.
//
// MySQL commits implicitly on DDL. A migration that alters a table and then
// updates rows therefore runs its UPDATE outside the transaction the migration
// began: the schema change has already landed, and if the data change fails,
// nothing goes back. The migration is recorded as failed with half of it on disk.
//
// What that costs is not abstract. A column added so that existing rows can be
// given a value - when a session was last used, say, filled from when it was
// opened so that nobody is signed out by an update - is exactly the migration
// that mixes the two, and split on MySQL it becomes the cause of what it was
// written to prevent.
//
// No migration of this chain mixes them, and the list below is empty for that
// reason rather than for want of looking: the chain this one replaced carried six
// that had run everywhere before the rule was written. A migration that needs
// both is two migrations - the schema in one, the data in the next - which is
// CLAUDE.md §7's multi-phase rule applied to the engine rather than to the
// deployment. It is atomic on every engine only if the schema half is a single
// statement; the case below holds that.
func TestNoNewMigrationMixesSchemaAndDataChanges(t *testing.T) {
	// A migration is never taken off this list by editing it once it has shipped.
	alreadyMixed := []string{}

	chain := readChain(t)

	entries := chain.calls["All"]
	if len(entries) == 0 {
		t.Fatal("no migrations found through All; this test is reading nothing")
	}

	var mixed []string

	for _, name := range entries {
		text := chain.reach(name)

		if changesSchema.MatchString(text) && changesData.MatchString(text) {
			mixed = append(mixed, name)
		}
	}

	sort.Strings(mixed)

	for _, name := range mixed {
		if !slices.Contains(alreadyMixed, name) {
			t.Errorf("the migration %s changes the schema and the data, which MySQL "+
				"cannot roll back as a unit: its DDL commits implicitly, so the data "+
				"change after it runs outside the migration's transaction and a "+
				"failure leaves half of it applied. Split it into two migrations, "+
				"schema first", name)
		}
	}

	for _, name := range alreadyMixed {
		if !slices.Contains(mixed, name) {
			t.Errorf("%s is listed here as already mixing the two and no longer "+
				"does; if it was split, take it off the list - and if it was edited "+
				"in place, that is the one thing an applied migration must never be",
				name)
		}
	}
}

var (
	changesSchema = regexp.MustCompile(`\b(CREATE TABLE|ALTER TABLE|DROP TABLE|CREATE INDEX|DROP INDEX)\b`)
	changesData   = regexp.MustCompile(`\b(INSERT INTO|UPDATE [a-z_]+ SET|DELETE FROM)\b`)
)

// chain is the migration file: what each function says, and what it calls.
type chain struct {
	source map[string]string
	calls  map[string][]string

	// schema is how many string literals in each function name a schema
	// change, which is how many schema statements it can run.
	schema map[string]int
}

// schemaStatements is how many schema statements a migration and everything it
// calls hold between them.
func (c chain) schemaStatements(name string) int {
	seen := map[string]bool{}

	var walk func(string) int

	walk = func(at string) int {
		if seen[at] {
			return 0
		}

		seen[at] = true
		count := c.schema[at]

		for _, next := range c.calls[at] {
			count += walk(next)
		}

		return count
	}

	return walk(name)
}

// reach is a function's own source plus that of everything it calls, so a
// migration whose statements live in a helper is still seen whole.
//
// Calls come from the syntax tree rather than from a text search, or a name
// mentioned in a comment or inside a SQL string would count as a call - and All,
// which names every migration there is, would then reach all of them and report
// the whole chain as one mixed migration.
func (c chain) reach(name string) string {
	seen := map[string]bool{}

	var walk func(string) string

	walk = func(at string) string {
		if seen[at] || c.source[at] == "" {
			return ""
		}

		seen[at] = true
		out := c.source[at]

		for _, next := range c.calls[at] {
			out += "\n" + walk(next)
		}

		return out
	}

	return walk(name)
}

func readChain(t *testing.T) chain {
	t.Helper()

	const path = "migration.go"

	fset := token.NewFileSet()

	parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	out := chain{source: map[string]string{}, calls: map[string][]string{}, schema: map[string]int{}}

	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil {
			continue
		}

		from := fset.Position(fn.Pos()).Offset
		to := fset.Position(fn.End()).Offset
		out.source[fn.Name.Name] = string(body[from:to])

		seen := map[string]bool{}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if lit, isLit := node.(*ast.BasicLit); isLit && lit.Kind == token.STRING &&
				changesSchema.MatchString(lit.Value) {
				out.schema[fn.Name.Name]++
			}

			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}

			if named, isNamed := call.Fun.(*ast.Ident); isNamed && !seen[named.Name] {
				seen[named.Name] = true

				out.calls[fn.Name.Name] = append(out.calls[fn.Name.Name], named.Name)
			}

			return true
		})
	}

	// Nothing reaches back into All, or every migration would appear to contain
	// every other one.
	delete(out.source, "All")

	return out
}

// A new migration runs at most one schema statement.
//
// The case above holds a narrower rule than MySQL does. MySQL commits every DDL
// statement on its own, so a migration with two of them is not atomic either,
// data or no data: measured against the test environment's MySQL 8.4, a
// transaction that created a table and then failed on a second CREATE TABLE
// rolled back and left the first table in place. GoFr records a migration in the
// same transaction as its statements, so the record is gone and the table is
// not, and the next start runs the migration again and stops on "already
// exists" - an installation that cannot start until somebody cleans its database
// by hand. PostgreSQL and SQLite roll DDL back, which is why nothing else shows
// it.
//
// So a new migration carries one schema statement, and a second change is a
// second migration - an index is one of its own, made by createIndex, which is
// why All calls that one so often. Counted as string literals in the syntax
// tree, not in the text, so a comment naming a statement does not count; a
// helper the migration calls is counted with it. A switch that gives each
// dialect its own statement counts as several here although only one runs -
// build the one statement from the dialect instead, as the column types already
// are.
func TestNoNewMigrationRunsMoreThanOneSchemaStatement(t *testing.T) {
	// Empty, and a migration is never taken off this list by editing it once it
	// has shipped.
	alreadyMultiple := []string{}

	chain := readChain(t)

	entries := chain.calls["All"]
	if len(entries) == 0 {
		t.Fatal("no migrations found through All; this test is reading nothing")
	}

	var multiple []string

	for _, name := range entries {
		if chain.schemaStatements(name) > 1 {
			multiple = append(multiple, name)
		}
	}

	for _, name := range multiple {
		if !slices.Contains(alreadyMultiple, name) {
			t.Errorf("the migration %s runs %d schema statements. MySQL commits each "+
				"one by itself, so a failure after the first leaves it applied with "+
				"the migration unrecorded, and the next start fails on it. Make it "+
				"one statement per migration", name, chain.schemaStatements(name))
		}
	}

	for _, name := range alreadyMultiple {
		if !slices.Contains(multiple, name) {
			t.Errorf("%s is listed as running several schema statements and no "+
				"longer does; if it was edited in place, that is the one thing an "+
				"applied migration must never be", name)
		}
	}
}
