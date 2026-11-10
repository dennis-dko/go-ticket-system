//go:build integration

package integration

import (
	"net/http"
	"slices"
	"testing"
)

// A browser of RFC 8291's example, subscribed through Google's push service.
var aPushChannel = map[string]any{
	"endpoint": "https://fcm.googleapis.com/fcm/send/integration",
	"keys": map[string]string{
		"p256dh": "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		"auth":   "BTBZMqHH6r4Tts7J_aSIgg",
	},
}

type pushState struct {
	Enabled   bool     `json:"enabled"`
	PublicKey string   `json:"publicKey"`
	Endpoints []string `json:"endpoints"`
}

// Whether the reminders are pushed at all is the administrator's to decide, and
// shipped off: somebody who keeps their own time can neither read the switch nor
// turn it, and no browser is subscribed while it is off.
func TestPushIsTheAdministratorsToSwitchAndShipsOff(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	var switched struct {
		Enabled bool `json:"enabled"`
	}

	admin.must(admin.api(http.MethodGet, "/settings/push", nil), http.StatusOK).Data(t, &switched)

	if switched.Enabled {
		t.Error("a new installation pushes reminders; it is to ship not doing so")
	}

	worker.must(worker.api(http.MethodGet, "/settings/push", nil), http.StatusForbidden)
	worker.must(worker.api(http.MethodPut, "/settings/push", map[string]any{"enabled": true}), http.StatusForbidden)

	var state pushState

	worker.must(worker.api(http.MethodGet, "/me/push", nil), http.StatusOK).Data(t, &state)

	if state.Enabled || state.PublicKey != "" {
		t.Errorf("an installation that does not push offers %+v", state)
	}

	if code := errorCode(t, worker.api(http.MethodPost, "/me/push", aPushChannel)); code != "pushOff" {
		t.Errorf("a browser subscribed while push is off is answered %q, want pushOff", code)
	}
}

// Switched on, somebody's browser is subscribed, among their browsers afterwards,
// and taken back again; an address that is no push service is refused, so the
// installation never posts into a network a person chose.
func TestABrowserIsSubscribedAndTakenBack(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	admin.must(admin.api(http.MethodPut, "/settings/push", map[string]any{"enabled": true}), http.StatusOK)

	var state pushState

	worker.must(worker.api(http.MethodGet, "/me/push", nil), http.StatusOK).Data(t, &state)

	if !state.Enabled || state.PublicKey == "" {
		t.Fatalf("switched on, the installation offers %+v", state)
	}

	elsewhere := map[string]any{"endpoint": "https://127.0.0.1/push", "keys": aPushChannel["keys"]}
	if code := errorCode(t, worker.api(http.MethodPost, "/me/push", elsewhere)); code != "pushServiceUnknown" {
		t.Errorf("an address that is no push service is answered %q, want pushServiceUnknown", code)
	}

	worker.must(worker.api(http.MethodPost, "/me/push", aPushChannel), http.StatusOK, http.StatusCreated).Data(t, &state)

	if !slices.Contains(state.Endpoints, aPushChannel["endpoint"].(string)) {
		t.Fatalf("after subscribing, the account's browsers are %v", state.Endpoints)
	}

	worker.must(worker.api(http.MethodDelete, "/me/push",
		map[string]any{"endpoint": aPushChannel["endpoint"]}), http.StatusOK, http.StatusNoContent)

	worker.must(worker.api(http.MethodGet, "/me/push", nil), http.StatusOK).Data(t, &state)

	if len(state.Endpoints) != 0 {
		t.Errorf("after taking the browser back, the account's browsers are %v", state.Endpoints)
	}
}
