package cli

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// projectRows is how many projects a form shows at once before the list
// scrolls, so that a long list still fits under the stopwatch.
const projectRows = 7

// answers are what a form has been told.
type answers struct {
	project     uint
	description string
	hours       string
	day         string
	stop        bool

	// confirmed is the answer to whether something is deleted or discarded, no
	// until it is given.
	confirmed bool

	// clockDay and clockTime are a moment on the owner's wall clock, when the
	// stopwatch's work started or ended.
	clockDay, clockTime string

	// template is the one chosen to fill a booking in, zero for none.
	template uint

	// file is where an export goes, or the file an import reads.
	file string

	// workDay is the day whose working time is recorded, and workStart, workEnd
	// and workBreaks what it is recorded as.
	workDay, workStart, workEnd, workBreaks string

	// cell is the week grid's cell the hours are entered into, and row the key of
	// the row added to it.
	cell cellPicked
	row  string

	// from, to, search and scope are what the entries are narrowed to; see
	// entryFilter.
	from, to, search, scope string

	// setting is which of the account's settings the settings form changes, and
	// the rest what it is changed to; see setting.go.
	setting                           string
	language, theme, zone             string
	country, region                   string
	unbookedDay                       bool
	alertHours, upper, lower          string
	targetHours, maxHours, countsFrom string
}

// formFor is the form for a change, writing into the screen's answers.
//
// One question at a time, each a step of its own that the breadcrumb names, and
// each asked in a sentence rather than by the name of its field: the field's
// name is what the breadcrumb says, and the sentence is what somebody answers.
func (m screen) formFor(doing string) *huh.Form {
	a, l := m.asked, m.s.lang
	running := l.nameOf(m.shown.projects, m.shown.clock.ProjectID)

	var steps []*huh.Group

	switch doing {
	case "start":
		steps = []*huh.Group{huh.NewGroup(m.projectField(&a.project)), huh.NewGroup(m.describe(&a.description))}
	case "switch":
		project := m.projectField(&a.project).
			Description(l.text("track.form.switchNote", "What has run so far is booked on {0}.", running))
		steps = []*huh.Group{huh.NewGroup(project), huh.NewGroup(m.describe(&a.description))}
	case "settings":
		steps = m.settingSteps(a)
	case "filter":
		steps = m.filterSteps(a)
	case "delete":
		steps = []*huh.Group{m.deleteStep(a)}
	case "assign":
		steps = []*huh.Group{m.assignStep(a)}
	case "deleteSome":
		steps = []*huh.Group{m.deleteSomeStep(a)}
	case "adjust", "end":
		steps = m.clockSteps(doing, a)
	case "discard":
		steps = []*huh.Group{m.discardStep(a)}
	case "bookTemplate":
		steps = []*huh.Group{m.templateStep(a)}
	case "tplNew":
		steps = m.newTemplateSteps(a)
	case "tplDelete":
		steps = []*huh.Group{m.templateDeleteStep(a)}
	case "export", "import":
		steps = []*huh.Group{m.fileStep(doing, a)}
	case "cell":
		steps = m.cellSteps(a)
	case "addRow":
		steps = []*huh.Group{m.addRowStep(a)}
	case "workNew", "workEdit":
		steps = m.workSteps(doing, a)
	case "workDelete":
		steps = []*huh.Group{m.workDeleteStep(a)}
	case "workPDF":
		steps = []*huh.Group{m.fileStep("export", a)}
	case "copyWeek":
		steps = []*huh.Group{m.copyStep(a)}
	case "stop":
		steps = []*huh.Group{huh.NewGroup(huh.NewConfirm().Key("stop").
			Title(l.text("track.form.stopQuestion", "Book {0} on {1}?", hoursAndMinutes(m.elapsed().Hours()), running)).
			Affirmative(l.text("track.form.stopYes", "Stop and book")).Negative(l.text("track.form.stopNo", "Keep it running")).Value(&a.stop))}
	default:
		steps = []*huh.Group{
			huh.NewGroup(huh.NewInput().Key("hours").Title(l.text("track.ask.hours", "How many hours?")).
				Placeholder(l.text("track.form.hoursHint", "1.5, 1,5 or 1:30")).Validate(l.readableHours).Value(&a.hours)),
			huh.NewGroup(m.projectField(&a.project)),
			huh.NewGroup(huh.NewInput().Key("day").Title(l.text("track.ask.day", "On which day?")).
				Placeholder(l.text("track.form.dayHint", "YYYY-MM-DD")).Validate(l.readableDay).Value(&a.day)),
			huh.NewGroup(m.describe(&a.description)),
		}
	}

	// Escape puts a form away; ctrl+c, which huh would use, closes the screen.
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", l.text("track.key.cancel", "cancel")))
	l.keyHelp(keys)

	// The screen's answer to whether the terminal is dark rather than the form's
	// own: the terminal says so once, before any form is open, and is not asked
	// again.
	dark := m.darkness()

	return huh.NewForm(steps...).
		WithTheme(huh.ThemeFunc(func(bool) *huh.Styles { return formTheme(dark) })).
		WithKeyMap(keys).
		WithShowHelp(false).
		WithWidth(m.inner())
}

