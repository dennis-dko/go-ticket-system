package cli

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// addedRow is a row added to a week's grid by hand, before anything is booked on
// it: the week, by its Monday, and the row's key. Kept on the screen alone, as
// the page keeps its weekExtraRows: a row with nothing on it is not a fact about
// anything, and it goes with the screen.
type addedRow struct{ monday, key string }

// addable are the rows that can still be added to the grid, as the page's picker
// offers them: the active projects with no row, in the order they are listed,
// and the hours without a project where they have none.
func (m screen) addable(rows []weekRow) []weekRow {
	shown := func(key string) bool {
		return slices.ContainsFunc(rows, func(r weekRow) bool { return r.key == key })
	}

	var addable []weekRow

	for _, p := range m.shown.projects {
		if p.Status == "active" && !shown(rowKey(&p.ID)) {
			addable = append(addable, *m.newRow(rowKey(&p.ID), &p.ID))
		}
	}

	if !shown("none") {
		addable = append(addable, *m.newRow("none", nil))
	}

	return addable
}

// addRowStep asks which project gets a row, in the page's words for its picker.
func (m screen) addRowStep(a *answers) *huh.Group {
	addable := m.addable(m.weekRows())

	options := make([]huh.Option[string], len(addable))
	for i, r := range addable {
		options[i] = huh.NewOption(r.name, r.key)
	}

	field := huh.NewSelect[string]().Key("row").Title(m.s.lang.text("week.addRow", "Add a row for")).
		Options(options...).Value(&a.row)

	// The height counts the title's line.
	if len(options) > projectRows {
		field = field.Height(projectRows + 1)
	}

	return huh.NewGroup(field)
}

// addRow adds the row chosen to the week shown, and puts the cursor on its
// Monday - where somebody adding a row starts typing, as the page has it.
func (m screen) addRow(key string) (tea.Model, tea.Cmd) {
	m.added = append(slices.Clip(m.added), addedRow{monday: m.week.monday, key: key})

	for at, r := range m.weekRows() {
		if r.key == key {
			m.picking, m.cell = "cell", weekCell{row: key, at: at, day: 0}
		}
	}

	return m, nil
}

// addedRows are the rows added by hand to the week shown that nothing is booked
// on yet: a project's while it is still listed, and the hours without one.
func (m screen) addedRows(booked map[string]*weekRow) []*weekRow {
	var rows []*weekRow

	for _, a := range m.added {
		if a.monday != m.week.monday || booked[a.key] != nil {
			continue
		}

		if a.key == "none" {
			rows = append(rows, m.newRow(a.key, nil))

			continue
		}

		for _, p := range m.shown.projects {
			if rowKey(&p.ID) == a.key {
				rows = append(rows, m.newRow(a.key, &p.ID))
			}
		}
	}

	return rows
}

// copyStep asks before last week is copied into the one shown, in the page's
// words: it books entries rather than filling a form in.
func (m screen) copyStep(a *answers) *huh.Group {
	l := m.s.lang

	return huh.NewGroup(huh.NewConfirm().Key("copy").Title(l.text("week.copyTitle", "Copy last week?")).
		Description(l.text("week.copyText",
			"The entries of the week from {0} are booked again on the same weekdays of this one - only on days with nothing booked yet.",
			l.date(addDays(m.week.monday, -7)))).
		Affirmative(l.text("week.copyConfirm", "Copy")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// weekCopy is what a copy of last week did, as /me/weeks/copy answers.
type weekCopy struct {
	Copied int     `json:"copied"`
	Hours  float64 `json:"hours"`
	Kept   int     `json:"kept"`
	Closed int     `json:"closed"`
}

// copyWeek books last week's entries into the days of the week shown that have
// nothing on them, once the answer was yes.
func (m screen) copyWeek() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, into := m.ctx, m.s, m.week.monday
	body := map[string]any{"from": addDays(into, -7), "to": into}

	return func() tea.Msg {
		var copied weekCopy
		if err := s.api.call(ctx, http.MethodPost, "/me/weeks/copy", body, &copied); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.weekCopied(copied)}
	}
}

// weekCopied says what a copy did as the page's weekCopyNews says it: a sentence
// for each part of it that happened, the days it left alone and the entries it
// left out included, since either would otherwise look like something the copy
// had missed.
func (l language) weekCopied(copied weekCopy) string {
	news := []string{l.text("week.copied", "Entries copied: {0} ({1}).", strconv.Itoa(copied.Copied), l.hours(copied.Hours))}

	if copied.Kept > 0 {
		news = append(news, l.text("week.copyKept", "Days already booked, left as they were: {0}.", strconv.Itoa(copied.Kept)))
	}

	if copied.Closed > 0 {
		news = append(news, l.text("week.copyClosed", "Left out because their project no longer takes bookings: {0}.",
			strconv.Itoa(copied.Closed)))
	}

	return strings.Join(news, " ")
}
