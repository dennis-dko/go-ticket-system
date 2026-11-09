package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// workMonth is the month's working time as /me/workdays answers it: a day for
// every day times were recorded on or hours were booked on, and what they add
// up to.
type workMonth struct {
	Days        []workDay `json:"days"`
	TotalWorked float64   `json:"totalWorked"`
	TotalBooked float64   `json:"totalBooked"`
}

// workDay is one day of it: whether its times were recorded, which a day only
// booked on has not, its start and end as a clock shows them, its breaks, the
// hours worked and booked, and the limits of the Working Hours Act it went past.
type workDay struct {
	Date         string       `json:"date"`
	Recorded     bool         `json:"recorded"`
	Start        string       `json:"start"`
	End          string       `json:"end"`
	BreakMinutes int          `json:"breakMinutes"`
	WorkedHours  float64      `json:"workedHours"`
	BookedHours  float64      `json:"bookedHours"`
	Notices      []workNotice `json:"notices"`
}

// workNotice is a limit of the Working Hours Act a day went past, and the
// minutes the rule names.
type workNotice struct {
	Rule    string `json:"rule"`
	Minutes int    `json:"minutes"`
}

// readWork reads the working time of the month from its first day.
func (s *session) readWork(ctx context.Context, first string) (workMonth, error) {
	var month workMonth

	q := url.Values{"from": {first}, "to": {lastOfMonth(first)}}
	err := s.api.call(ctx, http.MethodGet, "/me/workdays?"+q.Encode(), nil, &month)

	return month, err
}

// workChoices are the working time's menu, as the page's card offers it:
// recording a day, correcting one and taking back one that has times - for an
// account that may book its own time - and the card as a document, for anybody
// who sees it.
func (m screen) workChoices() []choice {
	l, days := m.s.lang, m.cal.month.work.Days

	var choices []choice

	if m.mayWrite() {
		choices = append(choices, choice{"n", "workNew", l.text("track.work.record", "Record a day")})

		if len(days) > 0 {
			choices = append(choices, choice{"e", "workEdit", l.text("track.work.correct", "Correct a day")})
		}

		for _, day := range days {
			if day.Recorded {
				choices = append(choices, choice{"d", "workDelete", l.text("track.work.delete", "Take back a day")})

				break
			}
		}
	}

	return append(choices, choice{"p", "workPDF", l.text("action.exportPdf", "Export as PDF")})
}

// pickingWorkDay reports whether what is being picked is a day of the working
// time.
func (m screen) pickingWorkDay() bool {
	return m.picking == "workEdit" || m.picking == "workDelete"
}

// workAnswers are the answers a working time's form starts from: a day picked
// filled in with what it holds, a new one with today and no breaks. A day with no
// start yet is offered the running stopwatch's, where the clock started on it -
// the page offers it the same way, and nothing is kept until the form is saved.
func (m screen) workAnswers(doing string) *answers {
	a := &answers{workDay: m.today().Format(time.DateOnly), workBreaks: "0"}

	if doing != "workNew" {
		day := m.cal.picked
		a.workDay = day.Date

		if day.Recorded {
			a.workStart, a.workEnd, a.workBreaks = day.Start, day.End, strconv.Itoa(day.BreakMinutes)
		}
	}

	if clock := m.shown.clock; a.workStart == "" && clock.Running && clock.StartedAt != nil {
		if started := clock.StartedAt.In(m.today().Location()); started.Format(time.DateOnly) == a.workDay {
			a.workStart = started.Format("15:04")
		}
	}

	return a
}

// workSteps ask for a day's times a question at a time: which day, for a new one,
// then its start, its end - which may be midnight, as 24:00 - and its breaks in
// minutes.
func (m screen) workSteps(doing string, a *answers) []*huh.Group {
	l := m.s.lang

	clock := func(key, question string, into *string, end bool) *huh.Input {
		return huh.NewInput().Key(key).Title(question).Placeholder("08:00").Value(into).
			Validate(func(raw string) error {
				if _, ok := clockTime(raw, end); !ok {
					return errors.New(l.text("track.work.badTime", "Write a time as a clock shows it, such as 08:00."))
				}

				return nil
			})
	}

	start := clock("workStart", l.text("track.ask.start", "When did the day begin?"), &a.workStart, false)

	var steps []*huh.Group

	if doing == "workNew" {
		steps = append(steps, huh.NewGroup(huh.NewInput().Key("workDay").Title(l.text("track.ask.workDay", "Which day?")).
			Placeholder(l.text("track.form.dayHint", "YYYY-MM-DD")).Validate(l.readableDay).Value(&a.workDay)))
	} else {
		start = start.Description(l.date(a.workDay))
	}

	return append(steps,
		huh.NewGroup(start),
		huh.NewGroup(clock("workEnd", l.text("track.ask.end", "When did it end?"), &a.workEnd, true)),
		huh.NewGroup(huh.NewInput().Key("workBreaks").Title(l.text("track.ask.breaks", "How many minutes of breaks?")).
			Validate(l.readableBreaks).Value(&a.workBreaks)))
}

