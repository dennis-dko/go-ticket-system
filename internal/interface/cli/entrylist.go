package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// listPage is how many entries the card asks for at a time: the page's own
// TIMESHEET_PAGE, a screenful rather than everything an account ever booked.
const listPage = 25

// listMost is the most entries the card holds. Reading the day again reads every
// entry already shown, from the first, so that the list keeps its place, and the
// API refuses a page larger than its MaxPageSize rather than shortening it.
const listMost = 500

// listFewest is the fewest entries the card shows, however short the window:
// fewer than that is not a list anybody can read.
const listFewest = 3

// subEntries names the entries card's menu, opened from the time entries' own.
const subEntries = "entries"

// entryFilter is what the entries card is narrowed to, as the page's filters
// narrow its list: the first and the last day as YYYY-MM-DD, words searched for
// in the descriptions, and a project - its id, "none" for the entries without
// one, or empty for every project. An empty field narrows nothing.
type entryFilter struct {
	from, to, search, project string
}

// query is the filter as the list's query string.
func (f entryFilter) query() url.Values {
	q := url.Values{}

	for key, value := range map[string]string{"from": f.from, "to": f.to, "q": f.search, "projectId": f.project} {
		if value != "" {
			q.Set(key, value)
		}
	}

	return q
}

// entryCard is the entries card: what it is narrowed to, the entries read of
// what that finds, how many it finds altogether, and how many of the account's
// entries wait for a project.
type entryCard struct {
	filter     entryFilter
	items      []entry
	total      int
	unassigned int
}

// entryPage is a page of entries as the list answers with it.
type entryPage struct {
	Items      []entry `json:"items"`
	TotalCount int     `json:"totalCount"`
}

// morePage is the next page of the entries card, read for the filter it was
// asked with from where the card ended then.
type morePage struct {
	filter entryFilter
	from   int
	page   entryPage
	err    error
}

// entryList reads count of the entries the filter finds, from offset on.
func (s *session) entryList(ctx context.Context, filter entryFilter, offset, count int) (entryPage, error) {
	q := filter.query()
	q.Set("limit", strconv.Itoa(count))
	q.Set("offset", strconv.Itoa(offset))

	var page entryPage

	err := s.api.call(ctx, http.MethodGet, "/timesheets?"+q.Encode(), nil, &page)

	return page, err
}

// readCard reads the entries card again: as many entries as it shows, from the
// first, and how many of the account's entries wait for a project.
func (s *session) readCard(ctx context.Context, filter entryFilter, shown int) (entryCard, error) {
	page, err := s.entryList(ctx, filter, 0, min(max(shown, listPage), listMost))
	if err != nil {
		return entryCard{}, err
	}

	waiting, err := s.entryList(ctx, entryFilter{project: "none"}, 0, 1)
	if err != nil {
		return entryCard{}, err
	}

	return entryCard{filter: filter, items: page.Items, total: page.TotalCount, unassigned: waiting.TotalCount}, nil
}

// more reads the next page of the entries card.
func (m screen) more() tea.Cmd {
	ctx, s, filter, from := m.ctx, m.s, m.card.filter, len(m.card.items)

	return func() tea.Msg {
		page, err := s.entryList(ctx, filter, from, listPage)

		return morePage{filter: filter, from: from, page: page, err: err}
	}
}

// addPage puts the next page under the entries already shown, and moves the
// card to its first entry. A page asked for under another filter, or from
// another place, belongs to a list that is gone and is dropped.
func (m screen) addPage(msg morePage) screen {
	m.busy = false

	switch {
	case msg.err != nil:
		m.news, m.failure = "", msg.err.Error()
	case msg.filter == m.card.filter && msg.from == len(m.card.items):
		m.card.items = append(m.card.items, msg.page.Items...)
		m.card.total, m.top = msg.page.TotalCount, msg.from
	}

	return m
}

// entryChoices are the menu of the entries card, as far as the card can take
// each change now.
func (m screen) entryChoices() []choice {
	l, card := m.s.lang, m.card

	choices := []choice{{"f", "filter", l.text("track.list.filter", "Filter the entries")}}

	if card.unassigned > 0 && card.filter.project != "none" {
		choices = append(choices, choice{"o", "unassigned", l.text("ts.unassigned", "{0} without a project", strconv.Itoa(card.unassigned))})
	}

	if card.filter != (entryFilter{}) {
		choices = append(choices, choice{"c", "unfilter", l.text("track.list.unfilter", "Show every entry again")})
	}

	if len(card.items) < card.total && len(card.items) < listMost {
		choices = append(choices, choice{"m", "more", l.text("ts.more", "Show more")})
	}

	return choices
}