// projectField picks one of the account's active projects, or none - and for an
// entry being corrected the one it is on, whatever has become of that project
// since, so that a correction keeps it unless it is changed.
func (m screen) projectField(into *uint) *huh.Select[uint] {
	l := m.s.lang
	options := []huh.Option[uint]{huh.NewOption(l.text("ts.noProject", "No project"), uint(0))}

	for _, p := range m.shown.projects {
		kept := m.doing == "edit" && m.editing.ProjectID != nil && *m.editing.ProjectID == p.ID

		if p.Status == "active" || kept {
			options = append(options, huh.NewOption(p.Name, p.ID))
		}
	}

	field := huh.NewSelect[uint]().Key("project").Title(l.text("track.ask.project", "Which project does the time go on?")).
		Options(options...).Value(into)

	// The height counts the title's line.
	if len(options) > projectRows {
		field = field.Height(projectRows + 1)
	}

	return field
}

// describe asks what the time went on, which may be nothing.
func (m screen) describe(into *string) *huh.Input {
	l := m.s.lang

	return huh.NewInput().Key("description").Title(l.text("track.ask.description", "What does it go on?")).
		Placeholder(l.text("track.form.optional", "optional")).CharLimit(model.MaxDescriptionLength).Value(into)
}

// readableHours refuses hours written in none of the ways parseHours reads.
func (l language) readableHours(raw string) error {
	_, err := l.parseHours(strings.TrimSpace(raw))

	return err
}

// keyHelp says in this language what huh's keys do, in the line under a form:
// the keys of the three kinds of field these forms have. Enter confirms an
// answer wherever it stands, which is what the screen's own menu calls it too.
func (l language) keyHelp(keys *huh.KeyMap) {
	help := func(binding *key.Binding, doing string) {
		binding.SetHelp(binding.Help().Key, doing)
	}

	help(&keys.Input.AcceptSuggestion, l.text("track.huh.complete", "complete"))
	help(&keys.Input.Prev, l.text("track.huh.back", "back"))
	help(&keys.Input.Next, l.text("track.huh.next", "next"))
	help(&keys.Input.Submit, l.text("track.huh.confirm", "confirm"))

	help(&keys.Select.Prev, l.text("track.huh.back", "back"))
	help(&keys.Select.Next, l.text("track.huh.confirm", "confirm"))
	help(&keys.Select.Submit, l.text("track.huh.confirm", "confirm"))
	help(&keys.Select.Up, l.text("track.huh.up", "up"))
	help(&keys.Select.Down, l.text("track.huh.down", "down"))
	help(&keys.Select.Left, l.text("track.huh.left", "left"))
	help(&keys.Select.Right, l.text("track.huh.right", "right"))
	help(&keys.Select.Filter, l.text("track.huh.filter", "filter"))
	help(&keys.Select.SetFilter, l.text("track.huh.setFilter", "set filter"))
	help(&keys.Select.ClearFilter, l.text("track.huh.clearFilter", "clear filter"))
	help(&keys.Select.HalfPageUp, l.text("track.huh.halfPageUp", "½ page up"))
	help(&keys.Select.HalfPageDown, l.text("track.huh.halfPageDown", "½ page down"))
	help(&keys.Select.GotoTop, l.text("track.huh.goToStart", "go to start"))
	help(&keys.Select.GotoBottom, l.text("track.huh.goToEnd", "go to end"))

	help(&keys.Confirm.Prev, l.text("track.huh.back", "back"))
	help(&keys.Confirm.Next, l.text("track.huh.next", "next"))
	help(&keys.Confirm.Submit, l.text("track.huh.confirm", "confirm"))
	help(&keys.Confirm.Toggle, l.text("track.huh.toggle", "toggle"))
	help(&keys.Confirm.Accept, l.text("track.huh.yes", "Yes"))
	help(&keys.Confirm.Reject, l.text("track.huh.no", "No"))
}

