//go:build integration

package integration

import (
	"net/http"
	"testing"
)

// Deleting things that other things point at.
//
// The schema declares foreign keys - user_id REFERENCES users(id) - with no ON
// DELETE clause, so the default is to refuse. Nothing sets PRAGMA foreign_keys
// on SQLite, where enforcement is off unless asked for. That combination means
// the same request can behave differently depending on which database an
// installation chose, which is exactly the kind of difference these tests exist
// to catch: the unit tests use an in-memory repository and cannot see it at all.
//
// What has to be true, whichever dialect is underneath: either the deletion
// takes everything that belonged to the account with it, or it is refused and
// says why. What must not happen is a half-deletion - an account gone with rows
// still pointing at it, or its passkeys still able to sign in.

// An account with nothing attached is deleted without being asked about, on every
// dialect - it is the only deletion with no ambiguity about it, and asking about
// something with no consequence trains people to click through the dialog that has
// one.
func TestDeletingAnAccountWithNothingAttached(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.signInAsAdmin("a-much-better-password")

	var created struct {
		ID uint `json:"id"`
	}

	c.must(c.api(http.MethodPost, "/users", map[string]any{
		"name": "Passing through", "email": "transient@example.com",
		"role": "agent", "password": "transient-password-1",
	}), http.StatusCreated, http.StatusOK).Data(t, &created)

	c.must(c.api(http.MethodDelete, path("/users/", created.ID), nil),
		http.StatusOK, http.StatusNoContent)

	if got := c.api(http.MethodGet, path("/users/", created.ID), nil).Status; got != http.StatusNotFound {
		t.Errorf("the deleted account reads as %d, want 404", got)
	}
}

// A role with permissions is two tables. Replacing its permissions deletes them
// all and inserts the new set, so a failure part-way leaves a role that grants
// less than it says - or nothing at all, taking every user on it with it.
func TestChangingARolesPermissionsIsAllOrNothing(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.signInAsAdmin("a-much-better-password")

	var role struct {
		ID          uint     `json:"id"`
		Permissions []string `json:"permissions"`
	}

	c.must(c.api(http.MethodPost, "/roles", map[string]any{
		"name": "reading", "description": "reads tickets and the knowledge base",
		"permissions": []string{"tickets:read", "faq:read"},
	}), http.StatusCreated, http.StatusOK).Data(t, &role)

	// A permission that does not exist has to be refused - and refusing after
	// the delete has already happened would leave the role stripped.
	refused := c.api(http.MethodPut, path("/roles/", role.ID), map[string]any{
		"name": "reading", "description": "reads tickets and the knowledge base",
		"permissions": []string{"tickets:read", "not:a:permission"},
	})

	if refused.Status == http.StatusOK {
		t.Fatal("a permission this application does not enforce was accepted")
	}

	var after struct {
		Permissions []string `json:"permissions"`
	}

	c.must(c.api(http.MethodGet, path("/roles/", role.ID), nil), http.StatusOK).Data(t, &after)

	if len(after.Permissions) != 2 {
		t.Errorf("the role now holds %v; a refused change left it altered, which is "+
			"how everyone on this role silently loses access", after.Permissions)
	}
}

// The built-in administrator stays, confirmation or not: it is the way back into
// an installation.
func TestTheBuiltInAdministratorCannotBeDeletedEvenWithConfirmation(t *testing.T) {
	t.Parallel()

	a := start(t)
	c := a.signInAsAdmin("a-much-better-password")

	var me struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}

	c.must(c.api(http.MethodGet, "/me", nil), http.StatusOK).Data(t, &me)

	if got := c.api(http.MethodDelete, path("/users/", me.User.ID, "?confirm=true"), nil).Status; got == http.StatusOK {
		t.Fatal("the built-in administrator was deleted")
	}

	c.must(c.api(http.MethodGet, "/me", nil), http.StatusOK)
}
