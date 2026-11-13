package service_test

import (
	"context"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	"github.com/dennis-dko/go-ticket-system/internal/domain/service"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/memory"
)

// Moving an account between roles has one real refusal in it, and the refusal
// is the point: moving the built-in administrator to a role without
// administration rights would lock an installation out of its own user
// management. It is reachable through the API, so it is something a caller can
// try.

type fixture struct {
	users repository.UserRepository
	roles repository.RoleRepository

	userDomain *service.UserDomainService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	users := memory.NewUserRepository()
	roles := memory.NewRoleRepository(users)

	return &fixture{
		users:      users,
		roles:      roles,
		userDomain: service.NewUserDomainService(users, roles),
	}
}

// role returns the named role, creating it only if the repository does not
// already have one.
//
// The memory repository seeds the standard roles, and saving a second "agent"
// would leave two - with GetByName answering the first, so the test would compare
// against a role nothing points at.
func (f *fixture) role(t *testing.T, name string, permissions ...string) *model.Role {
	t.Helper()

	if existing, err := f.roles.GetByName(context.Background(), name); err == nil && existing != nil {
		return existing
	}

	created, err := f.roles.Save(context.Background(),
		&model.Role{Name: name, Permissions: permissions})
	if err != nil {
		t.Fatalf("seed role %s: %v", name, err)
	}

	return created
}

func (f *fixture) user(t *testing.T, email string, roleID uint, system bool) *model.User {
	t.Helper()

	created, err := f.users.Save(context.Background(), &model.User{
		Name: email, Email: email, RoleID: roleID, IsSystem: system,
	})
	if err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}

	return created
}

func TestAssigningARoleToAnOrdinaryUser(t *testing.T) {
	f := newFixture(t)
	everyday := f.role(t, model.RoleAgent, model.AgentPermissions()...)

	// A custom role an installation could create, which is all this case needs -
	// somewhere to move somebody.
	oversight := f.role(t, "oversight", model.PermTicketRead, model.PermReportRead)
	user := f.user(t, "someone@example.com", everyday.ID, false)

	updated, err := f.userDomain.AssignRoleToUser(context.Background(), user.ID, "oversight")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	if updated.RoleID != oversight.ID {
		t.Errorf("the user is on role %d, want %d", updated.RoleID, oversight.ID)
	}
}

// The built-in administrator exists so an installation always has a way back
// in. A role that cannot administer roles is not a way back in - and this is
// reachable over the API, so it is something somebody can try by accident while
// tidying up permissions.
func TestTheBuiltInAdministratorCannotLoseAdministrationRights(t *testing.T) {
	f := newFixture(t)
	admin := f.role(t, model.RoleAdmin, model.AdminPermissions()...)
	agent := f.role(t, model.RoleAgent, model.AgentPermissions()...)
	system := f.user(t, "admin@local", admin.ID, true)

	_, err := f.userDomain.AssignRoleToUser(context.Background(), system.ID, model.RoleAgent)
	if err == nil {
		t.Fatal("the built-in administrator was moved to a role that cannot administer")
	}

	// And the refusal has to be an actual refusal, not a rollback after the fact.
	after, readErr := f.users.GetByID(context.Background(), system.ID)
	if readErr != nil {
		t.Fatalf("read the administrator back: %v", readErr)
	}

	if after.RoleID != admin.ID {
		t.Errorf("the role changed to %d despite the refusal", after.RoleID)
	}

	if after.RoleID == agent.ID {
		t.Error("the administrator ended up on the agent role")
	}
}

// Moving it to a *different* role that can still administer is fine - the rule
// is about the capability, not about pinning it to one particular role.
func TestTheBuiltInAdministratorCanMoveToAnotherAdministeringRole(t *testing.T) {
	f := newFixture(t)
	admin := f.role(t, model.RoleAdmin, model.AdminPermissions()...)
	other := f.role(t, "superuser", model.PermRoleWrite, model.PermUserWrite)
	system := f.user(t, "admin@local", admin.ID, true)

	updated, err := f.userDomain.AssignRoleToUser(context.Background(), system.ID, "superuser")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	if updated.RoleID != other.ID {
		t.Errorf("the administrator is on role %d, want %d", updated.RoleID, other.ID)
	}
}

func TestAssigningARoleThatDoesNotExist(t *testing.T) {
	f := newFixture(t)
	everyday := f.role(t, model.RoleAgent, model.AgentPermissions()...)
	user := f.user(t, "someone@example.com", everyday.ID, false)

	if _, err := f.userDomain.AssignRoleToUser(context.Background(), user.ID, "wizard"); err == nil {
		t.Error("a role that does not exist was assigned")
	}
}

func TestAssigningARoleToAUserThatDoesNotExist(t *testing.T) {
	f := newFixture(t)
	f.role(t, model.RoleAgent, model.AgentPermissions()...)

	if _, err := f.userDomain.AssignRoleToUser(context.Background(), 9999, model.RoleAgent); err == nil {
		t.Error("a role was assigned to a user that does not exist")
	}
}
