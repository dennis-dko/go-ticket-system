package migration_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/migration"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/sqldb"
)

// released is every version this chain had shipped when this test was last
// brought up to date. It is a floor rather than a register: a version missing
// from it is fine as long as it comes after all of these.
//
// It is the whole chain of the first release, because a merge to main cuts a
// release: whatever the chain holds when that merge lands has shipped.
var released = []int64{
	20261112010000, 20261112010100, 20261112010200, 20261112010300, 20261112010400,
	20261112010500, 20261112010600, 20261112010700, 20261112010800, 20261112010900,
	20261112011000, 20261112011100, 20261112011200, 20261112011300, 20261112011400,
	20261112011500, 20261112011600, 20261112011700, 20261112011800, 20261112011900,
	20261112012000, 20261112012100, 20261112012200, 20261112012300, 20261112012400,
	20261112012500, 20261112012600, 20261112012700, 20261112012800, 20261112012900,
	20261112013000, 20261112013100, 20261112013200, 20261112013300, 20261112013400,
	20261112013500, 20261112013600, 20261112013700, 20261112013800, 20261112013900,
	20261112014000, 20261112014100, 20261112014200, 20261112014300, 20261112014400,
	20261112014500, 20261112014600, 20261112014700, 20261112014800, 20261112014900,
	20261112015000,
}

// A migration added to the chain comes after every one before it.
//
// GoFr runs only the versions above the highest one an installation has already
// applied. It names the ones below it that never ran, at ERROR on every start,
// and still does not run them. So a migration added later with an older version
// is run by every fresh installation and skipped by every one that already has
// the newer version: the two end with different schemas, and the line that says
// so is read, if at all, once an update has already parted them. CLAUDE.md says to add to the end, and until this
// test that was a sentence. It is the shape parallel branches produce - each
// names its migration after the day it was started, and whichever merges second
// is the older one.
//
// Three checks, because each catches what the others cannot. The versions in
// the source are strictly increasing, which catches one appended with an older
// number. Every released version is still there, which is what append-only
// means. And every version this list has not seen is newer than all of it,
// which catches one slipped into the middle in its sorted place.
func TestEveryNewMigrationComesAfterEveryReleasedOne(t *testing.T) {
	inSource := versionsInSource(t)

	for i := 1; i < len(inSource); i++ {
		if inSource[i] <= inSource[i-1] {
			t.Errorf("migration %d follows %d in the chain; every installation that has "+
				"already run %d would skip it", inSource[i], inSource[i-1], inSource[i-1])
		}
	}

	chain := migration.All(sqldb.DialectSQLite)

	for _, version := range released {
		if _, ok := chain[version]; !ok {
			t.Errorf("released migration %d is gone from the chain, which is append-only", version)
		}
	}

	newest := slices.Max(released)

	for version := range chain {
		if !slices.Contains(released, version) && version <= newest {
			t.Errorf("migration %d is not among the released ones and is not newer than %d, "+
				"so every installation that has run %d would skip it", version, newest, newest)
		}
	}

	if len(chain) != len(inSource) {
		t.Errorf("the chain has %d migrations and the source lists %d; this test is reading "+
			"the wrong literal", len(chain), len(inSource))
	}
}

// versionsInSource reads the keys of the map All returns, in the order they are
// written.
func versionsInSource(t *testing.T) []int64 {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "migration.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing migration.go: %v", err)
	}

	var versions []int64

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "All" {
			return true
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			literal, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}

			if _, isMap := literal.Type.(*ast.MapType); !isMap {
				return true
			}

			for _, element := range literal.Elts {
				kv, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}

				key, ok := kv.Key.(*ast.BasicLit)
				if !ok || key.Kind != token.INT {
					t.Fatalf("a key of the chain is not a literal version: %T", kv.Key)
				}

				version, err := strconv.ParseInt(key.Value, 10, 64)
				if err != nil {
					t.Fatalf("reading version %s: %v", key.Value, err)
				}

				versions = append(versions, version)
			}

			return false
		})

		return false
	})

	if len(versions) == 0 {
		t.Fatal("no versions were read from All; this test is reading nothing")
	}

	return versions
}
