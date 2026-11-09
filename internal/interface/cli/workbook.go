package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/dennis-dko/go-time-recording/internal/support/i18n"
)

// The spreadsheet card, as the page keeps it under its list: the entries
// exported as the list narrows them, and a file of entries imported - checked
// first, as the page checks it, and written only once every row can be. The
// file is sent twice, once to look and once to write, because the installation
// keeps no half-finished import between two requests.

// importMost is the largest file the installation takes for an import, its
// maxImportBody. A larger one is refused here, before it is read whole only to
// be refused there.
const importMost = 32 << 20

// mostRowsShown is how many of a checked file's rows the card shows, the
// refused ones first: the summary above them says how many there are.
const mostRowsShown = 10

// importPlan is a file as the installation understood it: each row as it would
// be written or why it cannot be, how many of each, and how many entries were
// made where it was written. name and content are the file, kept to be sent
// again.
type importPlan struct {
	name    string
	content []byte

	Rows     []importRow `json:"rows"`
	Writable int         `json:"writable"`
	Rejected int         `json:"rejected"`
	Imported int         `json:"imported"`
}

// importRow is one row of a file, as the installation read it.
type importRow struct {
	Row           int     `json:"row"`
	Date          *string `json:"date"`
	User          string  `json:"user"`
	Project       string  `json:"project"`
	Hours         float64 `json:"hours"`
	Description   string  `json:"description"`
	Problem       string  `json:"problem"`
	ProblemCode   string  `json:"problemCode"`
	ProblemValues []any   `json:"problemValues"`
}

// checked is what became of a file sent to be checked or written.
type checked struct {
	plan    importPlan
	written bool
	err     error
}

// workbookChoices are the spreadsheet card's menu: the export, and for an
// account that may write its time the import - a file to check, and once one is
// checked, writing it where every row can be, another file, or none.
func (m screen) workbookChoices() []choice {
	l := m.s.lang
	choices := []choice{{"e", "export", l.text("wb.export", "Export as .xlsx")}}

	switch {
	case !m.mayWrite():
	case m.plan == nil:
		choices = append(choices, choice{"i", "import", l.text("track.wb.import", "Import a file")})
	default:
		if m.plan.Rejected == 0 && m.plan.Writable > 0 {
			choices = append(choices, choice{"i", "write", l.text("wb.import", "Import")})
		}

		choices = append(choices,
			choice{"o", "import", l.text("track.wb.another", "Choose another file")},
			choice{"c", "drop", l.text("track.wb.drop", "Put the file away")})
	}

	return choices
}

// exportName is the name the page gives an export: its word for the file and
// today, in the owner's zone.
func (m screen) exportName() string {
	return m.s.lang.text("wb.filename", "time-entries") + "-" + m.now.In(m.shown.zone).Format(time.DateOnly) + ".xlsx"
}

// fileStep asks where the export goes, or which file is to be imported.
func (m screen) fileStep(doing string, a *answers) *huh.Group {
	l := m.s.lang

	if doing == "export" {
		return huh.NewGroup(huh.NewInput().Key("file").Title(l.text("track.ask.exportFile", "Where should the file go?")).
			Validate(l.writableFile).Value(&a.file))
	}

	return huh.NewGroup(huh.NewInput().Key("file").Title(l.text("track.ask.importFile", "Which file should be checked?")).
		Placeholder(l.text("track.wb.path", "the path to an .xlsx file")).Validate(l.readableFile).Value(&a.file))
}

// exportTo saves the entries as the card narrows them, in the client's language,
// to the file named - which was checked not to be there yet, and is still not
// written over if it has come since.
func (m screen) exportTo() tea.Cmd {
	ctx, s, path := m.ctx, m.s, strings.TrimSpace(m.asked.file)

	q := m.card.filter.query()
	q.Set("lang", s.lang.code())

	return func() tea.Msg {
		book, err := s.api.fetch(ctx, "/timesheets/export?"+q.Encode())
		if err != nil {
			return done{err: err}
		}

		return s.saved(path, book)
	}
}

