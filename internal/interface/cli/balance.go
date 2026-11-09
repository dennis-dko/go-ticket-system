package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// subBalance names the balance's menu, opened from the overtime's own.
const subBalance = "balance"

// overtimeBalance is a period's balance as /users/{id}/overtime answers it: whose
// it is and what a day asks of them, every day that counts with what it came
// to, the totals, and from which day every working day counts - empty while
// only the days booked on do.
type overtimeBalance struct {
	UserName           string       `json:"userName"`
	From               string       `json:"from"`
	To                 string       `json:"to"`
	DailyTarget        float64      `json:"dailyTarget"`
	Days               []balanceDay `json:"days"`
	EveryDayCountsFrom string       `json:"everyDayCountsFrom"`
	TotalBooked        float64      `json:"totalBooked"`
	TotalTarget        float64      `json:"totalTarget"`
	TotalBalance       float64      `json:"totalBalance"`
}

// balanceDay is a day that counts towards a balance: the day as /me/days says
// it, and what was booked on it less what it asked.
type balanceDay struct {
	weekDay

	Balance float64 `json:"balance"`
}

// balanceRead is a period's balance read, with the period it was asked for, so
// that an answer landing after another period was chosen is not taken over the
// balance of the one chosen.
type balanceRead struct {
	from, to string
	balance  overtimeBalance
	err      error
}

// readBalance reads an account's balance over a period.
func (s *session) readBalance(ctx context.Context, account uint, from, to string) (overtimeBalance, error) {
	var balance overtimeBalance

	route := fmt.Sprintf("/users/%d/overtime?", account) + url.Values{"from": {from}, "to": {to}}.Encode()
	err := s.api.call(ctx, http.MethodGet, route, nil, &balance)

	return balance, err
}

// balancePeriod is the period the balance is calculated for: the one chosen, or
// the page's own - the first of the month to today. It is chosen apart from the
// hours' period, as the page asks for the two in fields of their own.
func (m screen) balancePeriod() (from, to string) {
	if m.ot.balanceFrom != "" && m.ot.balanceTo != "" {
		return m.ot.balanceFrom, m.ot.balanceTo
	}

	return firstOfMonth(m.today()), m.today().Format(time.DateOnly)
}

// readBalanceNow calculates the balance of the period shown, without the rest of
// the day.
func (m screen) readBalanceNow() tea.Cmd {
	ctx, s, account := m.ctx, m.s, m.shown.id
	from, to := m.balancePeriod()

	return func() tea.Msg {
		balance, err := s.readBalance(ctx, account, from, to)

		return balanceRead{from: from, to: to, balance: balance, err: err}
	}
}

// tookBalance keeps a balance read on its own, where its period is still the
// one shown.
func (m screen) tookBalance(msg balanceRead) screen {
	from, to := m.balancePeriod()

	switch {
	case msg.err != nil:
		m.stale = msg.err.Error()
	case msg.from == from && msg.to == to:
		m.ot.balance = msg.balance
	}

	return m
}

// balanceChoices are the balance's menu: choosing its period, and the balance as
// a document.
func (m screen) balanceChoices() []choice {
	l := m.s.lang

	return []choice{
		{"z", "balancePeriod", l.text("track.period", "Choose the period")},
		{"p", "balancePDF", l.text("action.exportPdf", "Export as PDF")},
	}
}

// chooseBalancePeriod puts the period answered on the balance, from its first
// day, and calculates it.
func (m screen) chooseBalancePeriod(a answers) (tea.Model, tea.Cmd) {
	m.ot.balanceFrom, m.ot.balanceTo, m.ot.balanceTop = strings.TrimSpace(a.balanceFrom), strings.TrimSpace(a.balanceTo), 0

	return m, m.readBalanceNow()
}

// balanceBody is what the balance's menu shows above it: the balance, once it has
// been calculated for the period shown.
func (m screen) balanceBody(look palette, width, rows int) []string {
	if from, to := m.balancePeriod(); m.ot.balance.From != from || m.ot.balance.To != to {
		return []string{look.muted.Render(m.s.lang.text("track.balance.reading", "Calculating the balance …"))}
	}

	return []string{m.balanceCardView(look, width, rows)}
}

