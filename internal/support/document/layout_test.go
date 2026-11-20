package document

import (
	"strings"
	"testing"
)

// A narrow column keeps its room beside a wide one.
//
// The width used to be shared out in proportion to what each column held, so a
// table with one long column scaled every other column down with it: in the
// working time's document a day carrying a limit of the Working Hours Act cut
// every date to "01.11…" and every time to "0…" - the figures the page is for,
// given up to make room for a sentence that could have folded.
func TestANarrowColumnKeepsItsRoomBesideAWideOne(t *testing.T) {
	t.Parallel()

	row := []string{"01.11.2026", "07:00", "19:00", "15", "11,75 Std.", "0,00 Std.",
		"Mehr als zehn Stunden (§ 3 ArbZG) · 45 Minuten Pause nötig (§ 4 ArbZG)"}

	table := Table{
		Columns: []string{"Datum", "Start", "Ende", "Pausen", "Gearbeitet", "Gebucht", "Arbeitszeitgesetz"},
		Numeric: []bool{false, false, false, true, true, true, false},
		Rows:    [][]string{row},
	}

	pdf := paper()
	widths := columnWidths(pdf, table)

	pdf.SetFont("go", "", bodySize)

	for i, cell := range row[:len(row)-1] {
		if lines := pdf.SplitText(cell, widths[i]); len(lines) != 1 {
			t.Errorf("%q does not fit its column of %.1f mm and is broken as %q", cell, widths[i], lines)
		}
	}

	total := 0.0
	for _, width := range widths {
		total += width
	}

	if total < contentWide-0.01 || total > contentWide+0.01 {
		t.Errorf("the columns are %.2f mm wide together, want the page's %.2f", total, contentWide)
	}
}

// A cell longer than its column folds onto more lines rather than being cut.
//
// Read off the page's operators: every line of text is one, so a cell that folds
// onto four lines writes three more than a cell of one word does.
func TestALongCellFoldsOntoMoreLines(t *testing.T) {
	t.Parallel()

	lines := func(text string) int {
		out, err := Write(Document{Title: "Arbeitszeit", Sections: []Section{{Table: &Table{
			Columns: []string{"Datum", "Arbeitszeitgesetz"},
			Rows:    [][]string{{"01.11.2026", text}},
		}}}})
		if err != nil {
			t.Fatalf("writing the document: %v", err)
		}

		return strings.Count(drawn(t, out), "Tj ET")
	}

	long := strings.Repeat("Mehr als zehn Stunden (§ 3 ArbZG) · ", 8)

	if more := lines(long) - lines("§ 3"); more < 2 {
		t.Errorf("a cell of %d characters took %d more line(s) than one of three, want it folded "+
			"onto several", len([]rune(long)), more)
	}
}

// A cell that would fold past a whole page is cut after its last line instead,
// with an ellipsis saying so: a row taller than the paper cannot be printed.
func TestACellStopsFoldingAtItsLastLine(t *testing.T) {
	t.Parallel()

	pdf := paper()
	pdf.SetFont("go", "", bodySize)

	folded := fold(pdf, strings.Repeat("ä", 4000), 60)

	if len(folded) != maxCellLines {
		t.Fatalf("4000 letters folded onto %d lines, want them stopped at %d", len(folded), maxCellLines)
	}

	if last := folded[len(folded)-1]; !strings.HasSuffix(last, "…") {
		t.Errorf("the last line reads %q, want it to end in an ellipsis", last)
	}

	if short := fold(pdf, "kurz", 60); len(short) != 1 || short[0] != "kurz" {
		t.Errorf("a short cell folded as %q", short)
	}
}
