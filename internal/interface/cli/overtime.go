package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// overtimeState is the overtime's tab: the hours of the period last read, the
// period chosen - empty for the page's own - and the first of its days on screen
// where the window holds fewer than the period has; the year last read for the
// tiles, the year chosen - zero for this one - and the first of its months on
// screen; and the balance last calculated, its period chosen and the first of
// its days on screen, likewise.
type overtimeState struct {
	stats    statistics
	from, to string
	top      int

	year               yearDays
	yearShown, yearTop int

	balance                overtimeBalance
	balanceFrom, balanceTo string
	balanceTop             int
}

// statistics is a period's hours as /me/statistics answers them: every day of
// it, the empty ones included, each project's, each activity a description names
// with a # and what names none.
type statistics struct {
	From          string         `json:"from"`
	To            string         `json:"to"`
	TotalHours    float64        `json:"totalHours"`
	Days          []dayTotal     `json:"days"`
	Projects      []projectTotal `json:"projects"`
	Tags          []tagTotal     `json:"tags"`
	UntaggedHours float64        `json:"untaggedHours"`
}

// dayTotal is a day's hours.
type dayTotal struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

// projectTotal is a project's hours: no project id for the hours on none, and no
// name for a project deleted since.
type projectTotal struct {
	ProjectID *uint   `json:"projectId"`
	Name      string  `json:"name"`
	Hours     float64 `json:"hours"`
}

// tagTotal is an activity's hours.
type tagTotal struct {
	Tag   string  `json:"tag"`
	Hours float64 `json:"hours"`
}

// statsRead is a period's hours read on their own, with the period they were
// asked for, so that an answer landing after another period was chosen is not
// taken over the hours of the one chosen.
type statsRead struct {
	from, to string
	stats    statistics
	err      error
}

// readStatistics reads the hours of a period.
func (s *session) readStatistics(ctx context.Context, from, to string) (statistics, error) {
	var stats statistics

	err := s.api.call(ctx, http.MethodGet, "/me/statistics?"+url.Values{"from": {from}, "to": {to}}.Encode(), nil, &stats)

	return stats, err
}

// statsPeriod is the period the hours are asked for: the one chosen, or the
// page's own - the first of the month to today, in the owner's zone.
func (m screen) statsPeriod() (from, to string) {
	if m.ot.from != "" && m.ot.to != "" {
		return m.ot.from, m.ot.to
	}

	return firstOfMonth(m.today()), m.today().Format(time.DateOnly)
}

// readStatsNow reads the hours of the period shown, without the rest of the day.
func (m screen) readStatsNow() tea.Cmd {
	ctx, s := m.ctx, m.s
	from, to := m.statsPeriod()

	return func() tea.Msg {
		stats, err := s.readStatistics(ctx, from, to)

		return statsRead{from: from, to: to, stats: stats, err: err}
	}
}

// tookStats keeps a period's hours read on their own, where it is still the
// period shown.
func (m screen) tookStats(msg statsRead) screen {
	from, to := m.statsPeriod()

	switch {
	case msg.err != nil:
		m.stale = msg.err.Error()
	case msg.from == from && msg.to == to:
		m.ot.stats = msg.stats
	}

	return m
}

// overtimeChoices are the overtime's menu: choosing the period, the year's
// tiles, the balance, and the hours as a document.
func (m screen) overtimeChoices() []choice {
	l := m.s.lang

	return []choice{
		{"z", "statsPeriod", l.text("track.period", "Choose the period")},
		{"y", "menu:" + subYear, l.text("field.year", "Year") + " …"},
		{"s", "menu:" + subBalance, l.text("ot.balance", "Balance") + " …"},
		{"p", "statsPDF", l.text("action.exportPdf", "Export as PDF")},
	}
}

// periodSteps ask for the first and the last day of a period, as the page's two
// fields do: the fields named for the card - stats or balance - with From and
// To after it.
func (m screen) periodSteps(card string, from, to *string) []*huh.Group {
	l := m.s.lang

	day := func(key, question string, into *string) *huh.Group {
		return huh.NewGroup(huh.NewInput().Key(key).Title(question).
			Placeholder(l.text("track.form.dayHint", "YYYY-MM-DD")).Validate(l.readableDay).Value(into))
	}

	return []*huh.Group{
		day(card+"From", l.text("track.ask.from", "From which day on?"), from),
		day(card+"To", l.text("track.ask.to", "Up to which day?"), to),
	}
}

