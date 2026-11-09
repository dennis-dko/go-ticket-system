//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
)

// A personal token does with its owner's hours everything the terminal's
// overtime tab asks of the installation, with the requests the client sends: the
// hours of a period with every day of it, a year of days, and the balance of a
// period, each answered for exactly the period asked - which is what the client
// tells its own answer from one still on its way by - and the hours laid out as a
// document. The client's own tests answer it from a fake installation, so
// without this case whether the real one takes those requests from a token, and
// answers them in the shape the client reads, would be assumed.
func TestATokenDoesWithItsOvertimeWhatTheTerminalAsks(t *testing.T) {
	t.Parallel()

	a, _, worker := startWithWorker(t)

	var token struct {
		Secret string `json:"secret"`
	}

	worker.must(worker.api(http.MethodPost, "/me/tokens", map[string]any{"name": "terminal"}),
		http.StatusCreated, http.StatusOK).Data(t, &token)

	ask := func(method, path string, body any, want ...int) response {
		t.Helper()

		answer := asTheClients(t, a.BaseURL(), token.Secret, method, path, body)

		for _, status := range want {
			if answer.Status == status {
				return answer
			}
		}

		t.Fatalf("the client's %s %s was answered %d: %s", method, path, answer.Status, answer.Body)

		return answer
	}

	// A Monday with an activity named and a Tuesday without, in a year gone by so
	// that no figure below depends on the day the case runs.
	ask(http.MethodPost, "/timesheets", map[string]any{"date": "2025-10-20", "durationHours": 9.5, "description": "#review"},
		http.StatusCreated, http.StatusOK)
	ask(http.MethodPost, "/timesheets", map[string]any{"date": "2025-10-21", "durationHours": 2},
		http.StatusCreated, http.StatusOK)

	// My hours, as the card asks for them.
	var stats struct {
		From       string  `json:"from"`
		To         string  `json:"to"`
		TotalHours float64 `json:"totalHours"`
		Days       []struct {
			Date  string  `json:"date"`
			Hours float64 `json:"hours"`
		} `json:"days"`
		Projects []struct {
			ProjectID *uint   `json:"projectId"`
			Hours     float64 `json:"hours"`
		} `json:"projects"`
		Tags []struct {
			Tag   string  `json:"tag"`
			Hours float64 `json:"hours"`
		} `json:"tags"`
		UntaggedHours float64 `json:"untaggedHours"`
	}

	ask(http.MethodGet, "/me/statistics?from=2025-10-20&to=2025-10-26", nil, http.StatusOK).Data(t, &stats)

	if stats.From != "2025-10-20" || stats.To != "2025-10-26" || stats.TotalHours != 11.5 || len(stats.Days) != 7 ||
		stats.Days[0].Hours != 9.5 || stats.Days[6].Hours != 0 {
		t.Errorf("the hours of the week were answered as %+v", stats)
	}

	if len(stats.Projects) != 1 || stats.Projects[0].ProjectID != nil || len(stats.Tags) != 1 ||
		stats.Tags[0].Tag != "review" || stats.Tags[0].Hours != 9.5 || stats.UntaggedHours != 2 {
		t.Errorf("the week's projects and activities were answered as %+v, %+v and %v untagged",
			stats.Projects, stats.Tags, stats.UntaggedHours)
	}

	// The hours as the client lays them out: a section a chart, as figures.
	section := func(heading string, rows [][]string) map[string]any {
		return map[string]any{"heading": heading, "table": map[string]any{
			"columns": []string{heading, "Hours"}, "numeric": []bool{false, true}, "rows": rows,
		}}
	}

	document := ask(http.MethodPost, "/exports/document?lang=en", map[string]any{
		"title": "My hours", "subtitle": "2025-10-20 – 2025-10-26",
		"sections": []map[string]any{
			section("Hours per day", [][]string{{"2025-10-20", "9.50 h"}, {"2025-10-21", "2.00 h"}}),
			section("Hours per project", [][]string{{"No project", "11.50 h"}}),
			section("Hours per activity", [][]string{{"#review", "9.50 h"}, {"No activity", "2.00 h"}}),
		},
		"summary": []map[string]string{{"label": "Total", "value": "11.50 h"}},
	}, http.StatusOK, http.StatusCreated)

	if !bytes.HasPrefix(document.Body, []byte("%PDF")) {
		t.Errorf("the document is not a PDF: %q", document.Body[:min(len(document.Body), 40)])
	}

	// The year's tiles: a whole year of days in one read, the empty ones with it,
	// and the year before to compare with - a leap year here, which a bound on a
	// range of days has to hold too.
	type yearDays struct {
		Days []struct {
			Date   string  `json:"date"`
			Booked float64 `json:"booked"`
		} `json:"days"`
	}

	var year, before yearDays

	ask(http.MethodGet, "/me/days?from=2025-01-01&to=2025-12-31", nil, http.StatusOK).Data(t, &year)
	ask(http.MethodGet, "/me/days?from=2024-01-01&to=2024-12-31", nil, http.StatusOK).Data(t, &before)

	booked := map[string]float64{}
	for _, day := range year.Days {
		booked[day.Date] = day.Booked
	}

	if len(year.Days) != 365 || len(before.Days) != 366 || booked["2025-10-20"] != 9.5 || booked["2025-10-21"] != 2 {
		t.Errorf("the year came to %d days and the one before to %d, with %v and %v booked on the two",
			len(year.Days), len(before.Days), booked["2025-10-20"], booked["2025-10-21"])
	}

	// The balance, of the account the token belongs to, which the client knows by
	// the id /me names.
	var me struct {
		User struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"user"`
	}

	ask(http.MethodGet, "/me", nil, http.StatusOK).Data(t, &me)

	var balance struct {
		UserName    string  `json:"userName"`
		From        string  `json:"from"`
		To          string  `json:"to"`
		TotalBooked float64 `json:"totalBooked"`
		Days        []struct {
			Date    string  `json:"date"`
			Booked  float64 `json:"booked"`
			Target  float64 `json:"target"`
			Balance float64 `json:"balance"`
		} `json:"days"`
	}

	ask(http.MethodGet, fmt.Sprintf("/users/%d/overtime?from=2025-10-20&to=2025-10-26", me.User.ID), nil,
		http.StatusOK).Data(t, &balance)

	if balance.From != "2025-10-20" || balance.To != "2025-10-26" || balance.UserName != me.User.Name ||
		balance.TotalBooked != 11.5 || len(balance.Days) == 0 || balance.Days[0].Date != "2025-10-20" ||
		balance.Days[0].Balance != balance.Days[0].Booked-balance.Days[0].Target {
		t.Errorf("the week's balance was answered as %+v", balance)
	}
}
