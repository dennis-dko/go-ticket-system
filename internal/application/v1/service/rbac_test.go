package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/command"
	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

func TestDefaultRolesAreSeeded(t *testing.T) {
	f := newFixture(t)

	roles, err := f.roles.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}

	byName := make(map[string]*model.Role, len(roles))
	for _, role := range roles {
		byName[role.Name] = role
	}

	for _, expected := range []string{model.RoleAdmin, model.RoleAgent, model.RoleCustomer} {
		if _, ok := byName[expected]; !ok {
			t.Fatalf("expected default role %q to exist", expected)
		}
	}

	// The admin role must be able to administer, or an installation could end
	// up with nobody able to manage roles.
	if !byName[model.RoleAdmin].Has(model.PermRoleWrite) {
		t.Error("the admin role must hold roles:write")
	}

	// Somebody who works on tickets does not administer: an agent that could
	// change roles or accounts could make itself whatever it liked.
	for _, administering := range model.AdministeringPermissions() {
		if byName[model.RoleAgent].Has(administering) {
			t.Errorf("the agent role holds %q, which administers the installation", administering)
		}
	}

	// A customer reaches the portal and nothing on the staff side, and nobody on
	// the staff side is a customer: the portal shows an account its own contact's
	// tickets, and a staff account has no contact.
	customer := byName[model.RoleCustomer]
	if len(customer.Permissions) != 1 || !customer.Has(model.PermPortalUse) {
		t.Errorf("the customer role holds %v, want the portal and nothing else", customer.Permissions)
	}

	for _, staff := range []string{model.RoleAdmin, model.RoleAgent} {
		if byName[staff].Has(model.PermPortalUse) {
			t.Errorf("the %s role holds %q, which is the customer's side", staff, model.PermPortalUse)
		}
	}
}

func TestSystemRoleCannotBeDeletedOrWeakened(t *testing.T) {
	f := newFixture(t)

	admin := roleNamed(t, f, model.RoleAdmin)

	requireKind(t, f.roles.DeleteRole(context.Background(), admin.ID), apperror.KindConflict)

	// Stripping permissions from the system role would lock the installation
	// out of its own administration.
	_, err := f.roles.UpdateRole(context.Background(), admin.ID, nil, nil, []string{model.PermUserRead})
	requireKind(t, err, apperror.KindConflict)

	renamed := "superuser"
	_, err = f.roles.UpdateRole(context.Background(), admin.ID, &renamed, nil, nil)
	requireKind(t, err, apperror.KindConflict)
}

// roleNamed returns the seeded role with the given name.
func roleNamed(t *testing.T, f *fixture, name string) *model.Role {
	t.Helper()

	roles, err := f.roles.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}

	for _, role := range roles {
		if role.Name == name {
			return role
		}
	}

	t.Fatalf("role %q not found", name)

	return nil
}

func TestCreateRoleRejectsUnknownPermission(t *testing.T) {
	f := newFixture(t)

	_, err := f.roles.CreateRole(context.Background(), "auditor", "",
		[]string{model.PermUserRead, "tickets:invent"})
	requireKind(t, err, apperror.KindInvalid)
}

// A role's description is held to the length every other description is, which
// the form already said: it took anything at all over the API.
func TestARolesDescriptionIsHeldToTheLengthOfADescription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	long := strings.Repeat("a", model.MaxDescriptionLength+1)

	_, err := f.roles.CreateRole(ctx, "auditor", long, []string{model.PermUserRead})
	requireKind(t, err, apperror.KindInvalid)

	role, err := f.roles.CreateRole(ctx, "auditor", strings.Repeat("a", model.MaxDescriptionLength), []string{model.PermUserRead})
	if err != nil {
		t.Fatalf("a description of exactly the length allowed is refused: %v", err)
	}

	_, err = f.roles.UpdateRole(ctx, role.ID, nil, &long, nil)
	requireKind(t, err, apperror.KindInvalid)
}