// choosePeriod puts the period answered on the card, from its first day, and
// reads its hours.
func (m screen) choosePeriod(a answers) (tea.Model, tea.Cmd) {
	m.ot.from, m.ot.to, m.ot.top = strings.TrimSpace(a.statsFrom), strings.TrimSpace(a.statsTo), 0

	return m, m.readStatsNow()
}

// overtimeBody is what the overtime shows above its menu: the hours of the period,
// once they have been read, or the year's card while its menu is open.
func (m screen) overtimeBody(look palette, width, rows int) []string {
	switch m.sub {
	case subYear:
		return m.yearBody(look, width, rows)
	case subBalance:
		return m.balanceBody(look, width, rows)
	}

	if from, to := m.statsPeriod(); m.ot.stats.From != from || m.ot.stats.To != to {
		return []string{look.muted.Render(m.s.lang.text("track.stats.reading", "Reading the hours …"))}
	}

	return []string{m.statsCardView(look, width, rows)}
}

// statsCardView draws a period's hours as the page's card draws them: headed by
// what it is and the period's total, and three charts - a bar a day, the empty
// days among them, a bar a project and a bar an activity with the hours naming
// none last. A period with more days than the window has room for shows rows of
// them from the one at the top, and says which.
func (m screen) statsCardView(look palette, width, rows int) string {
	l, stats := m.s.lang, m.ot.stats
	wrapped := lipgloss.NewStyle().Width(width)

	lines := []string{
		spread(width, look.title.Render(l.text("stats.title", "My hours")),
			look.muted.Render(l.text("stats.total", "Total")+": "+l.hours(stats.TotalHours))),
		look.muted.Render(l.date(stats.From) + " – " + l.date(stats.To)),
	}

	if stats.TotalHours <= 0 {
		return strings.Join(append(lines, look.muted.Render(l.text("stats.empty", "Nothing was recorded in this period."))), "\n")
	}

	labels, values := m.dayBars()
	rows = min(rows, len(labels))
	top := max(min(m.ot.top, len(labels)-rows), 0)

	lines = append(lines, look.ask.Render(l.text("stats.perDay", "Hours per day")))
	lines = append(lines, l.bars(look, width, labels[top:top+rows], values[top:top+rows], values)...)

	if rows < len(labels) {
		lines = append(lines, look.muted.Render(spread(width, "", l.place(top, rows, len(labels)))))
	}

	labels, values = m.projectBars()
	lines = append(lines, look.ask.Render(l.text("stats.perProject", "Hours per project")))
	lines = append(lines, l.bars(look, width, labels, values, values)...)

	labels, values = m.activityBars()
	lines = append(lines, look.ask.Render(l.text("stats.perActivity", "Hours per activity")),
		wrapped.Render(look.muted.Render(l.text("stats.activityHint", "Write an activity into a description with a # - #meeting, #review - and its hours are added up here. An entry naming two counts towards both, so these can add up to more than the total."))))

	return strings.Join(append(lines, l.bars(look, width, labels, values, values)...), "\n")
}

// dayBars are the day chart's bars: each day of the period and its hours, the
// empty days among them - a chart of only the days with entries shows a full
// week where there were two working days.
func (m screen) dayBars() ([]string, []float64) {
	labels := make([]string, len(m.ot.stats.Days))
	values := make([]float64, len(m.ot.stats.Days))

	for i, day := range m.ot.stats.Days {
		labels[i], values[i] = m.s.lang.date(day.Date), day.Hours
	}

	return labels, values
}

// projectBars are the project chart's bars, named as the page's projectBars names
// them: a project deleted since keeps its hours under a word for that.
func (m screen) projectBars() ([]string, []float64) {
	l := m.s.lang
	labels := make([]string, len(m.ot.stats.Projects))
	values := make([]float64, len(m.ot.stats.Projects))

	for i, project := range m.ot.stats.Projects {
		switch {
		case project.ProjectID == nil:
			labels[i] = l.text("stats.noProject", "No project")
		case project.Name == "":
			labels[i] = l.text("stats.deletedProject", "deleted project")
		default:
			labels[i] = project.Name
		}

		values[i] = project.Hours
	}

	return labels, values
}

