//go:build integration

package integration

import (
	"net/http"
	"slices"
	"testing"
)

// The built-in administrator administers the installation, and an account given
// the admin role is that administrator in every way that matters.
//
// Enforced by the same per-endpoint checks that refuse an agent, which is what
// these tests go through. A guarantee that only holds in the interface is not a
// guarantee; the interface hides what it cannot do, and a token client does not.
func TestTheAdministratorAdministers(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	// Accounts and roles.
	admin.must(admin.api(http.MethodPost, "/users", map[string]any{
		"name": "Olaf", "email": "olaf@example.com",
		"role": "agent", "password": "olaf-password-1",
	}), http.StatusCreated, http.StatusOK)

	admin.must(admin.api(http.MethodGet, "/users", nil), http.StatusOK)
	admin.must(admin.api(http.MethodGet, "/roles", nil), http.StatusOK)

	admin.must(admin.api(http.MethodPost, "/roles", map[string]any{
		"name": "reading", "description": "reads tickets and nothing else",
		"permissions": []string{"tickets:read"},
	}), http.StatusCreated, http.StatusOK)

	// The installation itself.
	admin.must(admin.api(http.MethodGet, "/setup", nil), http.StatusOK)
	admin.must(admin.api(http.MethodPut, "/settings/operational",
		map[string]any{"sessionIdleMinutes": 60}), http.StatusOK)
}

// The administrator's role is fixed: neither widened nor narrowed nor described.
//
// The application looks the role up by name, so stripping it would leave the
// installation unadministrable - and widening it would hand the one account nobody
// chose a right somebody did not decide to give, quietly, from the screen that
// administers roles. The customer portal is the right to try that with: it is the
// one the role does not hold, because it means something only for an account
// linked to a contact.
func TestTheAdministratorsRightsCannotBeChangedAtAll(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	var roles listOf[struct {
		ID          uint     `json:"id"`
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}]

	admin.must(admin.api(http.MethodGet, "/roles", nil), http.StatusOK).Data(t, &roles)

	var adminRole uint

	var held []string

	for _, role := range roles.Items {
		if role.Name == "admin" {
			adminRole = role.ID
			held = role.Permissions
		}
	}

	if adminRole == 0 {
		t.Fatal("there is no admin role")
	}

	// Everything on the staff side, and the portal not.
	for _, permission := range []string{"settings:manage", "roles:write", "users:write", "tickets:write", "queues:write"} {
		if !slices.Contains(held, permission) {
			t.Errorf("the seeded admin role does not hold %q", permission)
		}
	}

	if slices.Contains(held, "portal:use") {
		t.Error("the seeded admin role holds portal:use, which is a customer's")
	}

	widened := admin.api(http.MethodPut, path("/roles/", adminRole), map[string]any{
		"permissions": append(append([]string{}, held...), "portal:use"),
	})

	if widened.Status == http.StatusOK {
		t.Error("the administrator widened its own role")
	}

	narrowed := admin.api(http.MethodPut, path("/roles/", adminRole), map[string]any{
		"permissions": []string{"users:read"},
	})

	if narrowed.Status == http.StatusOK {
		t.Error("the administrator stripped its own role")
	}

	// Sending the set it already holds is not a change, and must not be refused as one:
	// a screen that returns its checkboxes in a different order would otherwise fail to
	// save for no reason anybody could see.
	admin.must(admin.api(http.MethodPut, path("/roles/", adminRole), map[string]any{
		"permissions": held,
	}), http.StatusOK)

	// And the description is refused too. It explains rather than grants, so it
	// looked like the installation's business - but the shipped roles are the ones
	// the interface translates, keyed on the name, and a description edited here is
	// overridden on screen by the translation. Nothing about these roles is editable,
	// which is also what their screen offers: they are shown rather than opened.
	described := admin.api(http.MethodPut, path("/roles/", adminRole), map[string]any{
		"description": "Runs this installation",
	})

	if described.Status != http.StatusConflict {
		t.Errorf("describing the administrator's role answered %d, want %d",
			described.Status, http.StatusConflict)
	}
}

