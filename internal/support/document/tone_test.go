package document

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// drawn inflates every stream of a PDF and joins them: the operators a page is
// drawn with. The colour of what is written can be read there, although the text
// itself is glyph numbers.
func drawn(t *testing.T, pdf []byte) string {
	t.Helper()

	var all strings.Builder

	for rest := pdf; ; {
		start := bytes.Index(rest, []byte("stream\n"))
		if start < 0 {
			break
		}

		rest = rest[start+len("stream\n"):]

		end := bytes.Index(rest, []byte("\nendstream"))
		if end < 0 {
			break
		}

		// A stream that is not deflated - none is, today - is not drawing.
		if inflated, err := zlib.NewReader(bytes.NewReader(rest[:end])); err == nil {
			read, _ := io.ReadAll(inflated)
			all.Write(read)
		}

		rest = rest[end+len("\nendstream"):]
	}

	return all.String()
}

// operator is how the writer sets c as the colour of what is written or filled
// next: one figure where the three channels agree, three where they do not.
func operator(c colour) string {
	channel := func(v int) string { return strconv.FormatFloat(float64(v)/255, 'f', 3, 64) }

	if c.r == c.g && c.g == c.b {
		return channel(c.r) + " g"
	}

	return fmt.Sprintf("%s %s %s rg", channel(c.r), channel(c.g), channel(c.b))
}

// A table is printed in the colours its cells had on the screen: a day's balance
// in credit green and in debt red, as the overtime screen shows them, a day with
// no working time recorded in the grey of what is quieter, and a limit of the
// Working Hours Act on its tint.
//
// The figures used to arrive as words and nothing else, so the page set every
// one of them in black - a column of balances whose sign was the only thing left
// saying which way each one went.
func TestEachCellIsPrintedInTheShadeItsToneSays(t *testing.T) {
	t.Parallel()

	write := func(tones [][]Tone) string {
		out, err := Write(Document{Title: "Überstunden", Sections: []Section{{Table: &Table{
			Columns: []string{"Tag", "Saldo"},
			Numeric: []bool{false, true},
			Rows: [][]string{
				{"01.11.2026", "+1,50 Std."}, {"02.11.2026", "-0,75 Std."},
				{"03.11.2026", "–"}, {"04.11.2026", "Mehr als zehn Stunden (§ 3 ArbZG)"},
			},
			Tones: tones,
		}}}})
		if err != nil {
			t.Fatalf("writing the document: %v", err)
		}

		return drawn(t, out)
	}

	plain := write(nil)
	toned := write([][]Tone{{"", Plus}, {"", Minus}, {Muted, Muted}, {"", Notice}})

	for _, want := range []struct {
		name  string
		shade colour
	}{
		{"a balance in credit", plusInk},
		{"a balance in debt", minusInk},
		{"a notice's tint", noticeFill},
	} {
		if strings.Contains(plain, operator(want.shade)) {
			t.Fatalf("a table with no tones already uses the shade of %s, so this case cannot tell", want.name)
		}

		if !strings.Contains(toned, operator(want.shade)) {
			t.Errorf("%s is not printed in %s", want.name, operator(want.shade))
		}
	}

	// The grey is the page's own, which the foot of the page is written in as
	// well, so it is counted rather than looked for.
	grey := "q " + operator(defaultMuted) + " BT"

	if more := strings.Count(toned, grey) - strings.Count(plain, grey); more != 2 {
		t.Errorf("the quieter day's two cells added %d figures written in the page's grey, want 2", more)
	}
}

// A figure of the summary in credit or in debt is printed in its shade rather
// than in the accent: the overtime screen shows its balance green or red, and the
// page printed it in the installation's blue whichever way it went.
func TestASummaryFigureInCreditOrDebtIsPrintedInItsShade(t *testing.T) {
	t.Parallel()

	write := func(tone Tone) string {
		out, err := Write(Document{
			Title:   "Überstunden",
			Summary: []Line{{Label: "Saldo", Value: "+12,25 Std.", Tone: tone}},
		})
		if err != nil {
			t.Fatalf("writing the document: %v", err)
		}

		return drawn(t, out)
	}

	if plain := write(""); strings.Contains(plain, operator(plusInk)) || strings.Contains(plain, operator(minusInk)) {
		t.Fatal("a summary that says nothing is printed in credit or debt, so this case cannot tell")
	}

	if !strings.Contains(write(Plus), operator(plusInk)) {
		t.Error("a balance in credit is not printed in its green")
	}

	if !strings.Contains(write(Minus), operator(minusInk)) {
		t.Error("a balance in debt is not printed in its red")
	}
}

