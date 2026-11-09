package cli

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// The templates card, as the page keeps it under its booking form: bookings
// somebody makes often, kept to fill the booking in and never to book by
// themselves.

// template is a kept booking as /me/templates answers with one. Any of the three
// may be missing, and none has a day, which is always the booking's.
type template struct {
	ID            uint     `json:"id"`
	ProjectID     *uint    `json:"projectId"`
	DurationHours *float64 `json:"durationHours"`
	Description   *string  `json:"description"`
}

// templates reads the account's templates.
func (s *session) templates(ctx context.Context) ([]template, error) {
	var listed struct {
		Items []template `json:"items"`
	}

	err := s.api.call(ctx, http.MethodGet, "/me/templates", nil, &listed)

	return listed.Items, err
}

// templateLabel is what a template fills in, in one line, as the page writes it:
// its project, its hours and its description.
func (m screen) templateLabel(t template) string {
	l := m.s.lang

	said := []string{l.text("ts.noProject", "No project")}
	if t.ProjectID != nil {
		said[0] = l.nameOf(m.shown.projects, t.ProjectID)
	}

	if t.DurationHours != nil && *t.DurationHours > 0 {
		said = append(said, l.hours(*t.DurationHours))
	}

	if t.Description != nil && *t.Description != "" {
		said = append(said, *t.Description)
	}

	return strings.Join(said, " · ")
}

// usable reports whether a template can still fill the form in: the project it
// names, where it names one, still takes bookings. One kept for a project
// finished since would fill in a project the form does not offer.
func (m screen) usable(t template) bool {
	return t.ProjectID == nil || slices.ContainsFunc(m.shown.projects, func(p project) bool {
		return p.ID == *t.ProjectID && p.Status == "active"
	})
}

// templateChoices are the templates card's menu.
func (m screen) templateChoices() []choice {
	l := m.s.lang
	choices := []choice{{"n", "tplNew", l.text("track.tpl.new", "Save a new template")}}

	if len(m.templates) > 0 {
		choices = append(choices, choice{"d", "tplDelete", l.text("track.tpl.delete", "Delete a template")})
	}

	return choices
}

// templateStep offers the templates that can fill the booking in, as the page's
// picker heads its form, and booking without one first.
func (m screen) templateStep(a *answers) *huh.Group {
	l := m.s.lang
	options := []huh.Option[uint]{huh.NewOption(l.text("track.tpl.none", "Book without a template"), uint(0))}

	for _, t := range m.templates {
		if m.usable(t) {
			options = append(options, huh.NewOption(m.templateLabel(t), t.ID))
		}
	}

	field := huh.NewSelect[uint]().Key("template").Title(l.text("tpl.fill", "Fill in from a template")).
		Options(options...).Value(&a.template)

	if len(options) > projectRows {
		field = field.Height(projectRows + 1)
	}

	return huh.NewGroup(field)
}

// fromTemplate is the booking form's answers filled from the template chosen,
// with the day the booking is for; none chosen fills in nothing.
func (m screen) fromTemplate(id uint, day string) *answers {
	a := &answers{day: day, stop: true}

	at := slices.IndexFunc(m.templates, func(t template) bool { return t.ID == id })
	if at < 0 {
		return a
	}

	t := m.templates[at]

	if t.ProjectID != nil {
		a.project = *t.ProjectID
	}

	if t.DurationHours != nil {
		a.hours = m.s.lang.number(*t.DurationHours)
	}

	if t.Description != nil {
		a.description = *t.Description
	}

	return a
}

// newTemplateSteps ask what a template keeps: the hours, which may be left out,
// the project and the description - all the booking form asks but the day.
func (m screen) newTemplateSteps(a *answers) []*huh.Group {
	l := m.s.lang

	return []*huh.Group{
		huh.NewGroup(huh.NewInput().Key("hours").Title(l.text("track.ask.hours", "How many hours?")).
			Placeholder(l.text("track.form.optional", "optional")).Validate(l.readableHoursOrNone).Value(&a.hours)),
		huh.NewGroup(m.projectField(&a.project)),
		huh.NewGroup(m.describe(&a.description)),
	}
}

// templateDeleteStep asks before the template picked is deleted, naming it as
// the page names it.
func (m screen) templateDeleteStep(a *answers) *huh.Group {
	l := m.s.lang
	named := l.text("tpl.one", "Template") + ` "` + m.templateLabel(m.chosen) + `"`

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("confirm.deleteTitle", "Delete for good?")).
		Description(named + " " + l.text("confirm.deleteText", "will be deleted. This cannot be undone.")).
		Affirmative(l.text("track.form.deleteYes", "Delete")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// saveTemplate keeps what the form was answered with. What was left out is
// sent as nothing, as the page sends it.
func (m screen) saveTemplate() tea.Cmd {
	ctx, s, a := m.ctx, m.s, *m.asked
	body := map[string]any{"projectId": nil, "durationHours": nil, "description": nil}

	if a.project != 0 {
		body["projectId"] = a.project
	}

	if raw := strings.TrimSpace(a.hours); raw != "" {
		worked, err := s.lang.parseHours(raw)
		if err != nil {
			return func() tea.Msg { return done{err: err} }
		}

		body["durationHours"] = worked
	}

	if text := strings.TrimSpace(a.description); text != "" {
		body["description"] = text
	}

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodPost, "/me/templates", body, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("tpl.saved", "Saved as a template")}
	}
}

// deleteTemplate deletes the template picked, once the answer was yes.
func (m screen) deleteTemplate() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, id := m.ctx, m.s, m.chosen.ID

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodDelete, fmt.Sprintf("/me/templates/%d", id), nil, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("tpl.deleted", "Template deleted")}
	}
}

// templateCardView draws the templates card as the page draws it: what templates
// are for, and each with its project, hours and description, the cursor before
// one while one is being picked.
func (m screen) templateCardView(look palette, width int) string {
	l := m.s.lang

	hint := l.text("track.tpl.hint", "Bookings you make often. A template only fills in a booking by hand: nothing is booked until its form is answered.")
	lines := []string{look.title.Render(l.text("tpl.title", "Templates")),
		lipgloss.NewStyle().Width(width).Render(look.muted.Render(hint))}

	if len(m.templates) == 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("tpl.empty", "No templates yet."))), "\n")
	}

	table := grid{
		headings: []string{l.text("field.project", "Project"), l.text("field.hours", "Hours"),
			l.text("field.description", "Description")},
		right: []bool{false, true},
		most:  []int{3},
	}

	for _, t := range m.templates {
		project := look.muted.Render(l.text("ts.noProject", "No project"))
		if t.ProjectID != nil {
			project = l.nameOf(m.shown.projects, t.ProjectID)
		}

		hours := "–"
		if t.DurationHours != nil && *t.DurationHours > 0 {
			hours = l.number(*t.DurationHours)
		}

		description := ""
		if t.Description != nil {
			description = *t.Description
		}

		table.rows = append(table.rows, []string{project, hours, description})
	}

	inner := width - 2
	lines = append(lines, "  "+table.heading(look, inner))

	for i, line := range table.lines(inner) {
		cursor := "  "
		if m.picking == "tplDelete" && i == m.row {
			cursor = look.title.Render("> ")
		}

		lines = append(lines, cursor+line)
	}

	return strings.Join(lines, "\n")
}

// readableHoursOrNone refuses hours written in no way parseHours reads; empty is
// none, which a template may keep.
func (l language) readableHoursOrNone(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	return l.readableHours(raw)
}
