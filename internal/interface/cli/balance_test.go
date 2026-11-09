package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// balanceOf answers /users/7/overtime for the period its query asks. Its first
// day has eight hours booked of eight asked, its second second hours of eight,
// and its third is a public holiday asking none; where whole is set every
// further day of the period follows, empty. Every working day counts from
// everyDay where it is not empty, and only the days booked on where it is.
func balanceOf(everyDay string, second float64, whole bool) func(url.Values) string {
	return func(q url.Values) string {
		from, _ := time.Parse(time.DateOnly, q.Get("from"))
		to, _ := time.Parse(time.DateOnly, q.Get("to"))

		var (
			days           []map[string]any
			booked, target float64
		)

		for d, i := from, 0; i < 3 || (whole && !d.After(to)); d, i = d.AddDate(0, 0, 1), i+1 {
			day := map[string]any{"date": d.Format(time.DateOnly), "booked": 0.0, "target": 0.0, "balance": 0.0}

			switch i {
			case 0:
				day["booked"], day["target"] = 8.0, 8.0
			case 1:
				day["booked"], day["target"], day["balance"] = second, 8.0, second-8
			case 2:
				day["holiday"] = "germanUnity"
			}

			booked += day["booked"].(float64)
			target += day["target"].(float64)
			days = append(days, day)
		}

		answer := map[string]any{
			"userId": 7, "userName": "Dana Example", "from": q.Get("from"), "to": q.Get("to"), "dailyTarget": 8,
			"days": days, "everyDayCountsFrom": nil, "totalBooked": booked, "totalTarget": target,
			"totalBalance": booked - target,
		}

		if everyDay != "" {
			answer["everyDayCountsFrom"] = everyDay
		}

		raw, _ := json.Marshal(answer)

		return string(raw)
	}
}

// onBalance is a screen showing the balance of the period from first to last.
func onBalance(first, last string) func(screen) bool {
	return func(m screen) bool {
		return settled(m) && m.sub == subBalance && m.ot.balance.From == first && m.ot.balance.To == last
	}
}

// balanceOpen moves to the overtime's tab, opens the balance with s and waits
// for the balance of the month so far.
func (w *watched) balanceOpen(first time.Time) string {
	w.t.Helper()

	w.overtimeOpen(first)
	w.press("s")

	return w.until("the balance is calculated", func(m screen) bool {
		return settled(m) && m.sub == subBalance && m.ot.balance.From == first.Format(time.DateOnly)
	})
}

// balancePeriod chooses the balance's period from first to last as somebody
// types it: z, then each day over the one the field offers.
func (w *watched) balancePeriod(first, last string) {
	w.t.Helper()

	w.press("z")
	w.until("the first day is asked", asking("balanceFrom"))

	for range 10 {
		w.press("backspace")
	}

	w.press(first, "enter")
	w.until("the last day is asked", asking("balanceTo"))

	for range 10 {
		w.press("backspace")
	}

	w.press(last, "enter")
}

// The balance is calculated as soon as it is opened, for the period the page's
// fields start with - from the first of the month to today - and drawn as the
// page draws it: its total as a credit in the colour of one, whose it is and what
// a day asks of them, which rule counted the days, and every day that counts with
// what was booked, what it asked and the difference, a public holiday named.
func TestTheBalanceIsCalculatedForTheMonthSoFar(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	day := func(n int) string { return first.AddDate(0, 0, n).Format(time.DateOnly) }

	w := watch(t, server.URL, f.token)
	shows := w.balanceOpen(first)

	sent := f.sentTo("GET /users/7/overtime")
	q, _ := sent[len(sent)-1]["?"].(url.Values)

	if q.Get("from") != day(0) || q.Get("to") != time.Now().In(mustZone(t, "Europe/Berlin")).Format(time.DateOnly) {
		t.Errorf("the balance was asked for %v, want from the first of the month to today", q)
	}

	said := prose(shows)

	for _, want := range []string{
		"Overtime › Balance",
		"Balance +1.50 h",
		"Dana Example · target 8.00 h/day · booked 17.50 h of 16.00 h",
		`Only the days you booked on count. Choose a day under "My account" to count every working day.`,
		"Date Booked Target Balance " + day(0) + " 8.00 h 8.00 h 0.00 h " + day(1) + " 9.50 h 8.00 h +1.50 h " +
			day(2) + " · German Unity Day 0.00 h 0.00 h 0.00 h",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the balance does not say %q:\n%s", want, shows)
		}
	}

	var (
		drawn string
		dark  bool
	)

	w.until("the screen is drawn", func(m screen) bool { drawn, dark = m.View().Content, m.darkness(); return true })

	if !strings.Contains(drawn, paletteFor(dark).credit.Render("+1.50 h")) {
		t.Errorf("a credit is not drawn in the colour of one:\n%s", shows)
	}

	w.close()
}

