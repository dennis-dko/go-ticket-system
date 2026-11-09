package cli

import (
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// choice is one line of the menu: what it does, what it is called, and the key
// that does it without moving there. The keys stay the same in every language,
// so a hand that knows them does not have to learn them again.
type choice struct{ key, doing, called string }

// choices are the menu: what the tab on screen offers, or the card whose menu is
// open under it and the way back, and reading the day again and closing the
// screen, which every tab offers. A tab offers nothing before the day is read or
// while a change is on its way.
func (m screen) choices() []choice {
	l := m.s.lang

	var changes []choice

	if m.loaded && !m.busy {
		switch {
		case m.sub == subEntries:
			changes = append(m.entryChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subClock:
			changes = append(m.clockChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subTemplates:
			changes = append(m.templateChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subWork:
			changes = append(m.workChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subLeave:
			changes = append(m.leaveChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subAway:
			changes = append(m.awayChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subWorkbook:
			changes = append(m.workbookChoices(), choice{"esc", "back", l.text("track.menu.back", "Back")})
		case m.sub == subYear:
			changes = []choice{{"esc", "back", l.text("track.menu.back", "Back")}}
		case m.currentTab().view == viewAccount:
			changes = m.settingChoices()
		case m.currentTab().view == viewWeek:
			changes = m.weekChoices()
		case m.currentTab().view == viewCalendar:
			changes = m.calendarChoices()
		case m.currentTab().view == viewOvertime:
			changes = m.overtimeChoices()
		default:
			changes = m.timesheetChoices()
		}
	}

	return append(changes,
		choice{"r", "refresh", l.text("track.menu.refresh", "Read the day again")},
		choice{"q", "quit", l.text("track.menu.quit", "Close the screen")})
}

// press answers a key while no form is open: a move to another tab or another
// week, a move through the menu or through the entries, enter on the line moved
// to, escape out of a card's menu, or the key of a change. While an entry or a
// cell is being picked the keys pick it, and while a history is shown they only
// close it.
func (m screen) press(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.past != nil:
		if k.String() == "esc" || k.String() == "enter" {
			m.past = nil
		}

		return m, nil
	case m.picking == "cell":
		return m.cellKey(k)
	case m.picking == "showDay":
		return m.dayKey(k)
	case m.picking != "":
		return m.pickKey(k)
	}

	choices := m.choices()
	at := min(m.cursor, len(choices)-1)

	if next, cmd, ok := m.moveKey(k.String()); ok {
		return next, cmd
	}

	switch k.String() {
	case "left", "shift+tab":
		return m.toTab(-1)
	case "right", "tab":
		return m.toTab(1)
	case "pgup", "pgdown":
		return m.page(k.String() == "pgdown"), nil
	case "esc":
		return m.choose("back")
	case "up", "k":
		m.cursor = max(at-1, 0)

		return m, nil
	case "down", "j":
		m.cursor = min(at+1, len(choices)-1)

		return m, nil
	case "enter":
		return m.choose(choices[at].doing)
	}

	for _, c := range choices {
		if c.key == k.String() {
			return m.choose(c.doing)
		}
	}

	return m, nil
}

// choose does what a line of the menu offers: opening a card's menu or going
// back from one, a change the card makes without asking, or the form for one
// that asks first.
func (m screen) choose(doing string) (tea.Model, tea.Cmd) {
	switch doing {
	case "quit":
		return m, tea.Quit
	case "refresh":
		if m.reading {
			return m, nil
		}

		return m.ask(time.Now())
	case "back":
		m.sub, m.cursor = "", 0

		return m, nil
	case "cell":
		return m.startCell()
	case "showDay":
		return m.startDay()
	case "closeDay":
		m.cal.day, m.cursor = "", 0

		return m, nil
	}

	if sub, ok := strings.CutPrefix(doing, "menu:"); ok {
		m.sub, m.cursor = sub, 0

		// The year is read when its card opens, and with the day only while it is
		// open: two years of days are the most any read on the screen asks for.
		if sub == subYear {
			return m, m.readYearNow()
		}

		return m, nil
	}

	if setting, ok := strings.CutPrefix(doing, "setting:"); ok {
		return m.openSetting(setting)
	}

	if next, cmd, ok := m.chooseOnCard(doing); ok {
		return next, cmd
	}

	return m.open(doing)
}

// open puts up the form for a change, with what the last one said cleared away.
// Booking by hand starts with the templates where there are any it can use, as
// the page's booking form starts with its picker.
func (m screen) open(doing string) (tea.Model, tea.Cmd) {
	m.news, m.failure = "", ""
	asked := &answers{day: m.readAt.In(m.shown.zone).Format(time.DateOnly), stop: true}

	if doing == "book" && slices.ContainsFunc(m.templates, m.usable) {
		doing = "bookTemplate"
	}

	switch doing {
	case "filter":
		asked = m.filterAnswers()
	case "edit":
		asked = m.correctionAnswers()
	case "adjust", "end", "discard":
		asked = m.clockAnswers(doing)
	case "export":
		asked.file = m.exportName()
	case "assign":
		// The first of the projects offered, rather than none, which is not one
		// of them.
		if active := m.activeProjects(); len(active) > 0 {
			asked.project = active[0].ID
		}
	case "workNew", "workEdit", "workDelete":
		asked = m.workAnswers(doing)
	case "workPDF":
		asked.file = m.workFileName()
	case "leaveNew":
		asked.leaveYear = strconv.Itoa(m.leaveYearShown())
	case "statsPeriod":
		asked.statsFrom, asked.statsTo = m.statsPeriod()
	case "statsPDF":
		asked.file = m.statsFileName()
	case "awayNew":
		today := m.today().Format(time.DateOnly)
		asked.awayFrom, asked.awayTo, asked.awayKind = today, today, absenceKinds[0]
	case "addRow":
		if addable := m.addable(m.weekRows()); len(addable) > 0 {
			asked.row = addable[0].key
		}
	}

	return m.openWith(doing, asked)
}

// openWith puts up the form for a change with the answers it starts from.
func (m screen) openWith(doing string, asked *answers) (tea.Model, tea.Cmd) {
	m.doing, m.asked = doing, asked
	m.form = m.formFor(doing)

	return m, m.form.Init()
}

// fillIn hands a message to the open form, and sends the change once the form
// is answered. A cell's form goes back to the grid whatever its answer, with
// the cursor where it was, as the page keeps the focus in the grid for the next
// cell.
func (m screen) fillIn(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)

	if m.form.State != huh.StateNormal && m.doing == "cell" {
		m.picking = "cell"
	}

	switch m.form.State {
	case huh.StateAborted:
		m.form = nil

		return m, nil
	case huh.StateCompleted:
		m.form = nil

		// A filter changes what the screen shows, not the installation, and so
		// does a row added to the week; a template only fills the booking in:
		// nothing is booked until its own form is answered.
		switch m.doing {
		case "filter":
			return m.narrow(m.asked.filtered())
		case "addRow":
			return m.addRow(m.asked.row)
		case "statsPeriod":
			return m.choosePeriod(*m.asked)
		case "bookTemplate":
			return m.openWith("book", m.fromTemplate(m.asked.template, m.asked.day))
		}

		change := m.request()
		if change == nil {
			return m, nil
		}

		m.busy = true

		return m, change
	}

	return m, cmd
}

// menu is the choices, the one the cursor is on in the accent behind a >, each
// with its key at the end of its line.
func (m screen) menu(look palette, width int) string {
	choices := m.choices()
	at := min(m.cursor, len(choices)-1)
	rows := make([]string, len(choices))

	for i, c := range choices {
		line := "  " + c.called
		if i == at {
			line = look.title.Render("> " + c.called)
		}

		rows[i] = spread(width, line, look.key.Render(c.key))
	}

	return strings.Join(rows, "\n")
}

// help names the keys that move between the tabs and through the menu, the way
// huh names its own under a form - so the menu and the forms say it alike. A
// change's own key stands at the end of its line instead. While an entry is
// being picked it names the keys that pick it, and under a history the one that
// closes it.
func (m screen) help(look palette) string {
	l := m.s.lang

	var keys [][2]string

	switch {
	case m.past != nil:
		keys = [][2]string{{"esc", l.text("history.close", "Close")}}
	case m.picking == "cell":
		keys = [][2]string{{"←/→", l.text("track.key.day", "day")}, {"↑/↓", l.text("track.key.row", "row")},
			{"enter", l.text("track.huh.confirm", "confirm")}, {"esc", l.text("track.key.cancel", "cancel")}}
	case m.picking == "showDay":
		keys = [][2]string{{"←/→", l.text("track.key.day", "day")}, {"↑/↓", l.text("track.key.week", "week")},
			{"enter", l.text("track.huh.confirm", "confirm")}, {"esc", l.text("track.key.cancel", "cancel")}}
	case m.picking != "":
		keys = [][2]string{{"↑", l.text("track.huh.up", "up")}, {"↓", l.text("track.huh.down", "down")}}

		if m.several() {
			keys = append(keys, [2]string{"space", l.text("track.key.tick", "tick")})
		}

		keys = append(keys, [2]string{"enter", l.text("track.huh.confirm", "confirm")},
			[2]string{"esc", l.text("track.key.cancel", "cancel")})
	default:
		if len(m.openTabs()) > 1 {
			keys = append(keys, [2]string{"←/→", l.text("track.key.tabs", "tabs")})
		}

		switch m.currentTab().view {
		case viewWeek:
			keys = append(keys, [2]string{"</>", l.text("track.key.weeks", "weeks")})

			if m.weekShown != "" {
				keys = append(keys, [2]string{"t", l.text("week.thisWeek", "This week")})
			}
		case viewCalendar:
			if m.sub == subLeave {
				keys = append(keys, [2]string{"</>", l.text("track.key.years", "years")})

				break
			}

			keys = append(keys, [2]string{"</>", l.text("track.key.months", "months")})

			if m.cal.shown != "" {
				keys = append(keys, [2]string{"t", l.text("cal.today", "Today")})
			}
		case viewOvertime:
			if m.sub == subYear {
				keys = append(keys, [2]string{"</>", l.text("track.key.years", "years")})
			}
		}

		keys = append(keys,
			[2]string{"↑", l.text("track.huh.up", "up")},
			[2]string{"↓", l.text("track.huh.down", "down")},
			[2]string{"enter", l.text("track.huh.confirm", "confirm")})
	}

	named := make([]string, len(keys))
	for i, k := range keys {
		named[i] = look.muted.Render(k[0]+" ") + look.muted.Faint(true).Render(k[1])
	}

	return strings.Join(named, look.muted.Faint(true).Render(" • "))
}
