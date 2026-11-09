//go:build integration

package integration

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

// A personal token does with its owner's calendar everything the terminal's
// calendar tab asks of the installation, with the requests the client sends: a
// day's working time recorded, read, laid out as a document and taken back; days
// of leave given a year, read and taken back; and days away recorded over a
// range, read and one of them taken back. The client's own tests answer it from a
// fake installation, so without this case whether the real one takes those
// requests from a token would be assumed.
func TestATokenDoesWithItsCalendarWhatTheTerminalAsks(t *testing.T) {
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

	// The working time: a day recorded as the form sends it, and read with its
	// month.
	ask(http.MethodPut, "/me/workdays/2026-10-19", map[string]any{"start": "08:00", "end": "16:30", "breakMinutes": 30},
		http.StatusOK, http.StatusCreated)

	var month struct {
		Days []struct {
			Date        string  `json:"date"`
			Recorded    bool    `json:"recorded"`
			WorkedHours float64 `json:"workedHours"`
		} `json:"days"`
	}

	ask(http.MethodGet, "/me/workdays?from=2026-10-01&to=2026-10-31", nil, http.StatusOK).Data(t, &month)

	if len(month.Days) != 1 || month.Days[0].Date != "2026-10-19" || !month.Days[0].Recorded || month.Days[0].WorkedHours != 8 {
		t.Errorf("the recorded day was read back as %+v", month.Days)
	}

	// The card as the client lays it out, which the installation answers with a
	// PDF rather than with data.
	document := ask(http.MethodPost, "/exports/document?lang=en", map[string]any{
		"title": "Working time", "subtitle": "October 2026",
		"sections": []map[string]any{{"heading": "Working time", "caption": "A record.", "table": map[string]any{
			"columns": []string{"Date", "Start"}, "numeric": []bool{false, false}, "rows": [][]string{{"2026-10-19", "08:00"}},
		}}},
		"summary": []map[string]string{{"label": "Worked", "value": "8.00 h"}},
	}, http.StatusOK, http.StatusCreated)

	if !bytes.HasPrefix(document.Body, []byte("%PDF")) {
		t.Errorf("the document is not a PDF: %q", document.Body[:min(len(document.Body), 40)])
	}

	ask(http.MethodDelete, "/me/workdays/2026-10-19", nil, http.StatusOK, http.StatusNoContent)

	// The leave: days given a year from a year on, read for the year, taken back.
	ask(http.MethodPut, "/me/vacation", map[string]any{"fromYear": 2026, "days": 27.5}, http.StatusOK, http.StatusCreated)

	var leave struct {
		Entitlements []struct {
			FromYear int     `json:"fromYear"`
			Days     float64 `json:"days"`
		} `json:"entitlements"`
	}

	ask(http.MethodGet, "/me/vacation?year=2026", nil, http.StatusOK).Data(t, &leave)

	if len(leave.Entitlements) != 1 || leave.Entitlements[0].FromYear != 2026 || leave.Entitlements[0].Days != 27.5 {
		t.Errorf("the days given were read back as %+v", leave.Entitlements)
	}

	ask(http.MethodDelete, "/me/vacation/2026", nil, http.StatusOK, http.StatusNoContent)

	// The days away: a working week recorded, the year's read, one taken back. In
	// this year, because the list the client asks for is the year today is in -
	// the second week of January, which no year starts or ends inside.
	monday := time.Date(time.Now().Year(), time.January, 8, 0, 0, 0, 0, time.UTC)
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}

	from, to := monday.Format(time.DateOnly), monday.AddDate(0, 0, 6).Format(time.DateOnly)

	var recorded struct {
		TotalCount int `json:"totalCount"`
	}

	ask(http.MethodPost, "/me/absences", map[string]string{"from": from, "to": to, "kind": "vacation"},
		http.StatusCreated, http.StatusOK).Data(t, &recorded)

	if recorded.TotalCount != 5 {
		t.Errorf("a week of leave recorded %d days, want the five its week asks something of", recorded.TotalCount)
	}

	var away struct {
		Items []struct {
			Day string `json:"day"`
		} `json:"items"`
	}

	ask(http.MethodGet, "/me/absences", nil, http.StatusOK).Data(t, &away)

	if len(away.Items) != 5 {
		t.Errorf("the year's days away were read as %+v", away.Items)
	}

	ask(http.MethodDelete, "/me/absences/"+from, nil, http.StatusOK, http.StatusNoContent)
}
