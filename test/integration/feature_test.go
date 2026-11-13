//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// --------------------------------------------------------------- timezones

func TestTimezoneIsInstanceWideWithAPersonalOverride(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	admin.must(admin.api(http.MethodPut, "/settings/timezone",
		map[string]string{"timezone": "Europe/Berlin"}), http.StatusOK)

	var me struct {
		User userResponse `json:"user"`
	}

	admin.must(admin.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if me.User.Timezone != "" {
		t.Errorf("no personal zone should be set, got %q", me.User.Timezone)
	}

	if me.User.EffectiveTimezone != "Europe/Berlin" {
		t.Errorf("the instance zone should apply, got %q", me.User.EffectiveTimezone)
	}

	// Someone working from abroad overrides it for themselves.
	admin.must(admin.api(http.MethodPut, "/me/timezone",
		map[string]string{"timezone": "Pacific/Auckland"}), http.StatusOK)

	admin.must(admin.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if me.User.EffectiveTimezone != "Pacific/Auckland" {
		t.Errorf("the personal zone should win, got %q", me.User.EffectiveTimezone)
	}

	// Clearing it follows the instance again - the normal way back.
	admin.must(admin.api(http.MethodPut, "/me/timezone",
		map[string]string{"timezone": ""}), http.StatusOK)

	admin.must(admin.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if me.User.EffectiveTimezone != "Europe/Berlin" {
		t.Errorf("clearing it should follow the instance, got %q", me.User.EffectiveTimezone)
	}
}

// A name that reads like a zone but is not one would be stored and then fall
// back to UTC at every use, moving every moment shown and the day of every
// ticket's number with nothing on screen to show it.
func TestAnUnknownTimezoneIsRefused(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	for _, name := range []string{"Europe/Munich", "GMT+2", "Nowhere"} {
		if r := admin.api(http.MethodPut, "/settings/timezone",
			map[string]string{"timezone": name}); r.Status != http.StatusBadRequest {
			t.Errorf("%q should be refused, got %d", name, r.Status)
		}
	}

	// The tz database has to be compiled into the binary, or every zone would
	// resolve to UTC on a host without zoneinfo files - a scratch container.
	admin.must(admin.api(http.MethodPut, "/settings/timezone",
		map[string]string{"timezone": "Pacific/Chatham"}), http.StatusOK)
}

// ------------------------------------------------------- operational limits

func TestOperationalLimitsApplyWithoutARestart(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	var before struct {
		Configured map[string]any `json:"configured"`
		Effective  struct {
			RateLimit int `json:"rateLimit"`
		} `json:"effective"`
		Defaults struct {
			RateLimit int `json:"rateLimit"`
		} `json:"defaults"`
	}

	admin.must(admin.api(http.MethodGet, "/settings/operational", nil), http.StatusOK).Data(t, &before)

	if len(before.Configured) != 0 {
		t.Errorf("nothing should be overridden on a fresh instance, got %v", before.Configured)
	}

	if before.Effective.RateLimit != before.Defaults.RateLimit {
		t.Error("with nothing overridden, the effective value is the environment's")
	}

	// The smallest budget there is. The limiter guards what a script or a guesser
	// sends - a sign-in, a request carrying a token - and leaves a signed-in
	// session alone, so a token nobody issued is what runs into it here, and the
	// administrator's own session is never held up by it.
	admin.must(admin.api(http.MethodPut, "/settings/operational",
		map[string]any{"rateLimit": 5, "rateLimitWindowSeconds": 60}), http.StatusOK)

	script := a.newClient().withHeader("Authorization", "Bearer gts_issued-to-nobody")

	// In force without a restart: a burst past five is refused within seconds.
	limited := eventually(func() bool {
		for range 8 {
			if script.api(http.MethodGet, "/me", nil).Status == http.StatusTooManyRequests {
				return true
			}
		}

		return false
	})

	if !limited {
		t.Fatal("a budget of five requests was not applied without a restart")
	}

	// Resetting returns to the environment's value, which lets the same caller
	// through again within the same window.
	admin.must(admin.api(http.MethodPut, "/settings/operational", map[string]any{}), http.StatusOK)

	if !eventually(func() bool {
		return script.api(http.MethodGet, "/me", nil).Status == http.StatusUnauthorized
	}) {
		t.Error("the environment's budget did not come back after the reset")
	}
}

// This is the one screen that can lock out its own administrator, so the
// bounds are what make it safe to offer at all.
func TestOperationalLimitsRejectValuesThatWouldLockTheInstance(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	refused := []map[string]any{
		{"sessionLifetimeHours": 0.001}, // signs everyone out mid-click
		{"rateLimit": 1},                // refuses the administrator's own sign-in
		{"ldapSyncMaxDeleteRatio": 1.5}, // cannot mean anything
	}

	for _, body := range refused {
		if r := admin.api(http.MethodPut, "/settings/operational", body); r.Status != http.StatusBadRequest {
			t.Errorf("%v should be refused, got %d", body, r.Status)
		}
	}
}

// ------------------------------------------------------------- guided tour

func TestTourIsOfferedOnceAndCanBeRestarted(t *testing.T) {
	t.Parallel()

	_, _, worker := startWithWorker(t)

	var me struct {
		User userResponse `json:"user"`
	}

	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if me.User.TourSeen {
		t.Fatal("a new account should be offered the tour")
	}

	worker.must(worker.api(http.MethodPut, "/me/tour", map[string]any{"seen": true}), http.StatusOK)
	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if !me.User.TourSeen {
		t.Error("the tour should be recorded as seen")
	}

	// Asking to see it again is allowed: being told you already had your chance
	// would be a strange way to treat someone who wants to look again.
	worker.must(worker.api(http.MethodPut, "/me/tour", map[string]any{"seen": false}), http.StatusOK)
	worker.must(worker.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if me.User.TourSeen {
		t.Error("the tour should be on offer again")
	}
}

// ------------------------------------------------------------------ health

func TestOperationalEndpointsAreServed(t *testing.T) {
	t.Parallel()

	a := start(t)

	for _, path := range []string{"/.well-known/alive", "/.well-known/health"} {
		resp, err := http.Get(a.BaseURL() + path)
		if err != nil {
			t.Errorf("%s: %v", path, err)

			continue
		}

		_ = resp.Body.Close()

		// A container orchestrator restarts on these, so they have to answer
		// without a session.
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s should answer 200 without authentication, got %d", path, resp.StatusCode)
		}
	}
}

// Saving a record without editing any field must still work.
//
// This is where the dialects genuinely disagree: MySQL reports how many rows an
// UPDATE actually *changed*, PostgreSQL and SQLite how many it *matched*. Code
// that reads zero as "no such row" turns an ordinary save into a 404 - but only
// on MySQL, and only when nothing changed, which is why it went unnoticed until
// the suite was pointed at MySQL.
func TestSavingWithoutChangingAnythingIsNotAnError(t *testing.T) {
	t.Parallel()

	_, admin, _ := startWithWorker(t)

	var made struct {
		ID uint `json:"id"`
	}

	admin.must(admin.api(http.MethodPost, "/users", map[string]any{
		"name": "Ulla", "email": "ulla@example.com", "role": "agent", "password": "ulla-password-1",
	}), http.StatusCreated, http.StatusOK).Data(t, &made)

	// Twice with the same values. The second is the one that used to 404.
	for attempt := 1; attempt <= 2; attempt++ {
		r := admin.api(http.MethodPut, path("/users/", made.ID),
			map[string]any{"name": "Ulla", "email": "ulla@example.com"})

		if r.Status != http.StatusOK {
			t.Fatalf("attempt %d: saving an unchanged account must succeed, got %d: %s",
				attempt, r.Status, r.Body)
		}
	}

	// The same for a role, which is saved whole as well.
	var role struct {
		ID uint `json:"id"`
	}

	admin.must(admin.api(http.MethodPost, "/roles", map[string]any{
		"name": "unchanged", "description": "saved twice", "permissions": []string{"tickets:read"},
	}), http.StatusCreated, http.StatusOK).Data(t, &role)

	for attempt := 1; attempt <= 2; attempt++ {
		if r := admin.api(http.MethodPut, path("/roles/", role.ID), map[string]any{
			"description": "saved twice", "permissions": []string{"tickets:read"},
		}); r.Status != http.StatusOK {
			t.Errorf("role attempt %d: got %d: %s", attempt, r.Status, r.Body)
		}
	}
}