// request is the change an answered form asks for, as it will be sent; nil
// where the answer was to change nothing.
func (m screen) request() tea.Cmd {
	switch m.doing {
	case "settings":
		return m.saveSetting()
	case "edit":
		return m.saveCorrection()
	case "delete":
		return m.deleteEntry()
	case "assign":
		return m.transferSome()
	case "deleteSome":
		return m.deleteSome()
	case "adjust", "end", "discard":
		return m.clockChange()
	case "tplNew":
		return m.saveTemplate()
	case "tplDelete":
		return m.deleteTemplate()
	case "export":
		return m.exportTo()
	case "import":
		return m.checkFile()
	case "cell":
		return m.saveCell()
	case "copyWeek":
		return m.copyWeek()
	case "workNew", "workEdit":
		return m.saveWorkDay()
	case "workDelete":
		return m.deleteWorkDay()
	case "workPDF":
		return m.exportWork()
	}

	ctx, s, a, shown := m.ctx, m.s, *m.asked, m.shown

	var picked *uint
	if a.project != 0 {
		picked = &a.project
	}

	name := s.lang.nameOf(shown.projects, picked)
	body := map[string]any{}

	if picked != nil {
		body["projectId"] = *picked
	}

	if text := strings.TrimSpace(a.description); text != "" {
		body["description"] = text
	}

	var change func() (string, error)

	switch m.doing {
	case "start":
		change = func() (string, error) { return s.startClock(ctx, body, name, shown.zone) }
	case "switch":
		change = func() (string, error) { return s.switchClock(ctx, body, name, shown.projects) }
	case "stop":
		if !a.stop {
			return nil
		}

		change = func() (string, error) { return s.stopClock(ctx, shown.projects, "") }
	case "book":
		worked, err := s.lang.parseHours(strings.TrimSpace(a.hours))
		if err != nil {
			return func() tea.Msg { return done{err: err} }
		}

		body["date"], body["durationHours"] = strings.TrimSpace(a.day), worked
		change = func() (string, error) { return s.bookHours(ctx, body, name) }
	default:
		return nil
	}

	return func() tea.Msg {
		said, err := change()

		return done{said: said, err: err}
	}
}

// formTheme is huh's base theme in the screen's look: the question in the teal,
// the answer the cursor is on in the accent behind a >, the others plain, and
// the keys under it as quiet as the menu's - without huh's own bar beside the
// field, because the screen's frame is around it already.
func formTheme(dark bool) *huh.Styles {
	c := tonesFor(dark)
	t := huh.ThemeBase(dark)

	t.Focused.Base = lipgloss.NewStyle()
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(c.ask)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(c.accent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(c.muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(c.danger)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(c.danger)
	t.Focused.SelectSelector = t.Focused.SelectSelector.SetString("> ").Foreground(c.accent).Bold(true)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(c.accent)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(c.accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(c.accent).Bold(true)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(c.onAccent).Background(c.accent)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(c.muted).Background(c.line)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(c.accent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(c.muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(c.accent)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.Title = t.Blurred.Title.Foreground(c.muted)
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description

	t.Help.ShortKey = t.Help.ShortKey.Foreground(c.muted)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(c.muted).Faint(true)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(c.muted).Faint(true)

	return t
}