// saved writes a file an export came back as to a path that is not taken - never
// over a file, not even one that has come since the path was asked for - and says
// where it went.
func (s *session) saved(path string, content []byte) done {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return done{err: err}
	}

	_, err = file.Write(content)
	if closed := file.Close(); err == nil {
		err = closed
	}

	if err != nil {
		return done{err: err}
	}

	if whole, err := filepath.Abs(path); err == nil {
		path = whole
	}

	return done{said: s.lang.text("wb.exported", "Export saved") + " – " + path}
}

// checkFile reads the file named and sends it to be checked.
func (m screen) checkFile() tea.Cmd {
	ctx, s, path := m.ctx, m.s, strings.TrimSpace(m.asked.file)

	return func() tea.Msg {
		content, err := readImport(path)
		if err != nil {
			return checked{err: err}
		}

		plan := importPlan{name: filepath.Base(path), content: content}
		err = s.api.upload(ctx, "/timesheets/import?lang="+s.lang.code(), plan.name, content,
			map[string]string{"dryRun": "true"}, &plan)

		return checked{plan: plan, err: err}
	}
}

// writeFile sends the file checked again, to be written.
func (m screen) writeFile() tea.Cmd {
	ctx, s, plan := m.ctx, m.s, *m.plan

	return func() tea.Msg {
		written := importPlan{name: plan.name, content: plan.content}
		err := s.api.upload(ctx, "/timesheets/import?lang="+s.lang.code(), plan.name, plan.content,
			map[string]string{"dryRun": "false"}, &written)

		return checked{plan: written, written: true, err: err}
	}
}

// took takes in what became of a file: its check shown, or what writing it made
// said and the day read again.
func (m screen) took(msg checked) (screen, tea.Cmd) {
	l := m.s.lang
	m.busy = false

	switch {
	case msg.err != nil:
		m.news, m.failure = "", msg.err.Error()

		return m, nil
	case msg.written:
		m.plan, m.failure = nil, ""
		m.news = l.text("wb.imported", "The file was imported") + ". " +
			l.text("wb.importedCount", "{0} entries created.", strconv.Itoa(msg.plan.Imported))

		return m.ask(m.now)
	}

	m.plan = &msg.plan

	return m, nil
}

// workbookCardView draws the spreadsheet card: what it exports and imports, and
// a file checked for an import - how many of its rows can be written, and the
// rows, the refused ones first, each with why.
func (m screen) workbookCardView(look palette, width int) string {
	l := m.s.lang
	wrapped := lipgloss.NewStyle().Width(width)

	lines := []string{look.title.Render(l.text("wb.title", "Spreadsheet")),
		wrapped.Render(look.muted.Render(l.text("track.wb.text",
			"Exports the entries as the entries card narrows them: {0}. An import creates entries; it never changes or replaces the ones that are already there.",
			m.filterSummary())))}

	if m.plan == nil {
		return strings.Join(lines, "\n")
	}

	p := m.plan

	summary := l.text("wb.allReady", "All {0} rows can be imported.", strconv.Itoa(p.Writable))

	switch {
	case len(p.Rows) == 0:
		summary = l.text("wb.empty", "The file has no entries in it.")
	case p.Rejected > 0:
		summary = l.text("wb.someRejected", "{0} of {1} rows can be imported. Nothing is written while any row is refused.",
			strconv.Itoa(p.Writable), strconv.Itoa(len(p.Rows)))
	}

	lines = append(lines, wrapped.Render(look.strong.Render(p.name)+": "+summary))

	if len(p.Rows) == 0 {
		return strings.Join(lines, "\n")
	}

	// The page's columns but its last: why a row is refused is the one thing
	// that must be read whole, so it stands under its row rather than in a
	// column cut short at the edge.
	table := grid{
		headings: []string{l.text("wb.row", "Row"), l.text("field.date", "Date"), l.text("field.user", "User"),
			l.text("field.project", "Project"), l.text("field.hours", "Hours"), l.text("field.description", "Description")},
		right: []bool{true, false, false, false, true},
		most:  []int{0, 0, 4, 5},
	}

	rows := refusedFirst(p.Rows)
	rows = rows[:min(len(rows), mostRowsShown)]

	for _, r := range rows {
		day := "–"
		if r.Date != nil {
			day = l.date(*r.Date)
		}

		table.rows = append(table.rows, []string{strconv.Itoa(r.Row), day, r.User, r.Project, l.number(r.Hours), r.Description})
	}

	lines = append(lines, table.heading(look, width))
	why := lipgloss.NewStyle().Width(width).PaddingLeft(2)

	for i, line := range table.lines(width) {
		lines = append(lines, line)

		if rows[i].Problem != "" {
			lines = append(lines, why.Render(look.failed.Render("✗ "+l.rowProblem(rows[i]))))
		}
	}

	if len(p.Rows) > mostRowsShown {
		lines = append(lines, look.muted.Render("…"))
	}

	return strings.Join(lines, "\n")
}

