package cli

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// branding is what the installation calls itself and what it says to everybody
// who opens it, in the client's language: the title the page heads itself with
// and the banner it shows above everything.
type branding struct {
	title, banner string
}

// branding reads the installation's name and banner: what it has written for
// the client's language, or what it has.
func (s *session) branding(ctx context.Context) (branding, error) {
	type texts struct {
		Title  string `json:"title"`
		Banner string `json:"banner"`
	}

	var answer struct {
		texts

		Translations map[string]texts `json:"translations"`
	}

	if err := s.api.call(ctx, http.MethodGet, "/branding", nil, &answer); err != nil {
		return branding{}, err
	}

	own := answer.Translations[s.lang.code()]
	pick := func(translated, written string) string {
		if t := strings.TrimSpace(translated); t != "" {
			return t
		}

		return strings.TrimSpace(written)
	}

	return branding{title: pick(own.Title, answer.Title), banner: pick(own.Banner, answer.Banner)}, nil
}

// notices are what about the owner's own time deserves a banner, as the
// installation works it out for the browser: the last working day with nothing
// booked on it, and a running balance past a bound they set. Each is left out
// where it does not hold, or where its owner asked not to be told.
type notices struct {
	unbookedDay string
	balance     *balance
}

// balance is the owner's running balance and the bounds they keep it within,
// nil for none.
type balance struct {
	hours        float64
	upper, lower *float64
	outside      string
}

// notices reads what the installation says deserves a banner.
func (s *session) notices(ctx context.Context) (notices, error) {
	var answer struct {
		UnbookedDay *string `json:"unbookedDay"`
		Balance     *struct {
			Hours   float64  `json:"hours"`
			Upper   *float64 `json:"upperBound"`
			Lower   *float64 `json:"lowerBound"`
			Outside string   `json:"outside"`
		} `json:"balance"`
	}

	if err := s.api.call(ctx, http.MethodGet, "/me/notices", nil, &answer); err != nil {
		return notices{}, err
	}

	var said notices

	if answer.UnbookedDay != nil {
		said.unbookedDay = *answer.UnbookedDay
	}

	if b := answer.Balance; b != nil {
		said.balance = &balance{hours: b.Hours, upper: b.Upper, lower: b.Lower, outside: b.Outside}
	}

	return said, nil
}

// noticeLines are the notices as the screen says them, each a sentence of its
// own: the stopwatch first, where it has run longer than its owner wanted to be
// told after, which the browser works out the same way from the same choice.
func (m screen) noticeLines() []string {
	l := m.s.lang

	var lines []string

	if clock := m.shown.clock; clock.Running && clock.StartedAt != nil && m.shown.alertHours > 0 &&
		m.elapsed() >= time.Duration(m.shown.alertHours)*time.Hour {
		lines = append(lines, l.text("track.notice.longRunning",
			"The stopwatch has been running since {0}, longer than the {1} you chose to be told after.",
			m.moment(*clock.StartedAt), strconv.Itoa(m.shown.alertHours)+" "+l.text("unit.hours", "h")))
	}

	if day := m.told.unbookedDay; day != "" {
		lines = append(lines, l.text("track.notice.unbooked",
			"Nothing is booked on {0}, a working day by your plan.", l.date(day)))
	}

	if b := m.told.balance; b != nil {
		switch {
		case b.outside == model.BalanceAbove && b.upper != nil:
			lines = append(lines, l.text("track.notice.above",
				"Your running balance is {0}, above the upper bound of {1} you set.",
				l.signedHours(b.hours), l.signedHours(*b.upper)))
		case b.outside == model.BalanceBelow && b.lower != nil:
			lines = append(lines, l.text("track.notice.below",
				"Your running balance is {0}, below the lower bound of {1} you set.",
				l.signedHours(b.hours), l.signedHours(*b.lower)))
		}
	}

	return lines
}

// moment writes when something happened the way the stopwatch line does: the
// time alone for today, and the day before it for an earlier one - a stopwatch
// somebody forgot is exactly the one that started on another day.
func (m screen) moment(at time.Time) string {
	at = at.In(m.shown.zone)

	if at.Format(time.DateOnly) == m.now.In(m.shown.zone).Format(time.DateOnly) {
		return at.Format("15:04")
	}

	return m.s.lang.date(at.Format(time.DateOnly)) + " " + at.Format("15:04")
}

// signedHours writes a balance the way the browser does: a plus before a credit,
// and the minus a debt carries.
func (l language) signedHours(h float64) string {
	if h > 0 {
		return "+" + l.hours(h)
	}

	return l.hours(h)
}