// clockTime reads a time of day as a clock shows it, the hour with its leading
// nought or without, and writes it as the page's field does: 08:00. The end of a
// day may be 24:00, its midnight.
func clockTime(raw string, end bool) (string, bool) {
	hour, minute, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok || len(hour) < 1 || len(hour) > 2 || len(minute) != 2 || !digits(hour+minute) {
		return "", false
	}

	h, _ := strconv.Atoi(hour)
	m, _ := strconv.Atoi(minute)

	switch {
	case m > 59, h > 24, h == 24 && (!end || m != 0):
		return "", false
	}

	return fmt.Sprintf("%02d:%02d", h, m), true
}

// digits reports whether text is nothing but the digits 0 to 9.
func digits(text string) bool {
	return strings.Trim(text, "0123456789") == ""
}

// readableBreaks refuses breaks that are not whole minutes of a day, as the
// page's field takes them: 0 to 1439.
func (l language) readableBreaks(raw string) error {
	minutes, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || minutes < 0 || minutes > 1439 || !digits(strings.TrimSpace(raw)) {
		return errors.New(l.text("track.work.badBreaks", "Write the breaks as whole minutes, from 0 to 1439."))
	}

	return nil
}

// workDeleteStep asks before a day's times are taken back, as the page's delete
// button asks, naming the day.
func (m screen) workDeleteStep(a *answers) *huh.Group {
	l := m.s.lang

	return huh.NewGroup(huh.NewConfirm().Key("confirm").
		Title(l.text("confirm.deleteTitle", "Delete for good?")).
		Description(l.date(a.workDay) + " " + l.text("confirm.deleteText", "will be deleted. This cannot be undone.")).
		Affirmative(l.text("track.form.deleteYes", "Delete")).Negative(l.text("action.cancel", "Cancel")).
		Value(&a.confirmed))
}

// saveWorkDay records a day's times in place of what it had, as the page's form
// sends them.
func (m screen) saveWorkDay() tea.Cmd {
	ctx, s, a := m.ctx, m.s, *m.asked

	start, _ := clockTime(a.workStart, false)
	end, _ := clockTime(a.workEnd, true)
	breaks, _ := strconv.Atoi(strings.TrimSpace(a.workBreaks))

	path := "/me/workdays/" + url.PathEscape(strings.TrimSpace(a.workDay))
	body := map[string]any{"start": start, "end": end, "breakMinutes": breaks}

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodPut, path, body, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("work.saved", "Working time recorded")}
	}
}

// deleteWorkDay takes a day's times back, once the answer was yes.
func (m screen) deleteWorkDay() tea.Cmd {
	if !m.asked.confirmed {
		return nil
	}

	ctx, s, path := m.ctx, m.s, "/me/workdays/"+url.PathEscape(m.asked.workDay)

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodDelete, path, nil, nil); err != nil {
			return done{err: err}
		}

		return done{said: s.lang.text("work.removed", "Working time taken back")}
	}
}

// workFileName is the name the page gives the working time as a document: its
// word for the file and today.
func (m screen) workFileName() string {
	return m.s.lang.text("work.filename", "working-time") + "-" + m.today().Format(time.DateOnly) + ".pdf"
}

// exportWork lays out the working time as the page's document, in the client's
// language, and saves it where it was asked to go.
func (m screen) exportWork() tea.Cmd {
	ctx, s, path, document := m.ctx, m.s, strings.TrimSpace(m.asked.file), m.workDocument()

	return func() tea.Msg {
		pdf, err := s.api.render(ctx, "/exports/document?lang="+s.lang.code(), document)
		if err != nil {
			return done{err: err}
		}

		return s.saved(path, pdf)
	}
}

// workDocument is the working time as the page's workDayDocument describes it:
// the card's table, every column but the actions, under the month it is for, and
// what the month came to. Plain, because the colours are the page's to read off
// its screen and a terminal has none to hand on.
func (m screen) workDocument() map[string]any {
	l, month := m.s.lang, m.cal.month.work
	title := l.text("work.title", "Working time")

	rows := make([][]string, len(month.Days))
	for i, day := range month.Days {
		notices := make([]string, len(day.Notices))
		for j, notice := range day.Notices {
			notices[j] = l.actNotice(notice)
		}

		rows[i] = append(m.workCells(day), strings.Join(notices, "; "))
	}

	return map[string]any{
		"title":    title,
		"subtitle": m.monthTitle(),
		"sections": []map[string]any{{
			"heading": title,
			"caption": l.text("work.documentCaption",
				"A record kept by its owner for themselves: when each day began and finished, its breaks, and the hours booked on it."),
			"table": map[string]any{
				"columns": append(m.workHeadings(), l.text("work.act", "Working Hours Act")),
				"numeric": []bool{false, false, false, true, true, true, false},
				"rows":    rows,
			},
		}},
		"summary": []map[string]string{
			{"label": l.text("work.worked", "Worked"), "value": l.hours(month.TotalWorked)},
			{"label": l.text("work.booked", "Booked"), "value": l.hours(month.TotalBooked)},
		},
	}
}