// refusedFirst are a file's rows, the refused ones first and each kept in its
// place among its kind.
func refusedFirst(rows []importRow) []importRow {
	var refused, fine []importRow

	for _, r := range rows {
		if r.Problem != "" {
			refused = append(refused, r)
		} else {
			fine = append(fine, r)
		}
	}

	return append(refused, fine...)
}

// rowProblem says why a row cannot be written, as the page's rowProblem says it:
// a sentence of the row's own where the dictionary has one, the refusal's where
// it has that, and the installation's English otherwise; a row refused for the
// shape of its values names the fields in the page's words.
func (l language) rowProblem(r importRow) string {
	if r.ProblemCode == "" {
		return r.Problem
	}

	if r.ProblemCode == "invalidFields" {
		if sentence, ok := l.words.Lookup("err.invalidFields"); ok {
			named := make([]string, 0, len(r.ProblemValues))

			for _, field := range r.ProblemValues {
				if name, ok := field.(string); ok {
					named = append(named, l.pageWord("field.", name))
				}
			}

			return sentence + ": " + strings.Join(named, ", ")
		}
	}

	for _, key := range []string{"row." + r.ProblemCode, "err." + r.ProblemCode} {
		if sentence, ok := l.words.Lookup(key); ok {
			return i18n.Fill(sentence, l.refusalValues(r.ProblemCode, r.ProblemValues)...)
		}
	}

	return r.Problem
}

// readImport reads a file to be imported, refusing one larger than the
// installation takes before reading it whole.
func readImport(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() { _ = file.Close() }()

	return io.ReadAll(io.LimitReader(file, importMost+1))
}

// writableFile refuses a name an export cannot be saved under: one that is
// there already, which is not written over, or one in a folder that is not.
func (l language) writableFile(raw string) error {
	path := strings.TrimSpace(raw)
	if path == "" {
		return errors.New(l.text("track.wb.noName", "name the file"))
	}

	if _, err := os.Stat(path); err == nil {
		return errors.New(l.text("track.wb.exists", "{0} is there already", path))
	}

	if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
		return errors.New(l.text("track.wb.noFolder", "there is no folder {0}", filepath.Dir(path)))
	}

	return nil
}

// readableFile refuses a name that is no file, or one larger than the
// installation takes for an import.
func (l language) readableFile(raw string) error {
	info, err := os.Stat(strings.TrimSpace(raw))

	switch {
	case err != nil || info.IsDir():
		return errors.New(l.text("track.wb.noFile", "there is no file {0}", strings.TrimSpace(raw)))
	case info.Size() > importMost:
		return errors.New(l.text("track.wb.tooLarge", "the file is larger than the {0} MB the installation takes",
			strconv.Itoa(importMost>>20)))
	}

	return nil
}
