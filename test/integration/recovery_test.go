//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/security"
)

// enableTOTP turns a second factor on for whoever c is signed in as.
func enableTOTP(t *testing.T, c *client) {
	t.Helper()

	setup := beginEnrolment(t, c)

	code, err := security.CurrentTOTPCode(setup.Secret)
	if err != nil {
		t.Fatalf("generate a code: %v", err)
	}

	c.must(c.api(http.MethodPut, "/me/totp", map[string]any{"code": code}), http.StatusOK, http.StatusNoContent)
}

// passwordSignIn signs in with a password alone and says whether a session was
// opened, or a second factor asked for first. A password refused opens nothing
// and asks for nothing.
func passwordSignIn(t *testing.T, a *app, email, password string) (opened *client, asked bool) {
	t.Helper()

	c := a.newClient()

	r := c.api(http.MethodPost, "/auth/login", map[string]string{"email": email, "password": password})
	if r.Status == http.StatusUnauthorized {
		return nil, false
	}

	var answer struct {
		User         *userResponse `json:"user"`
		TOTPRequired bool          `json:"totpRequired"`
	}

	c.must(r, http.StatusCreated, http.StatusOK).Data(t, &answer)

	if answer.User == nil {
		return nil, answer.TOTPRequired
	}

	return c, answer.TOTPRequired
}

// whoIs is the account c is signed in as.
func whoIs(t *testing.T, c *client) userResponse {
	t.Helper()

	var me struct {
		User userResponse `json:"user"`
	}

	c.must(c.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	return me.User
}

// changeLogged reads the newest entry of the administration log.
func changeLogged(t *testing.T, c *client) (action string, actorID uint, target string) {
	t.Helper()

	var page struct {
		Entries []struct {
			Action string `json:"action"`
			Actor  struct {
				ID uint `json:"id"`
			} `json:"actor"`
			Target *struct {
				Name string `json:"name"`
			} `json:"target"`
		} `json:"entries"`
	}

	c.must(c.api(http.MethodGet, "/admin/changes", nil), http.StatusOK).Data(t, &page)

	if len(page.Entries) == 0 {
		t.Fatal("the administration log is empty")
	}

	newest := page.Entries[0]
	if newest.Target != nil {
		target = newest.Target.Name
	}

	return newest.Action, newest.Actor.ID, target
}

// An administrator removes the second factor somebody lost with their phone.
//
// Only the owner could turn it off, and only with a code from the phone that
// is gone - so a lost phone locked the account for good. Removed, the password
// signs the account in again; the passkeys go with it, since they were most
// likely on that phone, and every session ends, since the phone may still have
// been signed in. Refused for the administrator's own account, and for one with
// nothing to remove, and recorded in the administration log.
func TestAnAdministratorRemovesTheSecondFactorSomebodyLost(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")
	phone := a.signInAsUser(admin, "Wera", "wera@example.com")

	enableTOTP(t, phone)

	wera := whoIs(t, phone)

	// A passkey on the same phone, put in through the application's own
	// repository: registering one takes an authenticator, and what is asked
	// here is only that it goes.
	keys := sqldb.NewPasskeyRepository(a.DB(t), a.Dialect())
	if _, err := keys.Save(context.Background(), &model.Passkey{
		UserID: wera.ID, Name: "phone", CredentialID: []byte("the-lost-phone"),
		PublicKey: []byte("its-key"), CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("give Wera a passkey: %v", err)
	}

	if _, asked := passwordSignIn(t, a, "wera@example.com", "another-password-1"); !asked {
		t.Fatal("the password alone signed in an account with a second factor, so there is nothing to remove")
	}

	path := fmt.Sprintf("/users/%d/second-factor", wera.ID)

	if refused := phone.api(http.MethodDelete, "/users/1/second-factor", nil); refused.Status != http.StatusForbidden {
		t.Errorf("an account without users:write removed a second factor: %d", refused.Status)
	}

	admin.must(admin.api(http.MethodDelete, path, nil), http.StatusOK, http.StatusNoContent)

	back, asked := passwordSignIn(t, a, "wera@example.com", "another-password-1")
	if asked || back == nil {
		t.Error("after its second factor was removed, the account still asked for one")
	}

	if n := a.Rows(t, "passkeys", "user_id = ?", wera.ID); n != 0 {
		t.Errorf("the account still holds %d passkeys", n)
	}

	if r := phone.api(http.MethodGet, "/me", nil); r.Status != http.StatusUnauthorized {
		t.Errorf("the session the lost phone held still answers %d", r.Status)
	}

	if action, _, target := changeLogged(t, admin); action != "user.secondFactor" || target != "Wera" {
		t.Errorf("the administration log's newest entry is %q for %q, want user.secondFactor for Wera", action, target)
	}

	again := admin.api(http.MethodDelete, path, nil)
	if got := refusalOf(t, again); again.Status != http.StatusConflict || got.Code != "noSecondFactor" {
		t.Errorf("removing a second factor that is not there answered %d %q, want 409 noSecondFactor",
			again.Status, got.Code)
	}

	own := admin.api(http.MethodDelete, fmt.Sprintf("/users/%d/second-factor", whoIs(t, admin).ID), nil)
	if got := refusalOf(t, own); own.Status != http.StatusConflict || got.Code != "cannotRemoveOwnSecondFactor" {
		t.Errorf("an administrator removing their own second factor got %d %q, want 409 "+
			"cannotRemoveOwnSecondFactor", own.Status, got.Code)
	}
}
