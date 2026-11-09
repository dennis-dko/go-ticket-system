package cli

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
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

	choices := append(m.rowChoices(), choice{"f", "filter", l.text("track.list.filter", "Filter the entries")})

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
	case "edit", "delete", "history", "assign", "deleteSome", "tplDelete", "workEdit", "workDelete",
		"leaveDelete", "awayDelete":
		return m.startPicking(doing), nil, true
	case "write":
		m.busy = true

		return m, m.writeFile(), true
	case "drop":
		m.plan = nil

		return m, nil, true
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
	if m.currentTab().view == viewOvertime {
		return m.pageDays(down)
	}

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
	switch m.sub {
	case subClock:
		return m.s.lang.text("timer.title", "Stopwatch")
	case subTemplates:
		return m.s.lang.text("tpl.title", "Templates")
	case subWorkbook:
		return m.s.lang.text("wb.title", "Spreadsheet")
	case subWork:
		return m.s.lang.text("work.title", "Working time")
	case subLeave:
		return m.s.lang.text("track.leave.title", "Leave")
	case subAway:
		return m.s.lang.text("absence.title", "Days away")
	}

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
// which. While an entry is being picked, the cursor stands before it, and the
// entries that cannot be picked are drawn quietly.
func (m screen) entryCardView(look palette, width, rows int) string {
	l, card := m.s.lang, m.card

	lines := []string{spread(width, look.title.Render(l.text("ts.entries", "Entries")), look.muted.Render(m.filterSummary()))}

	if len(card.items) == 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("ts.empty", "No entries for this filter."))), "\n")
	}

	// Two columns are kept for the cursor, so the entries do not move when one
	// is being picked.
	inner := width - 2
	table := m.entryGrid(look)
	lines = append(lines, "  "+table.heading(look, inner))

	rows = min(rows, len(card.items))
	top := max(min(m.top, len(card.items)-rows), 0)

	for i, line := range table.lines(inner)[top : top+rows] {
		at := top + i

		// The cursor's column, then a tick where several are being picked.
		cursor, tick := " ", " "

		if m.picking != "" && at == m.row {
			cursor = look.title.Render(">")
		}

		if m.several() && slices.Contains(m.ticked, card.items[at].ID) {
			tick = look.ok.Render("✓")
		}

		if m.picking != "" && !m.pickable(card.items[at]) {
			line = look.muted.Render(ansi.Strip(line))
		}

		lines = append(lines, cursor+tick+line)
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
		// Found by the id as the filter writes it rather than turned back into a
		// number, which on a 32-bit build cut a large id short into another's.
		scope = l.text("track.projectNumber", "project {0}", f.project)

		for _, p := range m.shown.projects {
			if strconv.FormatUint(uint64(p.ID), 10) == f.project {
				scope = p.Name
			}
		}
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

// entryGrid is the entries read as the page's table draws them: the day, the
// project up to a third of the line, the hours as a figure, and the description
// in what is left.
func (m screen) entryGrid(look palette) grid {
	l := m.s.lang
	table := grid{
		headings: []string{l.text("field.date", "Date"), l.text("field.project", "Project"),
			l.text("field.hours", "Hours"), l.text("field.description", "Description")},
		right: []bool{false, false, true},
		most:  []int{0, 3},
	}

	for _, e := range m.card.items {
		table.rows = append(table.rows, m.entryCells(look, e))
	}

	return table
}

// entryCells are one entry's cells: no project and no description in the quiet
// colour, as the page shows what is missing, and who booked or changed it where
// that was somebody else.
func (m screen) entryCells(look palette, e entry) []string {
	l := m.s.lang

	project := m.projectOf(e)
	if e.ProjectID == nil {
		project = look.muted.Render(project)
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

	return []string{l.date(e.Date), project, l.number(e.DurationHours), description}
}

// projectOf is the name of an entry's project, as the page's list names it.
func (m screen) projectOf(e entry) string {
	if e.ProjectID == nil {
		return m.s.lang.text("ts.noProject", "No project")
	}

	return m.s.lang.nameOf(m.shown.projects, e.ProjectID)
}
