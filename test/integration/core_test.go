//go:build integration

package integration

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// A fresh installation has to be usable and safe from the first request. These
// cover the path everybody takes before anything else works.

func TestFreshInstallServesTheInterfaceAndTheAPI(t *testing.T) {
	t.Parallel()

	a := start(t)

	page := a.newClient().do(http.MethodGet, "/", nil)
	if page.Status != http.StatusOK {
		t.Fatalf("the interface must be served, got %d", page.Status)
	}

	// The embedded assets, not a directory on disk - which is what makes the
	// single-binary claim true.
	for _, asset := range []string{"/app.css", "/app.js", "/theme.js", "/openapi.json"} {
		if r := a.newClient().do(http.MethodGet, asset, nil); r.Status != http.StatusOK {
			t.Errorf("%s must be served from the binary, got %d", asset, r.Status)
		}
	}

	if !strings.Contains(string(page.Body), `<html lang="en"`) {
		t.Error("English is the source language; the page should say so")
	}
}

func TestTheInitialPasswordMustBeChangedBeforeAnythingElse(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.newClient()

	user := c.signIn(adminEmail, adminPassword)
	if !user.MustChangePassword {
		t.Fatal("a fresh administrator must be flagged to change the password")
	}

	// The banner says the rest of the application is locked; the server has to
	// actually lock it, or the banner is a suggestion.
	if r := c.api(http.MethodGet, "/roles", nil); r.Status != http.StatusConflict {
		t.Errorf("expected the API to be refused, got %d", r.Status)
	}

	// Two things stay reachable on purpose: seeing who you are, and the wizard
	// that tells you what to do about it.
	c.must(c.api(http.MethodGet, "/me", nil), http.StatusOK)
	c.must(c.api(http.MethodGet, "/setup", nil), http.StatusOK)

	c.must(c.api(http.MethodPut, "/me/password", map[string]string{
		"currentPassword": adminPassword,
		"newPassword":     "a-much-better-password",
	}), http.StatusOK)

	// The session that made the change survives it, which is the point: this is the
	// device that just proved it knew the old password, and signing it out drops
	// somebody at a sign-in screen in the middle of setting the installation up.
	c.must(c.api(http.MethodGet, "/me", nil), http.StatusOK)

	// And the lock is lifted for it, without a second sign-in.
	c.must(c.api(http.MethodGet, "/roles", nil), http.StatusOK)
}

// Every OTHER session of that account does end, which is the half that protects
// something: a session opened with the old password somewhere else must stop
// working the moment the password changes.
func TestOtherSessionsDoNotSurviveAPasswordChange(t *testing.T) {
	t.Parallel()

	a := start(t)

	// Two sessions of the same account, both signed in with the initial password.
	first := a.newClient()
	first.signIn(adminEmail, adminPassword)

	second := a.newClient()
	second.signIn(adminEmail, adminPassword)

	first.must(first.api(http.MethodPut, "/me/password", map[string]string{
		"currentPassword": adminPassword,
		"newPassword":     "a-much-better-password",
	}), http.StatusOK)

	if r := second.api(http.MethodGet, "/me", nil); r.Status == http.StatusOK {
		t.Error("a session opened with the old password still works")
	}

	// The one that changed it is untouched.
	first.must(first.api(http.MethodGet, "/me", nil), http.StatusOK)
}

func TestTheOldPasswordStopsWorking(t *testing.T) {
	t.Parallel()

	a := start(t)
	a.signInAsAdmin("a-much-better-password")

	c := a.newClient()
	if r := c.api(http.MethodPost, "/auth/login", map[string]string{
		"email": adminEmail, "password": adminPassword,
	}); r.Status == http.StatusCreated || r.Status == http.StatusOK {
		t.Fatal("the documented initial password must stop working once changed")
	}
}

// A wrong password and an unknown account must be indistinguishable, or the
// response becomes a way to discover which addresses exist.
func TestFailedSignInsAreIndistinguishable(t *testing.T) {
	t.Parallel()

	a := start(t)
	a.signInAsAdmin("a-much-better-password")

	wrongPassword := a.newClient().api(http.MethodPost, "/auth/login", map[string]string{
		"email": adminEmail, "password": "not-the-password",
	})

	unknownAccount := a.newClient().api(http.MethodPost, "/auth/login", map[string]string{
		"email": "nobody@example.com", "password": "not-the-password",
	})

	if wrongPassword.Status != unknownAccount.Status {
		t.Errorf("statuses differ: %d vs %d", wrongPassword.Status, unknownAccount.Status)
	}

	if wrongPassword.Message() != unknownAccount.Message() {
		t.Errorf("messages differ: %q vs %q", wrongPassword.Message(), unknownAccount.Message())
	}
}

