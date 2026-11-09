package cli

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// The changes to one entry - correcting it, deleting it, and showing what it said
// before - pick it in the entries card: the change is chosen from the menu first
// and the entry after it, which is how the maintainer chose the screen to work.

// revision is what an entry said before one correction, as the installation
// keeps it: when it was replaced and by whom where that was somebody else.
type revision struct {
	Date          string    `json:"date"`
	DurationHours float64   `json:"durationHours"`
	ProjectID     *uint     `json:"projectId"`
	Description   *string   `json:"description"`
	ReplacedAt    time.Time `json:"replacedAt"`
	ReplacedBy    *actedBy  `json:"replacedBy"`
}

// history is an entry's earlier versions, the most recently replaced first, or
// why they could not be read.
type history struct {
	of       entry
	versions []revision
	err      error
}

// mayWrite reports whether the account may change its own time, which every
// change to an entry needs, as the page asks it.
func (m screen) mayWrite() bool {
	return slices.Contains(m.shown.permissions, model.PermTimesheetWriteOwn)
}

// revised reports whether an entry was saved again since it was booked: the only
// entries with a history, and the only ones the page offers one for.
func (e entry) revised() bool {
	return e.CreatedAt != nil && e.UpdatedAt != nil && !e.UpdatedAt.Equal(*e.CreatedAt)
}

// rowChoices are the entries card's changes to one entry, as far as there is an
// entry each can be made to.
func (m screen) rowChoices() []choice {
	l, items := m.s.lang, m.card.items

	var choices []choice

	if m.mayWrite() && len(items) > 0 {
		choices = append(choices,
			choice{"e", "edit", l.text("track.list.edit", "Correct an entry")},
			choice{"d", "delete", l.text("track.list.delete", "Delete an entry")})
	}

	if slices.ContainsFunc(items, entry.revised) {
		choices = append(choices, choice{"h", "history", l.text("track.list.history", "Earlier versions of an entry")})
	}

	return append(choices, m.severalChoices()...)
}

// entryLabel names an entry by its day and hours, as the page names a row it
// asks about.
func (l language) entryLabel(e entry) string {
	return l.date(e.Date) + ", " + l.hours(e.DurationHours)
}

// pickPrompt asks which entry the change being made is for.
func (m screen) pickPrompt() string {
	l := m.s.lang

	switch m.picking {
	case "delete":
		return l.text("track.pick.delete", "Choose the entry to delete.")
	case "history":
		return l.text("track.pick.history", "Choose the entry whose earlier versions to show.")
	case "assign":
		return l.text("track.pick.assign", "Tick the entries to give a project with space, then press enter.") +
			"  " + m.tickedCount()
	case "deleteSome":
		return l.text("track.pick.deleteSome", "Tick the entries to delete with space, then press enter.") +
			"  " + m.tickedCount()
	case "tplDelete":
		return l.text("track.pick.template", "Choose the template to delete.")
	case "cell":
		return l.text("track.pick.cell", "Choose the cell with the arrows, then press enter.")
	}

	return l.text("track.pick.edit", "Choose the entry to correct.")
}

// pickable reports whether an entry can be picked for the change one is being
// picked for.
func (m screen) pickable(e entry) bool {
	return m.picking != "history" || e.revised()
}

// startPicking puts the cursor on the first entry on screen that can be picked
// for doing, or the first in the card where none on screen can.
func (m screen) startPicking(doing string) screen {
	m.picking, m.news, m.failure, m.ticked = doing, "", "", nil

	// The templates card shows every template, so its picking starts at the
	// first; the entries card starts at the first entry on screen.
	first := 0
	if !m.pickingTemplate() {
		first = m.top
	}

	m.row = m.stepPick(first-1, 1)
	if m.row == first-1 {
		m.row = m.stepPick(-1, 1)
	}

	if m.row < 0 {
		m.picking = ""

		return m
	}

	if !m.pickingTemplate() {
		m.top = m.inView(m.row)
	}

	return m
}

// pickingTemplate reports whether what is being picked is a template rather
// than an entry.
func (m screen) pickingTemplate() bool {
	return m.picking == "tplDelete"
}

// pickCount is how many rows there are to pick from: the templates or the
// entries read.
func (m screen) pickCount() int {
	if m.pickingTemplate() {
		return len(m.templates)
	}

	return len(m.card.items)
}

// pickableAt reports whether the row at can be picked; every template can.
func (m screen) pickableAt(at int) bool {
	return m.pickingTemplate() || m.pickable(m.card.items[at])
}

// stepPick is the next row that can be picked from the one at from, in the
// direction by, or from itself where there is none.
func (m screen) stepPick(from, by int) int {
	for at := from + by; at >= 0 && at < m.pickCount(); at += by {
		if m.pickableAt(at) {
			return at
		}
	}

	return from
}

// inView is the first entry on screen once the one at row is in view.
func (m screen) inView(row int) int {
	rows := m.listRoom()

	switch {
	case row < m.top:
		return row
	case row >= m.top+rows:
		return row - rows + 1
	}

	return m.top
}

// pickKey answers a key while an entry is being picked: a move to the next or the
// previous entry that can be picked, or a window's worth of them, space to tick
// one where several are picked, enter on the one the cursor is on, and escape to
// put the picking away.
func (m screen) pickKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "space":
		if m.several() {
			m = m.tick()
		}
	case "up", "k":
		m.row = m.stepPick(m.row, -1)
	case "down", "j":
		m.row = m.stepPick(m.row, 1)
	case "pgup", "pgdown":
		by := 1
		if k.String() == "pgup" {
			by = -1
		}

		target := min(max(m.row+by*m.listRoom(), 0), m.pickCount()-1)
		if next := m.stepPick(target-by, by); m.pickableAt(next) {
			m.row = next
		}
	case "esc":
		m.picking = ""

		return m, nil
	case "enter":
		return m.picked()
	}

	if !m.pickingTemplate() {
		m.top = m.inView(m.row)
	}

	return m, nil
}

