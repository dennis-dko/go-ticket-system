//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

// uploadAsTheClient sends a file the way the terminal client imports one: a
// form with the file and its fields, as the owner of a personal token alone.
func uploadAsTheClient(t *testing.T, base, token, path string, content []byte, fields map[string]string) response {
	t.Helper()

	var form bytes.Buffer

	writer := multipart.NewWriter(&form)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}

	part, err := writer.CreateFormFile("file", "entries.xlsx")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/api/v1"+path, &form)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	read, _ := io.ReadAll(resp.Body)

	return response{Status: resp.StatusCode, Body: read, Header: resp.Header}
}

// A personal token does with its owner's entries everything the terminal's time
// entries tab asks of the installation, with the requests the client sends: the
// list narrowed, a correction, a history, a transfer and a deletion; the
// stopwatch's start corrected, stopped at an end, and discarded; templates kept
// and deleted; and the spreadsheet exported and checked for an import. The
// client's own tests answer it from a fake installation, so without this case
// whether the real one takes those requests from a token would be assumed.
func TestATokenDoesWithItsEntriesWhatTheTerminalAsks(t *testing.T) {
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

	var booked struct {
		ID uint `json:"id"`
	}

	ask(http.MethodPost, "/timesheets", map[string]any{"date": "2026-10-21", "durationHours": 0.5},
		http.StatusCreated, http.StatusOK).Data(t, &booked)
	ask(http.MethodPost, "/timesheets", map[string]any{"date": "2026-10-22", "durationHours": 2, "projectId": project.ID,
		"description": "review"}, http.StatusCreated, http.StatusOK)

	// The list as the card asks for it, narrowed as its filter narrows it.
	var listed struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
		TotalCount int `json:"totalCount"`
	}

	ask(http.MethodGet, "/timesheets?from=2026-10-01&to=2026-10-31&limit=25&offset=0&projectId=none", nil,
		http.StatusOK).Data(t, &listed)

	if listed.TotalCount != 1 || len(listed.Items) != 1 || listed.Items[0].ID != booked.ID {
		t.Errorf("the entries without a project were listed as %+v", listed)
	}

	// A correction with every field, as the client sends one, and its history.
	ask(http.MethodPut, fmt.Sprintf("/timesheets/%d", booked.ID), map[string]any{
		"projectId": 0, "date": "2026-10-21", "durationHours": 0.75, "description": "",
	}, http.StatusOK)

	var history struct {
		Items []any `json:"items"`
	}

	ask(http.MethodGet, fmt.Sprintf("/timesheets/%d/revisions", booked.ID), nil, http.StatusOK).Data(t, &history)

	if len(history.Items) != 1 {
		t.Errorf("the corrected entry has %d earlier versions, want one", len(history.Items))
	}

	ask(http.MethodPost, fmt.Sprintf("/timesheets/%d/transfer", booked.ID), map[string]uint{"projectId": project.ID},
		http.StatusCreated, http.StatusOK)

	// The stopwatch: started, its start corrected, stopped at an end on the
	// owner's wall clock; started again and discarded.
	var me struct {
		User struct {
			EffectiveTimezone string `json:"effectiveTimezone"`
		} `json:"user"`
	}

	ask(http.MethodGet, "/me", nil, http.StatusOK).Data(t, &me)

	zone, err := time.LoadLocation(me.User.EffectiveTimezone)
	if err != nil {
		t.Fatalf("the account's zone %q: %v", me.User.EffectiveTimezone, err)
	}

	started := time.Now().In(zone).Add(-2 * time.Hour)

	ask(http.MethodPost, "/me/timer", map[string]any{}, http.StatusCreated, http.StatusOK)
	ask(http.MethodPut, "/me/timer", map[string]string{"startedAt": started.Format("2006-01-02T15:04")}, http.StatusOK)

	var stopped struct {
		DurationHours float64 `json:"durationHours"`
	}

	ask(http.MethodPost, "/me/timer/stop", map[string]string{"endedAt": started.Add(time.Hour).Format("2006-01-02T15:04")},
		http.StatusCreated, http.StatusOK).Data(t, &stopped)

	if stopped.DurationHours != 1 {
		t.Errorf("the stopwatch stopped at an end booked %v hours, want one", stopped.DurationHours)
	}

	ask(http.MethodPost, "/me/timer", map[string]any{}, http.StatusCreated, http.StatusOK)
	ask(http.MethodDelete, "/me/timer", nil, http.StatusOK, http.StatusNoContent)

	// A template kept with what was left out sent as nothing, and deleted.
	var kept struct {
		ID uint `json:"id"`
	}

	ask(http.MethodPost, "/me/templates", map[string]any{"projectId": nil, "durationHours": 1.5, "description": nil},
		http.StatusCreated, http.StatusOK).Data(t, &kept)
	ask(http.MethodGet, "/me/templates", nil, http.StatusOK)
	ask(http.MethodDelete, fmt.Sprintf("/me/templates/%d", kept.ID), nil, http.StatusOK, http.StatusNoContent)

	// The spreadsheet, exported as the card narrows it and checked for an import.
	book := ask(http.MethodGet, "/timesheets/export?from=2026-10-01&to=2026-10-31&lang=en", nil, http.StatusOK)
	if !bytes.HasPrefix(book.Body, []byte("PK")) {
		t.Fatalf("the export is not a workbook: %q", book.Body[:min(len(book.Body), 40)])
	}

	checked := uploadAsTheClient(t, a.BaseURL(), token.Secret, "/timesheets/import?lang=en", book.Body,
		map[string]string{"dryRun": "true"})
	if checked.Status >= http.StatusMultipleChoices || !strings.Contains(string(checked.Body), `"dryRun":true`) {
		t.Errorf("the client's check of a file was answered %d: %s", checked.Status, checked.Body)
	}

	ask(http.MethodDelete, fmt.Sprintf("/timesheets/%d", booked.ID), nil, http.StatusOK, http.StatusNoContent)
}
