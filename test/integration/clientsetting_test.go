//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// asTheClients sends a request the way the terminal client and the app do: as
// the owner of a personal token and nothing else - no session, no CSRF token.
func asTheClients(t *testing.T, base, token, method, path string, body any) response {
	t.Helper()

	var payload io.Reader

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}

		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(t.Context(), method, base+"/api/v1"+path, payload)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	read, _ := io.ReadAll(resp.Body)

	return response{Status: resp.StatusCode, Body: read, Header: resp.Header}
}

// A personal token changes what its owner changes for themselves under My
// account, through the routes the browser uses and with the bodies the terminal
// client and the app send - so a setting changed there holds in the browser.
// What the clients read beside the day is answered to the token as well.
func TestATokenChangesWhatItsOwnerChangesUnderMyAccount(t *testing.T) {
	t.Parallel()

	a, _, worker := startWithWorker(t)

	var token struct {
		Secret string `json:"secret"`
	}

	worker.must(worker.api(http.MethodPost, "/me/tokens", map[string]any{"name": "phone"}),
		http.StatusCreated, http.StatusOK).Data(t, &token)

	var me struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}

	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	for _, read := range []string{"/me", "/branding", "/me/notices", "/me/holidays"} {
		if answer := asTheClients(t, a.BaseURL(), token.Secret, http.MethodGet, read, nil); answer.Status != http.StatusOK {
			t.Errorf("the clients' read of %s was answered %d: %s", read, answer.Status, answer.Body)
		}
	}

	for _, save := range []struct {
		path string
		body map[string]any
	}{
		{"/me/language", map[string]any{"language": "de"}},
		{"/me/theme", map[string]any{"theme": "dark"}},
		{"/me/timezone", map[string]any{"timezone": "Europe/Vienna"}},
		{"/me/holidays", map[string]any{"place": "DE-BY"}},
		{"/me/reminders", map[string]any{"unbookedDay": false, "timerAlertHours": 5}},
		{"/me/balance-limits", map[string]any{"upperBound": 10, "lowerBound": -5}},
		{fmt.Sprintf("/users/%d/working-times", me.User.ID),
			map[string]any{"dailyTargetHours": 7.5, "maxDailyHours": 0, "everyDayCountsFrom": "2026-11-02"}},
	} {
		if answer := asTheClients(t, a.BaseURL(), token.Secret, http.MethodPut, save.path, save.body); answer.Status != http.StatusOK {
			t.Errorf("the clients' save to %s was answered %d: %s", save.path, answer.Status, answer.Body)
		}
	}

	// And it holds in the browser, read through the browser's own session.
	var after struct {
		User struct {
			Language           string  `json:"language"`
			Theme              string  `json:"theme"`
			Timezone           string  `json:"timezone"`
			DailyTargetHours   float64 `json:"dailyTargetHours"`
			EveryDayCountsFrom string  `json:"everyDayCountsFrom"`
		} `json:"user"`
		Reminders struct {
			UnbookedDay     bool `json:"unbookedDay"`
			TimerAlertHours int  `json:"timerAlertHours"`
		} `json:"reminders"`
	}

	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &after)

	if u := after.User; u.Language != "de" || u.Theme != "dark" || u.Timezone != "Europe/Vienna" ||
		u.DailyTargetHours != 7.5 || u.EveryDayCountsFrom != "2026-11-02" {
		t.Errorf("the browser reads the account as %+v", u)
	}

	if r := after.Reminders; r.UnbookedDay || r.TimerAlertHours != 5 {
		t.Errorf("the browser reads the reminders as %+v", r)
	}

	if mine := holidaysOf(t, worker); mine.Place != "DE-BY" {
		t.Errorf("the browser reads the holidays as %+v", mine)
	}

	var notices struct {
		Balance *struct {
			Upper *float64 `json:"upperBound"`
			Lower *float64 `json:"lowerBound"`
		} `json:"balance"`
	}

	worker.must(worker.api(http.MethodGet, "/me/notices", nil), http.StatusOK).Data(t, &notices)

	if b := notices.Balance; b == nil || b.Upper == nil || *b.Upper != 10 || b.Lower == nil || *b.Lower != -5 {
		t.Errorf("the browser reads the bounds as %+v", notices.Balance)
	}
}