// picked makes the change the entry the cursor is on was picked for: its
// history is read, and a correction or a deletion asks first. Where several are
// picked and none is ticked, the one the cursor is on is the one meant.
func (m screen) picked() (tea.Model, tea.Cmd) {
	if m.pickingTemplate() {
		m.picking, m.chosen = "", m.templates[m.row]

		return m.open("tplDelete")
	}

	doing, e := m.picking, m.card.items[m.row]

	if m.several() && len(m.ticked) == 0 {
		m.ticked = []uint{e.ID}
	}

	m.picking, m.editing = "", e

	if doing == "history" {
		m.busy = true

		return m, m.readHistory(e)
	}

	return m.open(doing)
}

// correctionAnswers are the booking form's answers for the entry being
// corrected: what it says now, so that what is not changed is sent as it was.
func (m screen) correctionAnswers() *answers {
	e := m.editing
	a := &answers{hours: m.s.lang.number(e.DurationHours), day: e.Date}

	if e.ProjectID != nil {
		a.project = *e.ProjectID
	}

	if e.Description != nil {
		a.description = *e.Description
	}

	return a
}

// deleteStep asks before the entry picked is deleted, in the page's words.
func (m screen) deleteStep(a *answers) *huh.Group {
	l, e := m.s.lang, m.editing

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("confirm.deleteTitle", "Delete for good?")).
		Description(l.entryLabel(e) + " " + l.text("confirm.deleteText", "will be deleted. This cannot be undone.")).
		Affirmative(l.text("track.form.deleteYes", "Delete")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// saveCorrection sends the corrected entry with every field as the form was
// answered. A project taken off is sent as zero and an emptied description as
// nothing, as the page sends both, because a field left out of a correction is
// one left alone.
func (m screen) saveCorrection() tea.Cmd {
	ctx, s, a, id := m.ctx, m.s, *m.asked, m.editing.ID

	worked, err := s.lang.parseHours(strings.TrimSpace(a.hours))
	if err != nil {
		return func() tea.Msg { return done{err: err} }
	}

	body := map[string]any{
		"projectId": a.project, "date": strings.TrimSpace(a.day),
		"durationHours": worked, "description": strings.TrimSpace(a.description),
	}

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodPut, fmt.Sprintf("/timesheets/%d", id), body, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("msg.entrySaved", "Entry saved")}
	}
}

// deleteEntry deletes the entry picked, once the answer was yes.
func (m screen) deleteEntry() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, id := m.ctx, m.s, m.editing.ID

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodDelete, fmt.Sprintf("/timesheets/%d", id), nil, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("msg.entryDeleted", "Entry deleted")}
	}
}

// readHistory reads what an entry said before each correction.
func (m screen) readHistory(e entry) tea.Cmd {
	ctx, s := m.ctx, m.s

	return func() tea.Msg {
		var listed struct {
			Items []revision `json:"items"`
		}

		err := s.api.call(ctx, http.MethodGet, fmt.Sprintf("/timesheets/%d/revisions", e.ID), nil, &listed)

		return history{of: e, versions: listed.Items, err: err}
	}
}

// showHistory puts an entry's history on the screen until it is closed, or says
// why it could not be read.
func (m screen) showHistory(h history) screen {
	m.busy = false

	if h.err != nil {
		m.news, m.failure = "", h.err.Error()

		return m
	}

	m.past = &h

	return m
}

// historyView draws an entry's earlier versions as the page's dialog shows them:
// when each was replaced and by whom, and its day, project, hours and
// description then. Read only: nothing in a history is put back from here.
func (m screen) historyView(look palette, width int) string {
	l, h := m.s.lang, m.past

	lines := []string{look.title.Render(l.text("history.title", "Earlier versions of the entry from {0}", l.date(h.of.Date)))}

	if len(h.versions) == 0 {
		said := l.text("history.none", "Saved again since it was booked, without anything in it changing.")

		return strings.Join(append(lines, look.muted.Render(said)), "\n")
	}

	table := grid{
		headings: []string{l.text("history.replaced", "Replaced"), l.text("field.date", "Date"),
			l.text("field.project", "Project"), l.text("field.hours", "Hours"), l.text("field.description", "Description")},
		right: []bool{false, false, false, true},
		most:  []int{0, 0, 4},
	}

	for _, v := range h.versions {
		at := v.ReplacedAt.In(m.shown.zone)
		replaced := l.date(at.Format(time.DateOnly)) + " " + at.Format("15:04")

		if v.ReplacedBy != nil {
			replaced += look.muted.Render(" " + l.text("team.replacedBy", "by {0}", v.ReplacedBy.Name))
		}

		on := l.text("ts.noProject", "No project")
		if v.ProjectID != nil {
			on = l.text("history.unlistedProject", "a project no longer listed")

			if at := slices.IndexFunc(m.shown.projects, func(p project) bool { return p.ID == *v.ProjectID }); at >= 0 {
				on = m.shown.projects[at].Name
			}
		}

		description := ""
		if v.Description != nil {
			description = *v.Description
		}

		table.rows = append(table.rows, []string{replaced, l.date(v.Date), on, l.number(v.DurationHours), description})
	}

	return strings.Join(append(append(lines, table.heading(look, width)), table.lines(width)...), "\n")
}
