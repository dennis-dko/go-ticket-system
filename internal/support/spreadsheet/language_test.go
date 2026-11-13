package spreadsheet_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/dennis-dko/go-ticket-system/internal/support/spreadsheet"
)

// A workbook exported in one language can be imported again.
//
// The point of the whole exercise: translating the headings is only worth doing if
// the file stays usable, and the failure would be silent - a German export that
// looks perfect and is refused, or worse read one column out of step. Every
// language is checked rather than the one someone remembered to write a case for.
//
// Name, mail address, role and whether the password lives in the directory go out,
// and the last of them does not come back: an account cannot be moved into LDAP by
// editing a cell.
func TestAnExportInAnyLanguageReadsBackUnchanged(t *testing.T) {
	t.Parallel()

	for _, language := range append(spreadsheet.Languages(), "", "en", "de-DE") {
		t.Run("language="+language, func(t *testing.T) {
			t.Parallel()

			written, err := spreadsheet.WriteUsers(language, []spreadsheet.UserRow{{
				Name: "Vera", Email: "vera@example.test", Role: "agent",
				Directory: true,
			}})
			if err != nil {
				t.Fatalf("writing in %q: %v", language, err)
			}

			rows, problems, err := spreadsheet.ReadUsers(bytes.NewReader(written))
			if err != nil {
				t.Fatalf("reading back a %q workbook: %v", language, err)
			}

			if len(problems) != 0 {
				t.Fatalf("a workbook this package wrote in %q came back with problems: %v",
					language, problems)
			}

			if len(rows) != 1 {
				t.Fatalf("got %d rows back from a %q workbook, want 1", len(rows), language)
			}

			got := rows[0]

			if got.Email != "vera@example.test" || got.Name != "Vera" || got.Role != "agent" {
				t.Errorf("a column moved: %+v", got)
			}

			if got.Directory {
				t.Error("the directory column was read back, which would let a " +
					"spreadsheet move an account into the directory")
			}
		})
	}
}

// The German export actually says the German words, in its headings and in its
// cells.
//
// Without this the round-trip test above would still pass if translation were
// quietly doing nothing at all.
func TestTheGermanExportIsInGerman(t *testing.T) {
	t.Parallel()

	written, err := spreadsheet.WriteUsers("de", []spreadsheet.UserRow{{
		Name: "Vera", Email: "vera@example.test", Role: "agent", Directory: true,
	}})
	if err != nil {
		t.Fatalf("writing: %v", err)
	}

	book, err := excelize.OpenReader(bytes.NewReader(written))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}

	defer func() { _ = book.Close() }()

	sheets := book.GetSheetList()
	if len(sheets) != 1 || sheets[0] != "Benutzer" {
		t.Errorf("sheets are %v, want exactly [Benutzer]", sheets)
	}

	want := []string{"Name", "E-Mail", "Rolle", "Verzeichnis"}

	if got := spreadsheet.UserColumns("de"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("German columns are %v, want %v", got, want)
	}

	for i, heading := range append(want, "ja") {
		row := 1
		if i == len(want) {
			// The directory flag of the one account, under its heading.
			i, row = len(want)-1, 2
		}

		cell, nameErr := excelize.CoordinatesToCellName(i+1, row)
		if nameErr != nil {
			t.Fatalf("naming a cell: %v", nameErr)
		}

		got, readErr := book.GetCellValue(sheets[0], cell)
		if readErr != nil {
			t.Fatalf("reading %s: %v", cell, readErr)
		}

		if got != heading {
			t.Errorf("%s is %q, want %q", cell, got, heading)
		}
	}
}

// A workbook of one kind is not silently read as another.
//
// Each screen has its own import. Handing the people importer a sheet of roles
// has to fail loudly rather than match accounts on a column of descriptions.
func TestASheetOfTheWrongKindIsRefused(t *testing.T) {
	t.Parallel()

	roles, err := spreadsheet.WriteRoles("", []string{"tickets:read"}, []spreadsheet.RoleRow{{
		Name: "agent", Description: "works on tickets", Granted: []string{"tickets:read"},
	}})
	if err != nil {
		t.Fatalf("writing: %v", err)
	}

	rows, problems, err := spreadsheet.ReadUsers(bytes.NewReader(roles))
	if err != nil {
		// Refusing the file outright is a fine answer too.
		return
	}

	// It read something. Then every row of it has to be a complaint, not an
	// account: the mail column of a sheet of roles holds a description.
	if len(rows) > 0 {
		t.Errorf("a sheet of roles produced %d user rows: %+v", len(rows), rows)
	}

	if len(problems) == 0 {
		t.Error("a sheet of roles was read as people without a single complaint")
	}
}