// workHeadings are the card's columns of figures, as the page heads them.
func (m screen) workHeadings() []string {
	l := m.s.lang

	return []string{l.text("field.date", "Date"), l.text("field.start", "Start"), l.text("field.end", "End"),
		l.text("field.breakMinutes", "Breaks (minutes)"), l.text("work.worked", "Worked"), l.text("work.booked", "Booked")}
}

// workCells are a day's figures as the page's table writes them: a day booked on
// without times has a dash where its times would be.
func (m screen) workCells(day workDay) []string {
	l := m.s.lang

	if !day.Recorded {
		return []string{l.date(day.Date), "–", "–", "–", "–", l.hours(day.BookedHours)}
	}

	return []string{l.date(day.Date), day.Start, day.End, strconv.Itoa(day.BreakMinutes), l.hours(day.WorkedHours),
		l.hours(day.BookedHours)}
}

// actNotice says a limit of the Working Hours Act a day went past as the page's
// actNotice says it, as a sentence rather than a mark, which would mean nothing
// on paper.
func (l language) actNotice(notice workNotice) string {
	switch notice.Rule {
	case "longDay":
		return l.text("work.longDay", "More than ten hours (Working Hours Act § 3)")
	case "shortBreak":
		return l.text("work.shortBreak", "{0} minutes of breaks needed (Working Hours Act § 4)", strconv.Itoa(notice.Minutes))
	case "shortRest":
		return l.text("work.shortRest", "Only {0} of rest since the day before (Working Hours Act § 5)",
			l.hours(float64(notice.Minutes)/60))
	}

	return notice.Rule
}

// workCardView draws the working time as the page's card does: the month it is
// for, a row a day under the page's columns with the limits a day went past said
// under it, what the month came to, and what the marks mean. While a day is
// picked the cursor stands before it, and one that cannot be picked is quiet.
func (m screen) workCardView(look palette, width int) string {
	l, month := m.s.lang, m.cal.month.work
	lines := []string{spread(width, look.title.Render(l.text("work.title", "Working time")), look.muted.Render(m.monthTitle()))}

	if len(month.Days) == 0 {
		lines = append(lines, look.muted.Render(l.text("work.empty", "Nothing recorded or booked this month.")))
	} else {
		table := grid{headings: m.workHeadings(), right: []bool{false, false, false, true, true, true}}
		for _, day := range month.Days {
			table.rows = append(table.rows, m.workCells(day))
		}

		inner := width - 2
		lines = append(lines, "  "+table.heading(look, inner))

		past := 0

		for i, line := range table.lines(inner) {
			cursor := " "

			if m.pickingWorkDay() && i == m.row {
				cursor = look.title.Render(">")
			}

			if m.pickingWorkDay() && !m.pickableAt(i) {
				line = look.muted.Render(ansi.Strip(line))
			}

			lines = append(lines, cursor+" "+line)

			for _, notice := range month.Days[i].Notices {
				lines = append(lines, "    "+look.ask.Render("! "+l.actNotice(notice)))
			}

			if len(month.Days[i].Notices) > 0 {
				past++
			}
		}

		said := l.text("work.total", "Worked {0}, booked {1}", l.hours(month.TotalWorked), l.hours(month.TotalBooked))
		if past > 0 {
			said += " · " + l.text("work.noticeCount", "{0} days past a limit of the Working Hours Act.", strconv.Itoa(past))
		}

		lines = append(lines, lipgloss.NewStyle().Width(width).Render(look.muted.Render(said)))
	}

	hint := l.text("work.actHint", "Days past a limit of the Working Hours Act are marked: more than ten hours, a break too short for the hours, or fewer than eleven hours of rest since the day before. They are notices; what happened is what is recorded.")

	return strings.Join(append(lines, lipgloss.NewStyle().Width(width).Render(look.muted.Render(hint))), "\n")
}

// monthTitle is the calendar's month as its heading names it.
func (m screen) monthTitle() string {
	first, err := time.Parse(time.DateOnly, m.cal.month.first)
	if err != nil {
		return m.cal.month.first
	}

	return m.s.lang.monthName(first) + " " + strconv.Itoa(first.Year())
}