// chooseOnCard does what the entries card's menu offers without a form: the
// entries still waiting for a project, every entry again, and the next page. ok
// is false for anything else, which asks first.
func (m screen) chooseOnCard(doing string) (tea.Model, tea.Cmd, bool) {
	switch doing {
	case "unassigned":
		// The project filter's own "No project", as the page's link sets it: the
		// days and the words stay as they were.
		filter := m.card.filter
		filter.project = "none"
		next, cmd := m.narrow(filter)

		return next, cmd, true
	case "unfilter":
		next, cmd := m.narrow(entryFilter{})

		return next, cmd, true
	case "more":
		m.busy = true

		return m, m.more(), true
	}

	return m, nil, false
}

// narrow puts a filter on the entries card and reads it again, from its first
// entry.
func (m screen) narrow(filter entryFilter) (tea.Model, tea.Cmd) {
	m.card.filter, m.card.items, m.top = filter, nil, 0

	return m.ask(m.now)
}

// page moves the entries card a window's worth of entries down or up, no further
// than its ends.
func (m screen) page(down bool) screen {
	if m.currentTab().view != viewTimesheets {
		return m
	}

	rows := m.listRoom()
	last := max(len(m.card.items)-rows, 0)

	if down {
		m.top = min(m.top+rows, last)
	} else {
		m.top = max(min(m.top, last)-rows, 0)
	}

	return m
}

// subName is what the card whose menu is open is called.
func (m screen) subName() string {
	return m.s.lang.text("ts.entries", "Entries")
}

// filterAnswers are the filter form's answers before anybody has answered: the
// filter as it stands.
func (m screen) filterAnswers() *answers {
	f := m.card.filter

	return &answers{from: f.from, to: f.to, search: f.search, scope: f.project}
}

// filterSteps asks what the entries are narrowed to, a question at a time: from
// which day, up to which, which words, and which project.
func (m screen) filterSteps(a *answers) []*huh.Group {
	l := m.s.lang

	day := func(key, question string, into *string) *huh.Group {
		return huh.NewGroup(huh.NewInput().Key(key).Title(question).
			Placeholder(l.text("track.form.dayOrNone", "YYYY-MM-DD, or nothing")).Validate(l.readableDayOrNone).Value(into))
	}

	options := []huh.Option[string]{huh.NewOption(l.text("filter.allProjects", "All projects"), "")}
	for _, p := range m.shown.projects {
		options = append(options, huh.NewOption(p.Name, strconv.FormatUint(uint64(p.ID), 10)))
	}

	options = append(options, huh.NewOption(l.text("report.noProject", "No project"), "none"))

	scope := huh.NewSelect[string]().Key("scope").Title(l.text("track.ask.scope", "Which project's entries?")).
		Options(options...).Value(&a.scope)

	if len(options) > projectRows {
		scope = scope.Height(projectRows + 1)
	}

	return []*huh.Group{
		day("from", l.text("track.ask.from", "From which day on?"), &a.from),
		day("to", l.text("track.ask.to", "Up to which day?"), &a.to),
		huh.NewGroup(huh.NewInput().Key("search").Title(l.text("track.ask.search", "Which words do the descriptions hold?")).
			Placeholder(l.text("filter.searchPlaceholder", "a word, a ticket number, #meeting")).
			CharLimit(model.MaxDescriptionLength).Value(&a.search)),
		huh.NewGroup(scope),
	}
}

// filtered is the filter the form was answered with.
func (a answers) filtered() entryFilter {
	return entryFilter{
		from: strings.TrimSpace(a.from), to: strings.TrimSpace(a.to),
		search: strings.TrimSpace(a.search), project: a.scope,
	}
}

// entryCardView draws the entries card as the page draws its list: its title
// and what it is narrowed to, the entries under the page's columns, and how many
// of how many it shows where there are more. Where the window holds fewer than
// were read, rows of them are shown from the one at the top, and the card says
// which.
func (m screen) entryCardView(look palette, width, rows int) string {
	l, card := m.s.lang, m.card

	lines := []string{spread(width, look.title.Render(l.text("ts.entries", "Entries")), look.muted.Render(m.filterSummary()))}

	if len(card.items) == 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("ts.empty", "No entries for this filter."))), "\n")
	}

	columns := m.entryColumns(width)
	lines = append(lines, look.muted.Render(columns.row(l.text("field.date", "Date"), l.text("field.project", "Project"),
		l.text("field.hours", "Hours"), l.text("field.description", "Description"))))

	rows = max(min(rows, len(card.items)), min(listFewest, len(card.items)))
	top := min(m.top, len(card.items)-rows)

	for _, e := range card.items[top : top+rows] {
		lines = append(lines, m.entryRow(look, columns, e))
	}

	var tally, place string

	if len(card.items) < card.total {
		tally = l.text("ts.showing", "Showing {0} of {1} entries", strconv.Itoa(len(card.items)), strconv.Itoa(card.total))
	}

	if rows < len(card.items) {
		place = l.text("track.list.rows", "{0}–{1} of {2} · pgup/pgdn", strconv.Itoa(top+1), strconv.Itoa(top+rows),
			strconv.Itoa(len(card.items)))
	}

	if tally != "" || place != "" {
		lines = append(lines, look.muted.Render(spread(width, tally, place)))
	}

	return strings.Join(lines, "\n")
}

