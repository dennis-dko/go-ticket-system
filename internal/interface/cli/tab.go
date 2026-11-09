package cli

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// tab is one of the web interface's tabs as the screen offers it: the view it
// shows, named as the page names it, and the right the page asks for before it
// offers it - empty for one every account has. What it is called is tabName's.
type tab struct {
	view, perm string
}

// The views the screen has been taught, named as the page names them.
const (
	viewTimesheets = "timesheets"
	viewWeek       = "week"
	viewAccount    = "settings"
)

// tabs are the page's tabs in the page's order, as far as the screen has been
// taught them. The terminal learns the page an area at a time - decided by the
// maintainer in November 2026 - and a tab is listed once its area does what the
// page's does: a tab that opened onto nothing would offer what the screen
// cannot do.
var tabs = []tab{
	{viewTimesheets, model.PermTimesheetReadOwn},
	{viewWeek, model.PermTimesheetReadOwn},
	{viewAccount, ""},
}

// openTabs are the tabs this account may open, decided as the page decides it,
// by the rights read with the day. None before the day is read.
func (m screen) openTabs() []tab {
	if !m.loaded {
		return nil
	}

	var open []tab

	for _, t := range tabs {
		if t.perm == "" || slices.Contains(m.shown.permissions, t.perm) {
			open = append(open, t)
		}
	}

	return open
}

// currentTab is the tab on screen: the one moved to, or the first this account
// may open where that one is not theirs - a right taken away in the browser
// takes its tab with it at the next read, as it does on the page. Until the day
// is read it is the one the screen opens on.
func (m screen) currentTab() tab {
	open := m.openTabs()

	for _, t := range open {
		if t.view == m.view {
			return t
		}
	}

	if len(open) > 0 {
		return open[0]
	}

	return tabs[0]
}

// moveTab moves by steps through the tabs this account may open, round the
// ends, and puts the cursor on the first line of the new tab's own menu.
func (m screen) moveTab(by int) screen {
	open := m.openTabs()
	if len(open) < 2 {
		return m
	}

	current := m.currentTab().view
	at := slices.IndexFunc(open, func(t tab) bool { return t.view == current })

	m.view, m.sub, m.cursor = open[(at+by+len(open))%len(open)].view, "", 0

	return m
}

// toTab moves by steps through the tabs, and reads the week where it moves to
// the week's: one read when the tab was last open may have been booked on since,
// in the browser or under the time entries.
func (m screen) toTab(by int) (tea.Model, tea.Cmd) {
	m = m.moveTab(by)

	if m.currentTab().view == viewWeek {
		return m, m.readWeekNow()
	}

	return m, nil
}

// tabBody is what the tab on screen shows above its menu, with rows of the
// entries. My account shows its settings in the menu itself, each with what it
// is set to now.
func (m screen) tabBody(look palette, width, rows int) []string {
	switch m.currentTab().view {
	case viewTimesheets:
		return m.timesheetBody(look, width, rows)
	case viewWeek:
		return m.weekBody(look, width)
	}

	return nil
}

// menuPrompt is the question above the tab's menu.
func (m screen) menuPrompt() string {
	l := m.s.lang

	if m.currentTab().view == viewAccount {
		return l.text("track.settings.which", "What do you want to set?")
	}

	return l.text("track.menu.prompt", "Choose what to do next.")
}

// tabName is what the page heads a tab with, in this language.
func (m screen) tabName(t tab) string {
	l := m.s.lang

	switch t.view {
	case viewWeek:
		return l.text("nav.week", "Week")
	case viewAccount:
		return l.text("nav.settings", "My account")
	}

	return l.text("nav.timesheets", "Time entries")
}

// tabBar draws the tabs as the page does: side by side in the middle, each in
// the quiet colour and the one on screen filled with the accent. A row that
// would be wider than the frame goes on in another row rather than being cut
// short, as the page's does on a telephone.
func (m screen) tabBar(look palette, width int) string {
	current := m.currentTab().view

	var (
		rows, row []string
		used      int
	)

	for _, t := range m.openTabs() {
		style := look.tab
		if t.view == current {
			style = look.tabOn
		}

		label := style.Render(" " + m.tabName(t) + " ")
		wide := lipgloss.Width(label)

		if len(row) > 0 && used+1+wide > width {
			rows, row, used = append(rows, strings.Join(row, " ")), nil, 0
		}

		if len(row) > 0 {
			used++
		}

		row, used = append(row, label), used+wide
	}

	if len(row) > 0 {
		rows = append(rows, strings.Join(row, " "))
	}

	for i, r := range rows {
		rows[i] = lipgloss.PlaceHorizontal(width, lipgloss.Center, r)
	}

	return strings.Join(rows, "\n")
}