// From the day an account chose, every working day counts, and the balance says
// so; a debt is drawn in the colour of one.
func TestTheBalanceSaysFromWhichDayEveryWorkingDayCounts(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	f.balance = balanceOf("2026-10-01", 6, false)

	w := watch(t, server.URL, f.token)
	shows := w.balanceOpen(first)

	if said := prose(shows); !strings.Contains(said, "Every working day counts from 2026-10-01.") ||
		!strings.Contains(said, "Balance -2.00 h") {
		t.Errorf("the balance does not say its rule and its debt:\n%s", shows)
	}

	var (
		drawn string
		dark  bool
	)

	w.until("the screen is drawn", func(m screen) bool { drawn, dark = m.View().Content, m.darkness(); return true })

	if !strings.Contains(drawn, paletteFor(dark).debt.Render("-2.00 h")) {
		t.Errorf("a debt is not drawn in the colour of one:\n%s", shows)
	}

	w.close()
}

// The balance's period is chosen as the page's two fields choose it, apart from
// the hours' period above it, and the balance is calculated again for it.
func TestTheBalancesPeriodIsChosen(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.balanceOpen(first)

	w.balancePeriod("2026-10-01", "2026-10-31")
	shows := w.until("October's balance is calculated", onBalance("2026-10-01", "2026-10-31"))

	if !strings.Contains(prose(shows), "2026-10-01 8.00 h 8.00 h 0.00 h") {
		t.Errorf("October's days are not drawn:\n%s", shows)
	}

	w.until("the hours keep their own period", func(m screen) bool {
		from, _ := m.statsPeriod()
		return from == first.Format(time.DateOnly)
	})

	w.close()
}

// A period with more days than the window has room for shows as many as fit and
// says which, and PgUp and PgDn page through the rest.
func TestALongBalanceIsPaged(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	f.balance = balanceOf("", 9.5, true)

	w := watch(t, server.URL, f.token)
	w.balanceOpen(first)

	w.balancePeriod("2026-01-01", "2026-03-31")
	shows := w.until("the quarter's balance is calculated", onBalance("2026-01-01", "2026-03-31"))

	if !strings.Contains(shows, "of 90 · pgup/pgdn") || lineWith(shows, "2026-01-01", "8.00 h") == "" {
		t.Errorf("a quarter's days are not paged:\n%s", shows)
	}

	w.press("pgdown")
	shows = w.until("the days move on", func(m screen) bool { return settled(m) && m.ot.balanceTop > 0 })

	if lineWith(shows, "2026-01-01", "8.00 h") != "" {
		t.Errorf("the first day is still drawn after a page down:\n%s", shows)
	}

	w.close()
}

// A period in which nothing counts says so, as the page's empty table does.
func TestABalanceWithNothingCountingSaysSo(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	f.balance = func(q url.Values) string {
		return `{"userName":"Dana Example","from":"` + q.Get("from") + `","to":"` + q.Get("to") + `","dailyTarget":8,"days":[]}`
	}

	w := watch(t, server.URL, f.token)
	shows := w.balanceOpen(first)

	if !strings.Contains(prose(shows), "Nothing in this period counts towards the balance.") {
		t.Errorf("an empty balance does not say so:\n%s", shows)
	}

	w.close()
}

