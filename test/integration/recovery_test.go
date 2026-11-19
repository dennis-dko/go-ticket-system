//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/infrastructure/persistence/sqldb"
	"github.com/dennis-dko/timepulse/internal/support/security"
	"github.com/dennis-dko/timepulse/test/harness"
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

// recoveredPassword is the line the recover command prints the password on.
var recoveredPassword = regexp.MustCompile(`(?m)^ {4}([A-Z2-7]{26})\r?$`)

// The recover command lets the built-in administrator back in when nobody can
// sign in as it.
//
// The way back into an installation, run on the server beside the application
// rather than through it: the password chosen at setup is lost, and the second
// factor with the phone. The command prints a password once; with it the
// account signs in, has to choose its own, and has no second factor in the
// way. The old password no longer opens it, the session it had ends, and the
// administration log says it was the command line rather than an account.
func TestTheRecoverCommandLetsTheBuiltInAdministratorBackIn(t *testing.T) {
	t.Parallel()

	a := start(t)
	before := a.signInAsAdmin("a-much-better-password")

	enableTOTP(t, before)

	stdout, stderr, code := a.App.Command(t, "recover")
	if code != 0 {
		t.Fatalf("recover exited %d:\n%s\n%s", code, stdout, stderr)
	}

	found := recoveredPassword.FindStringSubmatch(stdout)
	if found == nil {
		t.Fatalf("recover printed no password:\n%s\n%s", stdout, stderr)
	}

	if r := before.api(http.MethodGet, "/me", nil); r.Status != http.StatusUnauthorized {
		t.Errorf("the session the built-in administrator had still answers %d", r.Status)
	}

	if old, asked := passwordSignIn(t, a, adminEmail, "a-much-better-password"); old != nil || asked {
		t.Error("the password from before the recovery still opens the account")
	}

	after, asked := passwordSignIn(t, a, adminEmail, found[1])
	if asked || after == nil {
		t.Fatal("the recovered password did not sign the built-in administrator in on its own")
	}

	if !whoIs(t, after).MustChangePassword {
		t.Error("the printed password is not one the account has to replace")
	}

	after.must(after.api(http.MethodPut, "/me/password", map[string]string{
		"currentPassword": found[1], "newPassword": "chosen-again-after-recovery",
	}), http.StatusOK, http.StatusNoContent)

	if action, actor, target := changeLogged(t, after); action != "user.recovered" || actor != 0 || target == "" {
		t.Errorf("the administration log's newest entry is %q by %d for %q, "+
			"want user.recovered by nobody's account for the built-in administrator", action, actor, target)
	}
}

// Where nothing is configured, the recover command says so rather than
// recovering nothing.
//
// An installation still waiting at its installer has no account, and a command
// run from the wrong directory finds no configuration - the same answer either
// way, and it names what it looked for.
func TestTheRecoverCommandSaysSoWhereNoDatabaseIsConfigured(t *testing.T) {
	t.Parallel()

	app := harness.StartUnconfigured(t)

	stdout, stderr, code := app.Command(t, "recover")
	if code == 0 {
		t.Fatalf("recover succeeded where nothing is configured:\n%s", stdout)
	}

	if !strings.Contains(stderr, "no database is configured") {
		t.Errorf("recover did not say why it could not run:\n%s", stderr)
	}
}
