package cli

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// The page's bar for several rows - give them a project, delete them - picks
// its entries in the card as well: the change is chosen first, the entries are
// ticked with space, and enter goes on with them. Each entry goes through its
// own route, one after another, as the page sends them, so that several can
// do nothing a change to one could not.

// mostNamed is how many of the entries ticked for deletion the question names,
// as the page's names the first five.
const mostNamed = 5

// severalChoices are the entries card's changes to several entries at once:
// giving them a project, for an account with the right to and where there is a
// project to give, and deleting them.
func (m screen) severalChoices() []choice {
	l := m.s.lang

	if !m.mayWrite() || len(m.card.items) == 0 {
		return nil
	}

	var choices []choice

	if slices.Contains(m.shown.permissions, model.PermTimesheetTransfer) && len(m.activeProjects()) > 0 {
		choices = append(choices, choice{"a", "assign", l.text("track.list.assign", "Assign entries to a project")})
	}

	return append(choices, choice{"x", "deleteSome", l.text("track.list.deleteSome", "Delete several entries")})
}

// several reports whether the entries being picked are for a change to several.
func (m screen) several() bool {
	return m.picking == "assign" || m.picking == "deleteSome"
}

// tick ticks the entry the cursor is on, or takes its tick off again.
func (m screen) tick() screen {
	id := m.card.items[m.row].ID

	if at := slices.Index(m.ticked, id); at >= 0 {
		m.ticked = slices.Delete(slices.Clone(m.ticked), at, at+1)
	} else {
		m.ticked = append(slices.Clone(m.ticked), id)
	}

	return m
}

// tickedEntries are the entries ticked, in the order the card lists them.
func (m screen) tickedEntries() []entry {
	var ticked []entry

	for _, e := range m.card.items {
		if slices.Contains(m.ticked, e.ID) {
			ticked = append(ticked, e)
		}
	}

	return ticked
}

// activeProjects are the account's projects that accept hours, the only ones
// an entry can be given.
func (m screen) activeProjects() []project {
	var active []project

	for _, p := range m.shown.projects {
		if p.Status == "active" {
			active = append(active, p)
		}
	}

	return active
}

// assignStep asks which of the active projects the ticked entries are given.
func (m screen) assignStep(a *answers) *huh.Group {
	l := m.s.lang
	active := m.activeProjects()
	options := make([]huh.Option[uint], len(active))

	for i, p := range active {
		options[i] = huh.NewOption(p.Name, p.ID)
	}

	field := huh.NewSelect[uint]().Key("project").Title(l.text("bulk.assignTo", "Assign to a project …")).
		Options(options...).Value(&a.project)

	if len(options) > projectRows {
		field = field.Height(projectRows + 1)
	}

	return huh.NewGroup(field)
}

// deleteSomeStep asks before the ticked entries are deleted, in the page's
// words, and names the first few of them.
func (m screen) deleteSomeStep(a *answers) *huh.Group {
	l := m.s.lang
	ticked := m.tickedEntries()

	named := make([]string, 0, mostNamed+1)
	for _, e := range ticked[:min(len(ticked), mostNamed)] {
		named = append(named, "· "+l.entryLabel(e))
	}

	if len(ticked) > mostNamed {
		named = append(named, "· …")
	}

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("bulk.title", "Delete the selected rows?")).
		Description(l.text("bulk.text", "This cannot be undone.") + "\n" + strings.Join(named, "\n")).
		Affirmative(l.text("bulk.delete", "Delete selected")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// transferSome gives each ticked entry the project chosen. A refusal partway
// does not undo the rest; it is said with the first of them by name, as the
// page says it.
func (m screen) transferSome() tea.Cmd {
	ctx, s, project, ticked := m.ctx, m.s, m.asked.project, m.tickedEntries()
	l, name := s.lang, s.lang.nameOf(m.shown.projects, &project)

	return func() tea.Msg {
		var refused []string

		for _, e := range ticked {
			body := map[string]uint{"projectId": project}
			if err := s.api.call(ctx, http.MethodPost, fmt.Sprintf("/timesheets/%d/transfer", e.ID), body, nil); err != nil {
				refused = append(refused, l.entryLabel(e)+": "+err.Error())
			}
		}

		if len(refused) > 0 {
			return done{err: errors.New(l.text("bulk.assignRefused", "{0} could not be assigned",
				strconv.Itoa(len(refused))) + " – " + refused[0])}
		}

		return done{said: l.text("bulk.assigned", "{0} assigned to {1}", strconv.Itoa(len(ticked)), name)}
	}
}

// deleteSome deletes each ticked entry, once the answer was yes, and says how
// many as the page says it.
func (m screen) deleteSome() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, ticked := m.ctx, m.s, m.tickedEntries()
	l := s.lang

	return func() tea.Msg {
		var refused []string

		for _, e := range ticked {
			if err := s.api.call(ctx, http.MethodDelete, fmt.Sprintf("/timesheets/%d", e.ID), nil, nil); err != nil {
				refused = append(refused, l.entryLabel(e)+": "+err.Error())
			}
		}

		// The page's two sentences count with {n} rather than {0}.
		if len(refused) > 0 {
			said := strings.Replace(l.text("bulk.refused", "{n} could not be deleted"), "{n}", strconv.Itoa(len(refused)), 1)

			return done{err: errors.New(said + " – " + refused[0])}
		}

		return done{said: strings.Replace(l.text("bulk.done", "{n} deleted"), "{n}", strconv.Itoa(len(ticked)), 1)}
	}
}

// tickedCount says how many entries are ticked, as the page's bar says it.
func (m screen) tickedCount() string {
	return strings.Replace(m.s.lang.text("bulk.count", "{n} selected"), "{n}", strconv.Itoa(len(m.ticked)), 1)
}