// A role has to grant something.
//
// One that grants nothing can be created and assigned, and whoever holds it signs in to
// an interface with almost nothing on it and no screen that matters - which reads as a
// broken installation rather than as a decision. Taking somebody's access away is what
// removing the account is for.
func TestARoleMustGrantAtLeastOnePermission(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	for _, attempt := range []struct {
		what string
		body map[string]any
	}{
		{"no permissions at all", map[string]any{
			"name": "hollow", "description": "grants nothing"}},
		{"an empty list", map[string]any{
			"name": "hollow", "description": "grants nothing", "permissions": []string{}}},
		{"a list of blanks", map[string]any{
			"name": "hollow", "description": "grants nothing",
			"permissions": []string{"", "  "}}},
	} {
		r := admin.api(http.MethodPost, "/roles", attempt.body)

		if r.Status == http.StatusCreated || r.Status == http.StatusOK {
			t.Errorf("a role with %s was created", attempt.what)

			continue
		}

		if r.Message() == "" {
			t.Errorf("a role with %s was refused without saying why", attempt.what)
		}
	}

	// And one that grants something is created, so this is not simply refusing
	// everything.
	admin.must(admin.api(http.MethodPost, "/roles", map[string]any{
		"name": "reader", "description": "reads the accounts",
		"permissions": []string{"users:read"},
	}), http.StatusCreated, http.StatusOK)
}

// The directory synchronisation is the installation's, and so is its schedule.
//
// A run deletes every account the directory no longer holds and leaves their tickets
// without an owner, and a schedule is the same run performed later and unattended -
// so all three, the connection, the run and the schedule, are refused to an account
// that does not administer the installation, the way the rest of its settings are.
func TestOnlyAnAdministratorRunsOrSchedulesTheDirectorySynchronisation(t *testing.T) {
	t.Parallel()

	_, admin, agent := startWithWorker(t)

	if got := agent.api(http.MethodGet, "/settings/ldap", nil).Status; got != http.StatusForbidden {
		t.Errorf("an agent read the directory settings: %d, want %d", got, http.StatusForbidden)
	}

	var stored map[string]any

	admin.must(admin.api(http.MethodGet, "/settings/ldap", nil), http.StatusOK).Data(t, &stored)
	stored["syncSchedule"] = "0 4 * * *"

	if got := agent.api(http.MethodPut, "/settings/ldap", stored).Status; got != http.StatusForbidden {
		t.Errorf("an agent scheduled the directory run: %d, want %d", got, http.StatusForbidden)
	}

	for _, route := range []string{"/settings/ldap/sync/preview", "/settings/ldap/sync"} {
		if got := agent.api(http.MethodPost, route, nil).Status; got != http.StatusForbidden {
			t.Errorf("%s answered an agent %d, want %d", route, got, http.StatusForbidden)
		}
	}

	// And the administrator can, so this is not simply a broken endpoint.
	admin.must(admin.api(http.MethodPut, "/settings/ldap", stored), http.StatusOK)
}

// An account holding the admin role is the built-in administrator in every way
// that matters.
//
// A handful of things once asked whether the caller *was* the built-in account
// rather than whether it administers: the directory synchronisation, its schedule,
// and the setup wizard. So an installation that had handed its administration to a
// person still had to sign in as the account nobody can attribute to one - which is
// the opposite of what giving somebody the role was for.
func TestTheAdminRoleIsTreatedAsTheBuiltInAdministrator(t *testing.T) {
	t.Parallel()

	a := start(t)
	admin := a.signInAsAdmin("a-much-better-password")

	granted := a.signInAsGrantedAdministrator(admin, "Gerd", "gerd@example.com")

	// The setup wizard.
	granted.must(granted.api(http.MethodGet, "/setup", nil), http.StatusOK)

	// The directory run, both by hand and on a schedule. A preview against no
	// configured directory fails for its own reasons; what is asserted is that it
	// is not refused as forbidden, which is the question here.
	for _, route := range []string{"/settings/ldap/sync/preview", "/settings/ldap/sync"} {
		if got := granted.api(http.MethodPost, route, nil).Status; got == http.StatusForbidden {
			t.Errorf("%s refused an account holding the admin role", route)
		}
	}

	var stored map[string]any

	granted.must(granted.api(http.MethodGet, "/settings/ldap", nil), http.StatusOK).Data(t, &stored)
	stored["syncSchedule"] = "0 4 * * *"

	granted.must(granted.api(http.MethodPut, "/settings/ldap", stored), http.StatusOK)
}