// filterSummary says what the entries card is narrowed to, since the screen has
// no fields to show it in: the project, the days and the words.
func (m screen) filterSummary() string {
	l, f := m.s.lang, m.card.filter

	scope := l.text("filter.allProjects", "All projects")

	switch f.project {
	case "":
	case "none":
		scope = l.text("report.noProject", "No project")
	default:
		id, _ := strconv.ParseUint(f.project, 10, 64)
		picked := uint(id)
		scope = l.nameOf(m.shown.projects, &picked)
	}

	said := []string{scope}

	switch {
	case f.from != "" && f.to != "":
		said = append(said, l.date(f.from)+" – "+l.date(f.to))
	case f.from != "":
		said = append(said, l.text("track.list.from", "from {0}", l.date(f.from)))
	case f.to != "":
		said = append(said, l.text("track.list.until", "until {0}", l.date(f.to)))
	}

	if f.search != "" {
		said = append(said, "“"+f.search+"”")
	}

	return strings.Join(said, " · ")
}

// entryColumns is how wide the card's columns are drawn: a date as the reader
// writes one, the project up to a third of the line, the hours as wide as the
// widest, and the description in the rest.
type entryColumns struct {
	day, project, hours int
}

// entryColumns measures the columns for the entries read.
func (m screen) entryColumns(width int) entryColumns {
	l := m.s.lang
	c := entryColumns{
		day:     lipgloss.Width(l.text("field.date", "Date")),
		project: lipgloss.Width(l.text("field.project", "Project")),
		hours:   lipgloss.Width(l.text("field.hours", "Hours")),
	}

	for _, e := range m.card.items {
		c.day = max(c.day, lipgloss.Width(l.date(e.Date)))
		c.project = max(c.project, lipgloss.Width(m.projectOf(e)))
		c.hours = max(c.hours, lipgloss.Width(l.number(e.DurationHours)))
	}

	c.project = min(c.project, width/3)

	return c
}

// row sets the four cells of a line in their columns, the hours to the right as
// the page sets figures.
func (c entryColumns) row(day, project, hours, description string) string {
	pad := func(text string, width int) string {
		text = ansi.Truncate(text, width, "…")

		return text + strings.Repeat(" ", max(width-lipgloss.Width(text), 0))
	}

	return pad(day, c.day) + "  " + pad(project, c.project) + "  " +
		strings.Repeat(" ", max(c.hours-lipgloss.Width(hours), 0)) + hours + "  " + description
}

// entryRow is one entry as a line of the card: no project and no description in
// the quiet colour, as the page shows what is missing, and who booked or changed
// it where that was somebody else.
func (m screen) entryRow(look palette, c entryColumns, e entry) string {
	l := m.s.lang
	width := m.inner()

	project := m.projectOf(e)
	if e.ProjectID == nil {
		project = look.muted.Render(ansi.Truncate(project, c.project, "…"))
	}

	description := look.muted.Render("–")
	if e.Description != nil && *e.Description != "" {
		description = *e.Description
	}

	if e.RecordedBy != nil {
		description += look.muted.Render(" · " + l.text("team.recordedBy", "booked by {0}", e.RecordedBy.Name))
	}

	if e.UpdatedBy != nil {
		description += look.muted.Render(" · " + l.text("team.updatedBy", "changed by {0}", e.UpdatedBy.Name))
	}

	return ansi.Truncate(c.row(l.date(e.Date), project, l.number(e.DurationHours), description), width, "…")
}

// projectOf is the name of an entry's project, as the page's list names it.
func (m screen) projectOf(e entry) string {
	if e.ProjectID == nil {
		return m.s.lang.text("ts.noProject", "No project")
	}

	return m.s.lang.nameOf(m.shown.projects, e.ProjectID)
}