// And they must take the same time, which is the half the words do not cover.
//
// The message and the status were identical long before this test existed. The
// duration was not: an address nobody holds was refused as soon as the lookup
// missed, while a real address ran bcrypt first. That is one millisecond against
// sixty, from the same endpoint, with the same sentence on screen - so anybody
// with a list of names and a stopwatch could read back which of them work here,
// one request each, learning nothing about passwords and everything about
// people.
//
// Medians of interleaved samples, so a slow moment on the machine running this
// lands on both sides rather than on one. The margin is wide because the
// difference it guards against is not: the ratio was near zero before and near
// one after, and half is nowhere near either edge.
func TestFailedSignInsTakeTheSameTime(t *testing.T) {
	t.Parallel()

	a := start(t)
	a.signInAsAdmin("a-much-better-password")

	const samples = 5

	var real, absent []time.Duration

	timed := func(email string) time.Duration {
		started := time.Now()

		a.newClient().api(http.MethodPost, "/auth/login", map[string]string{
			"email": email, "password": "not-the-password",
		})

		return time.Since(started)
	}

	for range samples {
		real = append(real, timed(adminEmail))
		absent = append(absent, timed("nobody@example.com"))
	}

	median := func(taken []time.Duration) time.Duration {
		slices.Sort(taken)

		return taken[len(taken)/2]
	}

	withAccount, without := median(real), median(absent)

	if without*2 < withAccount {
		t.Errorf("an unknown address is refused in %v and a known one in %v: the "+
			"gap is the answer to \"does this person work here\", given to anybody "+
			"who asks in the only way that cannot be seen on screen",
			without, withAccount)
	}
}

// ------------------------------------------------------------------- setup

// The database is deliberately absent from this wizard. It cannot be a step
// here, because every other step is stored in the database it would choose:
// answering it at this point would point the application at an empty one and
// leave the password change behind in the old database - so the installation
// would come back up reachable with the initial password from the
// documentation. It is settled by the installer, before the application starts.
func TestTheWizardRequiresAPasswordAndAZoneAndNotADatabase(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.newClient()
	c.signIn(adminEmail, adminPassword)

	var state struct {
		Completed bool `json:"completed"`
		Steps     []struct {
			ID       string `json:"id"`
			Done     bool   `json:"done"`
			Required bool   `json:"required"`
		} `json:"steps"`
	}

	c.must(c.api(http.MethodGet, "/setup", nil), http.StatusOK).Data(t, &state)

	if state.Completed {
		t.Error("a fresh installation cannot have completed the wizard")
	}

	required := map[string]bool{}
	for _, step := range state.Steps {
		if step.ID == "database" {
			t.Error("the wizard must not ask about the database; the installer already did")
		}

		if step.Required {
			required[step.ID] = true
		}
	}

	for _, id := range []string{"password", "timezone"} {
		if !required[id] {
			t.Errorf("%s should be required", id)
		}
	}

	// Making everything mandatory trains people to click past the wizard, and
	// then the step that mattered goes past too.
	if len(required) != 2 {
		t.Errorf("exactly password and timezone should be required, got %v", required)
	}
}

func TestSetupWizardCompletesAndStaysAway(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.signInAsAdmin("a-much-better-password")

	c.must(c.api(http.MethodPut, "/settings/timezone",
		map[string]string{"timezone": "Europe/Berlin"}), http.StatusOK)
	c.must(c.api(http.MethodPost, "/setup/complete", nil), http.StatusCreated, http.StatusOK)

	var state struct {
		Completed bool `json:"completed"`
		Steps     []struct {
			ID       string `json:"id"`
			Done     bool   `json:"done"`
			Required bool   `json:"required"`
		} `json:"steps"`
	}

	c.must(c.api(http.MethodGet, "/setup", nil), http.StatusOK).Data(t, &state)

	if !state.Completed {
		t.Error("the wizard should be recorded as dismissed")
	}

	for _, step := range state.Steps {
		if step.Required && !step.Done {
			t.Errorf("%s is required and still outstanding", step.ID)
		}
	}
}

// The wizard is a list of what is not configured yet, which is a useful thing
// for an attacker to read.
func TestSetupWizardIsAdministratorOnly(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	admin.must(admin.api(http.MethodPost, "/users", map[string]any{
		"name": "Erika", "email": "erika@example.com",
		"role": "agent", "password": "erika-password-1",
	}), http.StatusCreated, http.StatusOK)

	user := a.newClient()
	user.signIn("erika@example.com", "erika-password-1")

	if r := user.api(http.MethodGet, "/setup", nil); r.Status != http.StatusForbidden {
		t.Errorf("a user must not read the setup state, got %d", r.Status)
	}
}