func TestRoleInUseCannotBeDeleted(t *testing.T) {
	f := newFixture(t)

	role, err := f.roles.CreateRole(context.Background(), "auditor", "read only",
		[]string{model.PermUserRead})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}

	if _, err := f.users.CreateUser(context.Background(), command.CreateUserCommand{
		Name: "Auditor", Email: "auditor@example.com", Role: "auditor",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	requireKind(t, f.roles.DeleteRole(context.Background(), role.ID), apperror.KindConflict)
}

// signIn is the sign-in the application actually performs.
//
// The tests here used to call AuthService.Authenticate, which the application
// never calls: a second implementation of the password check, differing from the
// real one in that it did not refuse a directory-backed account. Testing it
// proved something about code nobody ran, and left the path that does run
// covered only by the integration suite.
func signIn(t *testing.T, f *fixture, email, password string) *service.Principal {
	t.Helper()

	result, err := f.sessions.Login(context.Background(), email, password, "")
	if err != nil {
		t.Fatalf("signing in as %s: %v", email, err)
	}

	return result.Principal
}

// signInIsRefused is signIn for the cases that are meant to be turned away.
func signInIsRefused(t *testing.T, f *fixture, email, password string) {
	t.Helper()

	if _, err := f.sessions.Login(context.Background(), email, password, ""); err == nil {
		t.Fatalf("signing in as %s with %q was accepted", email, password)
	}
}

func TestSystemUserIsProtected(t *testing.T) {
	f := newFixture(t)

	created, err := f.auth.EnsureSystemUser(context.Background())
	if err != nil {
		t.Fatalf("ensure system user: %v", err)
	}

	if !created {
		t.Fatal("expected the system user to be created on first call")
	}

	// Calling again must not create a second one.
	again, err := f.auth.EnsureSystemUser(context.Background())
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}

	if again {
		t.Error("the system user must only be created once")
	}

	principal := signIn(t, f, service.SystemUserEmail, service.SystemUserPassword)

	if !principal.User.IsSystem {
		t.Error("the built-in administrator must be flagged as a system user")
	}

	if !principal.User.MustChangePassword {
		t.Error("the initial password must be flagged for replacement")
	}

	// Deleting it would leave the installation with no guaranteed way back in.
	err = f.users.DeleteUser(context.Background(),
		command.DeleteUserCommand{ID: principal.User.ID})
	requireKind(t, err, apperror.KindConflict)

	// Nor may it be demoted to a role that cannot administer.
	_, err = f.userDomain.AssignRoleToUser(context.Background(),
		principal.User.ID, model.RoleAgent)
	requireKind(t, err, apperror.KindConflict)
}

func TestAuthenticationRejectsWrongPassword(t *testing.T) {
	f := newFixture(t)

	if _, err := f.auth.EnsureSystemUser(context.Background()); err != nil {
		t.Fatalf("ensure system user: %v", err)
	}

	signInIsRefused(t, f, service.SystemUserEmail, "not-the-password")

	// An unknown account must fail the same way as a wrong password, so the
	// response cannot be used to discover which addresses exist. Both refusals
	// are wanted here rather than only the fact of them, so this asks for them
	// directly instead of going through the helper.
	_, unknownErr := f.sessions.Login(context.Background(), "nobody@example.com", "whatever", "")
	_, wrongErr := f.sessions.Login(context.Background(), service.SystemUserEmail, "wrong", "")

	if unknownErr == nil || wrongErr == nil {
		t.Fatal("both cases must fail")
	}

	if unknownErr.Error() != wrongErr.Error() {
		t.Errorf("failures must be indistinguishable: %q vs %q", unknownErr, wrongErr)
	}
}

func TestChangePassword(t *testing.T) {
	f := newFixture(t)

	if _, err := f.auth.EnsureSystemUser(context.Background()); err != nil {
		t.Fatalf("ensure system user: %v", err)
	}

	principal := signIn(t, f, service.SystemUserEmail, service.SystemUserPassword)

	// Too short.
	err := f.auth.ChangePassword(context.Background(), principal.User.ID,
		service.SystemUserPassword, "short")
	requireKind(t, err, apperror.KindInvalid)

	// Wrong current password.
	err = f.auth.ChangePassword(context.Background(), principal.User.ID,
		"wrong-current", "a-much-better-password")
	requireKind(t, err, apperror.KindInvalid)

	if err := f.auth.ChangePassword(context.Background(), principal.User.ID,
		service.SystemUserPassword, "a-much-better-password"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	signInIsRefused(t, f, service.SystemUserEmail, service.SystemUserPassword)

	updated := signIn(t, f, service.SystemUserEmail, "a-much-better-password")

	if updated.User.MustChangePassword {
		t.Error("changing the password must clear the must-change flag")
	}
}

func TestPrincipalPermissionsComeFromRole(t *testing.T) {
	f := newFixture(t)

	if _, err := f.users.CreateUser(context.Background(), command.CreateUserCommand{
		Name: "Worker", Email: "worker@example.com", Role: model.RoleAgent,
		Password: "worker-password",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	principal := signIn(t, f, "worker@example.com", "worker-password")

	if !principal.Can(model.PermTicketWrite) {
		t.Error("an agent must be able to work on tickets")
	}

	if principal.Can(model.PermUserDelete) {
		t.Error("a user must not be able to delete users")
	}
}
