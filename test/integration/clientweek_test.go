//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
)

// A personal token does with its owner's week everything the terminal's week tab
// asks of the installation, with the requests the client sends: the week's
// entries and its days read from Monday to Sunday, last week copied into it, a
// cell's entry corrected by its hours alone and taken back. The client's own
// tests answer it from a fake installation, so without this case whether the
// real one takes those requests from a token would be assumed.
func TestATokenDoesWithItsWeekWhatTheTerminalAsks(t *testing.T) {
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

	var project struct {
		ID uint `json:"id"`
	}

	worker.must(worker.api(http.MethodPost, "/projects", map[string]any{"name": "Website"}),
		http.StatusCreated, http.StatusOK).Data(t, &project)

	// Two hours on the Monday of the week before, copied into the week after it.
	ask(http.MethodPost, "/timesheets", map[string]any{"date": "2026-10-12", "durationHours": 2, "projectId": project.ID},
		http.StatusCreated, http.StatusOK)

	var copied struct {
		Copied int     `json:"copied"`
		Hours  float64 `json:"hours"`
	}

	ask(http.MethodPost, "/me/weeks/copy", map[string]string{"from": "2026-10-12", "to": "2026-10-19"},
		http.StatusCreated, http.StatusOK).Data(t, &copied)

	if copied.Copied != 1 || copied.Hours != 2 {
		t.Errorf("last week was copied as %+v, want its one entry of two hours", copied)
	}

	// The week as the grid reads it: its entries and its days.
	var listed struct {
		Items []struct {
			ID            uint    `json:"id"`
			Date          string  `json:"date"`
			DurationHours float64 `json:"durationHours"`
		} `json:"items"`
		TotalCount int `json:"totalCount"`
	}

	ask(http.MethodGet, "/timesheets?from=2026-10-19&to=2026-10-25&limit=500&offset=0", nil,
		http.StatusOK).Data(t, &listed)

	if listed.TotalCount != 1 || len(listed.Items) != 1 || listed.Items[0].Date != "2026-10-19" {
		t.Fatalf("the week the copy went into lists %+v", listed)
	}

	var week struct {
		Days []struct {
			Date   string  `json:"date"`
			Booked float64 `json:"booked"`
		} `json:"days"`
	}

	ask(http.MethodGet, "/me/days?from=2026-10-19&to=2026-10-25", nil, http.StatusOK).Data(t, &week)

	if len(week.Days) != 7 || week.Days[0].Date != "2026-10-19" || week.Days[0].Booked != 2 {
		t.Errorf("the week's days were read as %+v", week.Days)
	}

	// A cell's number changed: the entry behind it corrected by its hours alone,
	// which leaves its day and its project as they were.
	id := listed.Items[0].ID

	ask(http.MethodPut, fmt.Sprintf("/timesheets/%d", id), map[string]any{"durationHours": 3}, http.StatusOK)
	ask(http.MethodGet, "/timesheets?from=2026-10-19&to=2026-10-25&limit=500&offset=0&projectId="+
		fmt.Sprint(project.ID), nil, http.StatusOK).Data(t, &listed)

	if len(listed.Items) != 1 || listed.Items[0].DurationHours != 3 || listed.Items[0].Date != "2026-10-19" {
		t.Errorf("the corrected cell's entry reads %+v, want three hours on its day and project", listed.Items)
	}

	// And emptied: taken back.
	ask(http.MethodDelete, fmt.Sprintf("/timesheets/%d", id), nil, http.StatusOK, http.StatusNoContent)
}
