package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/command"
	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	domainservice "github.com/dennis-dko/go-ticket-system/internal/domain/service"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/memory"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

// fixture wires the services against in-memory repositories and seeds one
// account, which nearly every case needs.
type fixture struct {
	users *service.UserApplicationService
	roles *service.RoleApplicationService
	auth  *service.AuthService

	// sessions is the real sign-in path: an address and a password in, a
	// principal out. Tests ask it about authentication rather than asking
	// AuthService, which the application never calls for that.
	sessions *service.SessionService

	// domain service, exercised where a rule lives in the domain layer
	userDomain *domainservice.UserDomainService

	// repositories, so a test can build a service the fixture does not wire
	userRepo *memory.UserRepository
	roleRepo *memory.RoleRepository

	// owned is how many unfinished tickets each account owns, for the cases
	// about what an account leaves behind.
	owned *ownedTickets

	userID uint
}

// ownedTickets stands in for the tickets accounts own: a number per account,
// which is all a deletion asks about them.
type ownedTickets struct {
	byUser map[uint]int
}

func (o *ownedTickets) CountUnfinishedOwnedBy(_ context.Context, userID uint) (int, error) {
	return o.byUser[userID], nil
}

// memoryPurger stands in for the SQL purge in the unit tests: it releases what
// the account owned and then removes the account.
//
// Deliberately not a no-op stub. The service decides whether a deletion is
// allowed, and a purger that removed nothing would let a test pass while the
// account survived. Whether the *real* erasure covers every table is a question
// about SQL, and the integration tests ask it against all three databases.
type memoryPurger struct {
	users repository.UserRepository
	owned *ownedTickets
}

func (p *memoryPurger) PurgeUser(ctx context.Context, userID uint) error {
	delete(p.owned.byUser, userID)

	return p.users.Delete(ctx, userID)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	userRepo := memory.NewUserRepository()
	roleRepo := memory.NewRoleRepository(userRepo)
	sessionRepo := memory.NewSessionRepository()
	owned := &ownedTickets{byUser: map[uint]int{}}

	auth := service.NewAuthService(userRepo, roleRepo)

	f := &fixture{
		users: service.NewUserApplicationService(userRepo, roleRepo, owned,
			&memoryPurger{users: userRepo, owned: owned}),
		roles:      service.NewRoleApplicationService(roleRepo),
		auth:       auth,
		sessions:   service.NewSessionService(userRepo, roleRepo, sessionRepo, auth, time.Hour),
		userDomain: domainservice.NewUserDomainService(userRepo, roleRepo),
		userRepo:   userRepo,
		roleRepo:   roleRepo,
		owned:      owned,
	}

	user, err := f.users.CreateUser(context.Background(), command.CreateUserCommand{
		Name: "Dennis", Email: "dennis@example.com", Role: model.RoleAgent,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	f.userID = user.Result.ID

	return f
}

func requireKind(t *testing.T, err error, want apperror.Kind) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected an error of kind %v, got nil", want)
	}

	if got := apperror.KindOf(err); got != want {
		t.Fatalf("expected kind %v, got %v (%v)", want, got, err)
	}
}
