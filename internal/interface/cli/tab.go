package cli

import (
	"context"
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
	viewCalendar   = "calendar"
	viewOvertime   = "overtime"
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
	{viewCalendar, model.PermTimesheetReadOwn},
	{viewOvertime, model.PermTimesheetReadOwn},
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

// tabAt is what the tab on screen reads besides the day, as it stands when the
// read is asked for: which tab, the week moved to, the month and the year of
// leave, the period of hours, and the year of tiles while their card is open -
// zero while it is not. One value rather than a list of strings side by side,
// where two swapped would still compile.
type tabAt struct {
	view, monday, first, from, to string
	year, tiles                   int
}

// tabAt is what a read of the day asks for besides, now.
func (m screen) tabAt() tabAt {
	from, to := m.statsPeriod()

	at := tabAt{view: m.currentTab().view, monday: m.weekMonday(), first: m.calendarMonth(), from: from, to: to,
		year: m.leaveYearShown()}

	if m.sub == subYear {
		at.tiles = m.tilesYear()
	}

	return at
}

// tabRead is what a read of the day brings besides for the tab on screen: the
// week's grid on the week's tab, the month and the cards beside it on the
// calendar's, and the hours of a period and the year on the overtime's - with
// what they were asked for.
type tabRead struct {
	week                            weekCard
	month                           calendarCard
	leave                           leaveYear
	away                            []absence
	stats                           statistics
	year                            yearDays
	asked                           tabAt
	weeked, monthed, statsd, yeared bool
}

// readTab reads what the tab on screen shows besides the day.
func (s *session) readTab(ctx context.Context, at tabAt) (tabRead, error) {
	var err error

	read := tabRead{asked: at}

	switch at.view {
	case viewWeek:
		read.week, err = s.readWeek(ctx, at.monday)
		read.weeked = err == nil
	case viewOvertime:
		read.stats, err = s.readStatistics(ctx, at.from, at.to)
		read.statsd = err == nil

		if err == nil && at.tiles != 0 {
			read.year, err = s.readYear(ctx, at.tiles)
			read.yeared = err == nil
		}
	case viewCalendar:
		if read.month, err = s.readMonth(ctx, at.first); err == nil {
			read.leave, err = s.readLeave(ctx, at.year)
		}

		if err == nil {
			read.away, err = s.readAway(ctx)
		}

		read.monthed = err == nil
	}

	return read, err
}

// tookTab keeps what a read brought for the tab, as it was asked for, as the
// entries card is kept: one read for a week or a month the tab has moved on from
// belongs to a tab that is gone.
func (m screen) tookTab(read tabRead) screen {
	if read.weeked && read.week.monday == m.weekMonday() {
		m.week = read.week
	}

	if read.monthed && read.month.first == m.calendarMonth() {
		m.cal.month = read.month
	}

	if read.monthed && read.leave.Year == m.leaveYearShown() {
		m.cal.leave = read.leave
	}

	if read.monthed {
		m.cal.away = read.away
	}

	if from, to := m.statsPeriod(); read.statsd && read.asked.from == from && read.asked.to == to {
		m.ot.stats = read.stats
	}

	if read.yeared && read.year.year == m.tilesYear() {
		m.ot.year = read.year
	}

	return m
}

// toTab moves by steps through the tabs, and reads the week where it moves to
// the week's: one read when the tab was last open may have been booked on since,
// in the browser or under the time entries.
func (m screen) toTab(by int) (tea.Model, tea.Cmd) {
	m = m.moveTab(by)

	switch m.currentTab().view {
	case viewWeek:
		return m, m.readWeekNow()
	case viewCalendar:
		return m, tea.Batch(m.readMonthNow(), m.readLeaveNow(), m.readAwayNow())
	case viewOvertime:
		return m, m.readStatsNow()
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
	case viewCalendar:
		return m.calendarBody(look, width)
	case viewOvertime:
		return m.overtimeBody(look, width, rows)
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
	case viewCalendar:
		return l.text("nav.calendar", "Calendar")
	case viewOvertime:
		return l.text("nav.overtime", "Overtime")
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
