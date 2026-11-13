package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
)

// countingUsers counts the questions a run puts to the accounts, by kind.
type countingUsers struct {
	repository.UserRepository

	lists, lookups, saves int
}

func (c *countingUsers) GetAll(ctx context.Context) ([]*model.User, error) {
	c.lists++

	return c.UserRepository.GetAll(ctx)
}

func (c *countingUsers) GetByID(ctx context.Context, id uint) (*model.User, error) {
	c.lookups++

	return c.UserRepository.GetByID(ctx, id)
}

func (c *countingUsers) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	c.lookups++

	return c.UserRepository.GetByEmail(ctx, email)
}

func (c *countingUsers) GetByExternalID(ctx context.Context, externalID string) (*model.User, error) {
	c.lookups++

	return c.UserRepository.GetByExternalID(ctx, externalID)
}

func (c *countingUsers) Save(ctx context.Context, user *model.User) (*model.User, error) {
	c.saves++

	return c.UserRepository.Save(ctx, user)
}

// countingRoles counts how often a run asks for a role.
type countingRoles struct {
	repository.RoleRepository

	asked int
}

func (c *countingRoles) GetByName(ctx context.Context, name string) (*model.Role, error) {
	c.asked++

	return c.RoleRepository.GetByName(ctx, name)
}

// countingOwned counts how often a run asks what an account would leave behind.
type countingOwned struct {
	service.OwnedTickets

	counted int
}

func (c *countingOwned) CountUnfinishedOwnedBy(ctx context.Context, userID uint) (int, error) {
	c.counted++

	return c.OwnedTickets.CountUnfinishedOwnedBy(ctx, userID)
}

// A run asks the database by what it changes, not by how large the directory is.
//
// One question for the accounts, one for each departure's tickets and one for
// each role the answer names; then a write for every arrival and every
// departure. Nothing is asked per directory entry - both indexes are built from
// the one answer and the one list of accounts - so a directory of thousands of
// whom three have left reads what a directory of ten does. That is what keeps a
// preview, which somebody opens to decide, the cost of the decision rather than
// of the directory, and it is one lookup inside the loop away from being lost:
// the arrival's role was resolved per entry once, and is kept per role since.
func TestARunReadsTheDatabaseByWhatChangesNotByTheSizeOfTheDirectory(t *testing.T) {
	for _, arrivals := range []int{20, 400} {
		t.Run(fmt.Sprintf("%d arrivals", arrivals), func(t *testing.T) {
			f := newFixture(t)

			users := &countingUsers{UserRepository: f.userRepo}
			roles := &countingRoles{RoleRepository: f.roleRepo}
			owned := &countingOwned{OwnedTickets: f.owned}
			directory := &fakeDirectory{enabled: true}
			purger := &recordingPurger{users: f.userRepo}

			sync := service.NewLDAPSyncService(directory, users, roles, owned, purger, nil, 1, model.RoleAgent)

			for _, leaver := range []string{"one@example.com", "two@example.com", "three@example.com"} {
				externalUser(t, f, leaver)
			}

			for _, stayer := range []string{"stays@example.com", "also@example.com"} {
				externalUser(t, f, stayer)

				directory.users = append(directory.users, service.ExternalUser{Email: stayer})
			}

			for i := range arrivals {
				directory.users = append(directory.users, service.ExternalUser{
					ID:    fmt.Sprintf("arrival-%d", i),
					Email: fmt.Sprintf("arrival-%d@example.com", i),
				})
			}

			report, err := sync.Sync(context.Background())
			if err != nil {
				t.Fatalf("sync: %v", err)
			}

			if len(report.Deleted) != 3 || len(report.Created) != arrivals {
				t.Fatalf("the run deleted %d and created %d (%s); want 3 and %d",
					len(report.Deleted), len(report.Created), report.Aborted, arrivals)
			}

			if users.lists != 1 || users.lookups != 0 {
				t.Errorf("the run listed the accounts %d time(s) and looked %d up one at a time; "+
					"want the list once and no account asked for by itself", users.lists, users.lookups)
			}

			if roles.asked != 1 {
				t.Errorf("the run asked for a role %d time(s) for %d arrivals that all start on the same one",
					roles.asked, arrivals)
			}

			if owned.counted != 3 {
				t.Errorf("the run asked what an account leaves behind %d time(s) for 3 departures",
					owned.counted)
			}

			if users.saves != arrivals {
				t.Errorf("the run wrote %d account(s) for %d arrivals", users.saves, arrivals)
			}
		})
	}
}