// activityBars are the activity chart's bars, as the page's activityBars has
// them: each activity with its #, and the hours naming none last.
func (m screen) activityBars() ([]string, []float64) {
	var (
		labels []string
		values []float64
	)

	for _, tag := range m.ot.stats.Tags {
		labels, values = append(labels, "#"+tag.Tag), append(values, tag.Hours)
	}

	if untagged := m.ot.stats.UntaggedHours; untagged > 0 {
		labels = append(labels, m.s.lang.text("stats.noActivity", "No activity"))
		values = append(values, untagged)
	}

	return labels, values
}

// eighths are the blocks a bar ends in, an eighth of a column each.
var eighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// bars draws a chart's bars: each label, a bar as long as its value's share of
// the largest in scale, to an eighth of a column, and its hours at the right.
// scale is every value the chart has, so a page of days is drawn to the same
// length as the next.
func (l language) bars(look palette, width int, labels []string, values, scale []float64) []string {
	largest := 0.0
	for _, v := range scale {
		largest = max(largest, v)
	}

	labelWidth, valueWidth := 0, 0
	for i, label := range labels {
		labelWidth = max(labelWidth, lipgloss.Width(label))
		valueWidth = max(valueWidth, lipgloss.Width(l.hours(values[i])))
	}

	labelWidth = min(labelWidth, width/3)
	room := max(width-2-labelWidth-1-1-valueWidth, 1)

	lines := make([]string, len(labels))

	for i, label := range labels {
		bar := ""

		if largest > 0 {
			eighthsLong := int(values[i] / largest * float64(room*8))
			bar = strings.Repeat("█", eighthsLong/8) + eighths[eighthsLong%8]
		}

		lines[i] = "  " + fitLeft(label, labelWidth) + " " + look.title.Render(fitLeft(bar, room)) + " " +
			fitRight(l.hours(values[i]), valueWidth)
	}

	return lines
}

// statsFileName is the name the page gives the hours as a document: its word for
// the file and today.
func (m screen) statsFileName() string {
	return m.s.lang.text("stats.filename", "statistics") + "-" + m.today().Format(time.DateOnly) + ".pdf"
}

// exportStats lays out the hours as the page's statisticsDocument does: a
// section a chart, in the order they are on the card, under the period and the
// total - each chart as its figures, since a terminal draws no picture to hand on.
func (m screen) exportStats() tea.Cmd {
	l, stats := m.s.lang, m.ot.stats

	section := func(heading, column string, labels []string, values []float64) map[string]any {
		rows := make([][]string, len(labels))
		for i, label := range labels {
			rows[i] = []string{label, l.hours(values[i])}
		}

		return map[string]any{"heading": heading, "table": map[string]any{
			"columns": []string{column, l.text("field.hours", "Hours")}, "numeric": []bool{false, true}, "rows": rows,
		}}
	}

	days, dayHours := m.dayBars()
	projects, projectHours := m.projectBars()
	activities, activityHours := m.activityBars()

	document := map[string]any{
		"title":    l.text("stats.title", "My hours"),
		"subtitle": l.date(stats.From) + " – " + l.date(stats.To),
		"sections": []map[string]any{
			section(l.text("stats.perDay", "Hours per day"), l.text("field.date", "Date"), days, dayHours),
			section(l.text("stats.perProject", "Hours per project"), l.text("field.project", "Project"), projects, projectHours),
			section(l.text("stats.perActivity", "Hours per activity"), l.text("track.stats.activity", "Activity"),
				activities, activityHours),
		},
		"summary": []map[string]string{{"label": l.text("stats.total", "Total"), "value": l.hours(stats.TotalHours)}},
	}

	ctx, s, path := m.ctx, m.s, strings.TrimSpace(m.asked.file)

	return func() tea.Msg {
		pdf, err := s.api.render(ctx, "/exports/document?lang="+s.lang.code(), document)
		if err != nil {
			return done{err: err}
		}

		return s.saved(path, pdf)
	}
}
