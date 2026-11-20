package rest

import (
	"reflect"
	"testing"

	"github.com/dennis-dko/timepulse/internal/support/document"
)

// What the screen says with a cell's colour reaches the document beside the cell,
// and so does what it says with a figure's: the overtime screen's balances, green
// in credit and red in debt, and its total. A tone past the last cell of its row
// colours nothing and is not kept.
func TestADocumentRequestCarriesWhatTheScreensColoursSay(t *testing.T) {
	t.Parallel()

	request := DocumentRequest{
		Title: "Überstunden",
		Sections: []DocumentSectionRequest{{Table: &DocumentTableRequest{
			Columns: []string{"Tag", "Saldo"},
			Rows:    [][]string{{"01.11.2026", "+1,50"}, {"02.11.2026", "-0,75"}, {"03.11.2026", "0,00"}},
			Tones:   [][]string{{"", "plus"}, {"", "minus", "plus"}},
		}}},
		Summary: []document.Line{{Label: "Saldo", Value: "+0,75", Tone: document.Plus}},
	}

	doc, err := request.document()
	if err != nil {
		t.Fatalf("the request was refused: %v", err)
	}

	want := [][]document.Tone{{"", document.Plus}, {"", document.Minus}, nil}

	if got := doc.Sections[0].Table.Tones; !reflect.DeepEqual(got, want) {
		t.Errorf("the table's tones arrived as %q, want %q", got, want)
	}

	if got := doc.Summary[0].Tone; got != document.Plus {
		t.Errorf("the balance's tone arrived as %q, want %q", got, document.Plus)
	}
}