// The balance is laid out as the page's overtimeDocument lays it out: the table
// under the period, captioned with whose it is, and its total as the summary.
func TestTheBalanceIsExportedAsADocument(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)
	path := filepath.Join(t.TempDir(), "balance.pdf")

	w := watch(t, server.URL, f.token)
	w.balanceOpen(first)

	w.press("p")
	shows := w.until("where it goes is asked", asking("file"))

	if !strings.Contains(shows, "overtime-") {
		t.Errorf("the file is not named as the page names it:\n%s", shows)
	}

	for range 60 {
		w.press("backspace")
	}

	w.press(path, "enter")
	w.until("the document is saved", func(m screen) bool { return settled(m) && strings.Contains(m.news, path) })

	if written, err := os.ReadFile(path); err != nil || string(written) != fakeDocument {
		t.Fatalf("the document was not saved as it came: %q, %v", written, err)
	}

	sent := sentOnce(t, f, "POST /exports/document")
	sections, _ := sent["sections"].([]any)
	summary, _ := sent["summary"].([]any)

	if sent["title"] != "Overtime" || len(sections) != 1 || len(summary) != 1 {
		t.Fatalf("the document is %v with %d sections and %d summaries, want Overtime with one of each",
			sent["title"], len(sections), len(summary))
	}

	section, _ := sections[0].(map[string]any)
	table, _ := section["table"].(map[string]any)
	rows, _ := table["rows"].([]any)
	total, _ := summary[0].(map[string]any)

	if section["heading"] != "Balance" || section["caption"] != "Dana Example · target 8.00 h/day · booked 17.50 h of 16.00 h" ||
		strings.Join(anyStrings(table["columns"]), ",") != "Date,Booked,Target,Balance" || len(rows) != 3 ||
		strings.Join(anyStrings(rows[1]), ",") != first.AddDate(0, 0, 1).Format(time.DateOnly)+",9.50 h,8.00 h,+1.50 h" {
		t.Errorf("the balance's section is %v", section)
	}

	if total["label"] != "Balance" || total["value"] != "+1.50 h" {
		t.Errorf("the summary is %v, want the balance's total", total)
	}

	w.close()
}

// A balance read for a period chosen before, landing after another was chosen,
// is not taken over the balance of the one chosen since. It is handed to the
// screen after the newer one rather than raced for, so the order tested is the
// same every time.
func TestABalanceForAPeriodChosenBeforeIsNotTaken(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.balanceOpen(first)

	w.balancePeriod("2026-10-01", "2026-10-31")
	w.until("October's balance is calculated", onBalance("2026-10-01", "2026-10-31"))

	var late overtimeBalance
	if err := json.Unmarshal([]byte(balanceOf("", 9.5, false)(url.Values{"from": {"2026-09-01"}, "to": {"2026-09-30"}})), &late); err != nil {
		t.Fatal(err)
	}

	w.p.Send(balanceRead{from: late.From, to: late.To, balance: late})
	shows := w.until("the late answer is handed over", settled)

	if !strings.Contains(prose(shows), "2026-10-01 8.00 h 8.00 h 0.00 h") {
		t.Errorf("a balance for the period before was taken over the one chosen since:\n%s", shows)
	}

	w.close()
}

// A read of the day that set out before the balance's period was chosen brings
// the balance of the period before, and it is not taken over the one chosen
// since. The installation holds that read until the period has been chosen and
// calculated, so the order tested is the same every time.
func TestADayReadFromBeforeTheBalancesPeriodWasChosenLeavesIt(t *testing.T) {
	f, server := newFakeInstallation(t)
	first := monthHere(t)

	w := watch(t, server.URL, f.token)
	w.balanceOpen(first)

	held, release := make(chan struct{}, 1), make(chan struct{})

	var once sync.Once

	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)

	f.mu.Lock()
	f.balance = func(q url.Values) string {
		if q.Get("from") == first.Format(time.DateOnly) {
			select {
			case held <- struct{}{}:
			default:
			}

			<-release
		}

		return balanceOf("", 9.5, false)(q)
	}
	f.mu.Unlock()

	w.press("r")

	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the day was not read again with its balance")
	}

	w.balancePeriod("2026-10-01", "2026-10-31")
	w.until("October's balance is calculated while the day is read", func(m screen) bool {
		return m.form == nil && m.ot.balance.From == "2026-10-01"
	})

	free()
	shows := w.until("the day is read", settled)

	if !strings.Contains(prose(shows), "2026-10-01 8.00 h 8.00 h 0.00 h") {
		t.Errorf("the balance of the period before was taken over the one chosen since:\n%s", shows)
	}

	w.close()
}