// Changing the administrator password is required, and required means two
// independent things hold - because a wizard is a screen, and a screen can be
// closed.
//
// The wizard comes back while the step is outstanding, whatever "completed"
// says. And the server refuses the rest of the API regardless of the wizard,
// so an installation cannot be used on the password from the documentation
// even by someone who never opens the interface.
func TestChangingTheAdministratorPasswordCannotBeSkipped(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.newClient()
	c.signIn(adminEmail, adminPassword)

	// Settle everything else and dismiss the wizard, without touching the
	// password.
	c.must(c.api(http.MethodPut, "/settings/timezone",
		map[string]string{"timezone": "Europe/Berlin"}), http.StatusOK)
	c.must(c.api(http.MethodPost, "/setup/complete", nil), http.StatusCreated, http.StatusOK)

	var state struct {
		Completed bool `json:"completed"`
		Steps     []struct {
			ID       string `json:"id"`
			Done     bool   `json:"done"`
			Required bool   `json:"required"`
		} `json:"steps"`
	}

	c.must(c.api(http.MethodGet, "/setup", nil), http.StatusOK).Data(t, &state)

	var outstanding []string

	for _, step := range state.Steps {
		if step.Required && !step.Done {
			outstanding = append(outstanding, step.ID)
		}
	}

	if len(outstanding) != 1 || outstanding[0] != "password" {
		t.Fatalf("expected the password step to be the one outstanding, got %v", outstanding)
	}

	// Dismissing settles the optional steps only; this one brings it back.
	if state.Completed && len(outstanding) == 0 {
		t.Error("the wizard must not count as finished with the password outstanding")
	}

	// And the second guarantee, which does not depend on the interface at all.
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/roles"},
		{http.MethodGet, "/users"},
		{http.MethodPost, "/me/tokens"},
	} {
		r := c.api(call.method, call.path, map[string]any{"name": "x", "expiresInDays": 0})

		if r.Status != http.StatusConflict {
			t.Errorf("%s %s should be refused while the initial password stands, got %d",
				call.method, call.path, r.Status)
		}
	}

	// Once changed, both give way at once.
	c.must(c.api(http.MethodPut, "/me/password", map[string]string{
		"currentPassword": adminPassword,
		"newPassword":     "a-much-better-password",
	}), http.StatusOK)

	after := a.newClient()
	after.signIn(adminEmail, "a-much-better-password")
	after.must(after.api(http.MethodGet, "/roles", nil), http.StatusOK)

	after.must(after.api(http.MethodGet, "/setup", nil), http.StatusOK).Data(t, &state)

	for _, step := range state.Steps {
		if step.Required && !step.Done {
			t.Errorf("%s should be settled now", step.ID)
		}
	}
}

// Changing a password ends every other session, and keeps this one.
//
// Both halves matter and they pull in opposite directions. A session opened with
// the old password on another device has to stop working at once, or changing a
// password after losing a laptop achieves nothing. The session doing the changing
// has to survive, because it just proved it knew the old password - and because
// ending it signed people out of the setup wizard between two of its own steps,
// which is where this rule was first got wrong.
//
// Written down because the README described it as ending *every* session, which
// is the behaviour this replaced and not the one the code has.
func TestChangingAPasswordEndsTheOtherSessionsAndKeepsThisOne(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	worker := a.signInAsAgent(admin, "Wera", "wera@example.com")

	// The same account, signed in twice - a second device.
	elsewhere := a.newClient()
	elsewhere.signIn("wera@example.com", "another-password-1")
	elsewhere.must(elsewhere.api(http.MethodGet, "/me", nil), http.StatusOK)

	worker.must(worker.api(http.MethodPut, "/me/password", map[string]any{
		"currentPassword": "another-password-1",
		"newPassword":     "a-third-password-1",
	}), http.StatusOK)

	// This one carries on. Anything else and the wizard cannot finish itself.
	if got := worker.api(http.MethodGet, "/me", nil).Status; got != http.StatusOK {
		t.Errorf("the session that changed the password answers %d; it has to survive "+
			"its own change", got)
	}

	// The other one does not, which is the point of ending them at all.
	if got := elsewhere.api(http.MethodGet, "/me", nil).Status; got == http.StatusOK {
		t.Error("a session opened with the old password on another device still works")
	}
}