// A total is printed as its screen shows it: one the screen sets quietly in its
// grey - the report's, the statistics', the working time's - is grey on paper,
// its name with it, and one the screen writes as plain text, as the statement of
// work does, is in the body's ink. Neither is in the accent.
//
// Every figure of the summary used to be printed in the accent, on the reasoning
// that the number the picture is about should be in the picture's colour. The
// maintainer asked on 2026-11-20 for the totals to look as they do on the screen,
// where none of them is blue.
func TestATotalIsPrintedAsItsScreenShowsIt(t *testing.T) {
	t.Parallel()

	write := func(tone Tone) string {
		out, err := Write(Document{
			Title:   "Auswertung",
			Summary: []Line{{Label: "Gesamt", Value: "7,50 Std.", Tone: tone}},
		})
		if err != nil {
			t.Fatalf("writing the document: %v", err)
		}

		return drawn(t, out)
	}

	accent := "q " + operator(defaultAccent) + " BT"
	grey := "q " + operator(defaultMuted) + " BT"

	plain, quiet := write(""), write(Muted)

	// The title is written in the accent either way, and only the title is.
	if got := strings.Count(plain, accent); got != 1 {
		t.Errorf("a plain total is written in the accent beside the title: %d writings in it, want the title's one", got)
	}

	if got := strings.Count(quiet, accent); got != 1 {
		t.Errorf("a quiet total is written in the accent beside the title: %d writings in it, want the title's one", got)
	}

	// The foot of the page is grey either way, so the grey is counted.
	if more := strings.Count(quiet, grey) - strings.Count(plain, grey); more != 2 {
		t.Errorf("a quiet total added %d writings in the page's grey, want two - its name and its figure", more)
	}
}

// A tone the page does not know is printed as none rather than refused: a
// document is not worth losing over a word a newer screen sent.
func TestAToneThePageDoesNotKnowIsPrintedAsNone(t *testing.T) {
	t.Parallel()

	out, err := Write(Document{
		Title:    "Überstunden",
		Sections: []Section{{Table: &Table{Columns: []string{"Saldo"}, Rows: [][]string{{"1,00"}}, Tones: [][]Tone{{"purple"}}}}},
		Summary:  []Line{{Label: "Saldo", Value: "1,00", Tone: "purple"}},
	})
	if err != nil {
		t.Fatalf("a tone nobody knows was refused: %v", err)
	}

	page := drawn(t, out)

	for _, shade := range []colour{plusInk, minusInk, noticeFill} {
		if strings.Contains(page, operator(shade)) {
			t.Errorf("a tone nobody knows was printed in %s", operator(shade))
		}
	}
}

// The shades a tone is printed in are the light screen's.
//
// The page cannot take them from the screen it was exported from, which may be
// dark and drawn for a dark ground, so it keeps its own - and this holds them to
// the stylesheet's light theme, the one drawn on white, so the two cannot drift.
// The grey of what is quieter is the page's own and is not held here: the page
// writes its captions in it, and the screen writes both in one grey as well.
func TestTheTonesArePrintedInTheLightScreensShades(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "interface", "web", "asset", "app.css"))
	if err != nil {
		t.Fatalf("reading the stylesheet: %v", err)
	}

	css := string(raw)

	// The first :root block is the light theme: what a page is drawn in before
	// any script has run, and what [data-theme="light"] repeats.
	start := strings.Index(css, "\n:root {")
	if start < 0 {
		t.Fatal("the stylesheet has no :root block any more; this test is out of date")
	}

	light := css[start : start+strings.Index(css[start:], "\n}")]

	hex := func(where, pattern string) colour {
		t.Helper()

		match := regexp.MustCompile(pattern).FindStringSubmatch(where)
		if match == nil {
			t.Fatalf("the stylesheet no longer says %s; this test is out of date", pattern)
		}

		return parseColour(match[1], colour{-1, -1, -1})
	}

	if got := hex(css, `\.plus \{ color: (#[0-9a-f]{6});`); got != plusInk {
		t.Errorf("the screen draws a balance in credit in %v and the page prints it in %v", got, plusInk)
	}

	if !strings.Contains(css, ".minus { color: var(--danger);") {
		t.Fatal("a balance in debt is no longer drawn in the danger colour; this test is out of date")
	}

	if got := hex(light, `--danger: (#[0-9a-f]{6});`); got != minusInk {
		t.Errorf("the light screen draws a balance in debt in %v and the page prints it in %v", got, minusInk)
	}

	notice := regexp.MustCompile(`\.act-notice \{[^}]*background: color-mix\(in srgb, (#[0-9a-f]{6}) (\d+)%, var\(--surface\)\);`).
		FindStringSubmatch(css)
	if notice == nil {
		t.Fatal("a notice is no longer tinted by mixing a hue into the surface; this test is out of date")
	}

	hue := parseColour(notice[1], colour{-1, -1, -1})
	surface := hex(light, `--surface: (#[0-9a-f]{6});`)
	share, _ := strconv.Atoi(notice[2])

	mix := func(a, b int) int { return int(math.Round(float64(a*share+b*(100-share)) / 100)) }

	if got := (colour{mix(hue.r, surface.r), mix(hue.g, surface.g), mix(hue.b, surface.b)}); got != noticeFill {
		t.Errorf("the light screen tints a notice %v and the page prints it on %v", got, noticeFill)
	}
}