// balanceCardView draws the balance as the page's result card draws it: headed
// by its total, a credit or a debt in the colour of one, then whose it is and
// what a day asks of them, which rule counted the days, and every day that
// counts with what was booked, what it asked and the difference - as many as
// the window has room for, from the one at the top, saying which.
func (m screen) balanceCardView(look palette, width, rows int) string {
	l, balance := m.s.lang, m.ot.balance
	wrapped := lipgloss.NewStyle().Width(width)

	// The page's pill is quiet for nothing either way.
	total := look.signedIn(l, balance.TotalBalance)
	if balance.TotalBalance == 0 {
		total = look.muted.Render(total)
	}

	lines := []string{
		look.title.Render(l.text("ot.balance", "Balance")) + " " + total,
		wrapped.Render(look.muted.Render(m.balanceMeta())),
		wrapped.Render(look.muted.Render(m.balanceRule())),
	}

	if len(balance.Days) == 0 {
		return strings.Join(append(lines,
			look.muted.Render(l.text("ot.nothingCounts", "Nothing in this period counts towards the balance."))), "\n")
	}

	table := m.balanceTable(look)
	days := table.lines(width)
	rows = min(rows, len(days))
	top := max(min(m.ot.balanceTop, len(days)-rows), 0)

	lines = append(lines, table.heading(look, width))
	lines = append(lines, days[top:top+rows]...)

	if rows < len(days) {
		lines = append(lines, look.muted.Render(spread(width, "", l.place(top, rows, len(days)))))
	}

	return strings.Join(lines, "\n")
}

// signedIn writes a balance with its sign, in the colour the page's .plus and
// .minus give a credit and a debt.
func (look palette) signedIn(l language, h float64) string {
	switch {
	case h > 0:
		return look.credit.Render(l.signedHours(h))
	case h < 0:
		return look.debt.Render(l.signedHours(h))
	}

	return l.signedHours(h)
}

// balanceMeta is what the page says beside the balance's heading: whose it is,
// what a day asks of them, and what was booked against what was asked.
func (m screen) balanceMeta() string {
	l, balance := m.s.lang, m.ot.balance

	return l.text("ot.meta", "{0} · target {1}/day · booked {2} of {3}", balance.UserName, l.hours(balance.DailyTarget),
		l.hours(balance.TotalBooked), l.hours(balance.TotalTarget))
}

// balanceRule says which rule counted the days, because the same week reads
// differently under each, as the page says it under the heading.
func (m screen) balanceRule() string {
	l, balance := m.s.lang, m.ot.balance

	if balance.EveryDayCountsFrom != "" {
		return l.text("ot.ruleEveryDay", "Every working day counts from {0}.", l.date(balance.EveryDayCountsFrom))
	}

	return l.text("ot.ruleBooked", `Only the days you booked on count. Choose a day under "My account" to count every working day.`)
}

// balanceTable is the page's table of the days that count: each with its date
// and why it may ask less, what was booked, what it asked, and the difference
// in the colour of a credit or a debt.
func (m screen) balanceTable(look palette) grid {
	l := m.s.lang

	table := grid{
		headings: []string{l.text("field.date", "Date"), l.text("ot.booked", "Booked"), l.text("ot.target", "Target"),
			l.text("ot.balance", "Balance")},
		right: []bool{false, true, true, true},
	}

	for _, day := range m.ot.balance.Days {
		table.rows = append(table.rows, []string{m.balanceDate(day), l.hours(day.Booked), l.hours(day.Target),
			look.signedIn(l, day.Balance)})
	}

	return table
}

// balanceDate is a day's first cell, as the page writes it: its date, and the
// public holiday it is or the reason somebody was away, where either is so.
func (m screen) balanceDate(day balanceDay) string {
	l := m.s.lang
	said := []string{l.date(day.Date)}

	if day.Holiday != "" {
		said = append(said, l.holidayOf(day.weekDay))
	}

	if day.Absence != "" {
		said = append(said, l.absenceName(day.Absence))
	}

	return strings.Join(said, " · ")
}

// balanceFileName is the name the page gives the balance as a document: its word
// for the file and today.
func (m screen) balanceFileName() string {
	return m.s.lang.text("ot.filename", "overtime") + "-" + m.today().Format(time.DateOnly) + ".pdf"
}

// exportBalance lays out the balance as the page's overtimeDocument does: the
// table of the days under the period, captioned with whose it is, and the total
// as the summary. Plain, because the colours are the page's to read off its
// screen and a terminal has none to hand on.
func (m screen) exportBalance() tea.Cmd {
	l, balance := m.s.lang, m.ot.balance
	title := l.text("ot.balance", "Balance")

	rows := make([][]string, len(balance.Days))
	for i, day := range balance.Days {
		rows[i] = []string{m.balanceDate(day), l.hours(day.Booked), l.hours(day.Target), l.signedHours(day.Balance)}
	}

	document := map[string]any{
		"title":    l.text("nav.overtime", "Overtime"),
		"subtitle": l.date(balance.From) + " – " + l.date(balance.To),
		"sections": []map[string]any{{
			"heading": title,
			"caption": m.balanceMeta(),
			"table": map[string]any{
				"columns": []string{l.text("field.date", "Date"), l.text("ot.booked", "Booked"),
					l.text("ot.target", "Target"), title},
				"numeric": []bool{false, true, true, true},
				"rows":    rows,
			},
		}},
		"summary": []map[string]string{{"label": title, "value": l.signedHours(balance.TotalBalance)}},
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
