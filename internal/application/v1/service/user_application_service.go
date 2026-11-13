package service

import (
	"context"
	"strings"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/command"
	"github.com/dennis-dko/go-ticket-system/internal/application/v1/common"
	"github.com/dennis-dko/go-ticket-system/internal/application/v1/query"
	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
	"github.com/dennis-dko/go-ticket-system/internal/support/security"
)

// UserService administers accounts. Deleting one leaves the tickets it owned
// without an owner.
type UserService interface {
	CreateUser(ctx context.Context, cmd command.CreateUserCommand) (*command.CreateUserCommandResult, error)
	GetUser(ctx context.Context, q query.GetUserQuery) (*query.GetUserQueryResult, error)
	ListUsers(ctx context.Context, q query.ListUsersQuery) (*query.ListUsersQueryResult, error)
	UpdateUser(ctx context.Context, cmd command.UpdateUserCommand) (*command.UpdateUserCommandResult, error)
	DeleteUser(ctx context.Context, cmd command.DeleteUserCommand) error
}

// OwnedTickets answers what an account would leave behind: how many tickets it
// owns that somebody still has to answer.
//
// Asked before an account is removed, by an administrator's deletion and by the
// directory synchronisation alike, so both say the same number about the same
// account.
type OwnedTickets interface {
	CountUnfinishedOwnedBy(ctx context.Context, userID uint) (int, error)
}

// UserApplicationService is the UserService the application runs with. An
// account the directory owns keeps the name and the address the directory
// gives it.
type UserApplicationService struct {
	userRepository repository.UserRepository
	roleRepository repository.RoleRepository

	// owned answers how many unfinished tickets an account owns, which is what
	// a deletion has to warn about.
	owned OwnedTickets

	// purger removes an account together with everything that is its own.
	//
	// Deleting only the account row is not an option: the schema declares
	// foreign keys, so PostgreSQL refuses it outright, and SQLite and MySQL -
	// neither of which enforces them - accept it and leave tickets, sessions and
	// tokens behind pointing at nobody. Same request, two different wrong
	// answers, depending on a choice made in the installer.
	purger UserPurger
}

// NewUserApplicationService needs the tickets an account owns to say what
// deleting it would leave behind, and the purger to remove it in one go.
func NewUserApplicationService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	owned OwnedTickets,
	purger UserPurger,
) *UserApplicationService {
	return &UserApplicationService{
		userRepository: userRepo,
		roleRepository: roleRepo,
		owned:          owned,
		purger:         purger,
	}
}

var _ UserService = (*UserApplicationService)(nil)

// CreateUser processes the command to create a user
func (s *UserApplicationService) CreateUser(
	ctx context.Context,
	cmd command.CreateUserCommand,
) (*command.CreateUserCommandResult, error) {
	if err := validateUser(cmd.Name, cmd.Email); err != nil {
		return nil, err
	}

	role, err := s.resolveRole(ctx, cmd.Role)
	if err != nil {
		return nil, err
	}

	// A user with no password cannot sign in, so a new account either gets the
	// password it was given or one that must be replaced on first use.
	password := cmd.Password
	mustChange := false

	if password == "" {
		password = SystemUserPassword
		mustChange = true
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, passwordError(err)
	}

	createdUser, err := s.userRepository.Save(ctx, &model.User{
		Name:               cmd.Name,
		Email:              normalizeEmail(cmd.Email),
		RoleID:             role.ID,
		PasswordHash:       hash,
		MustChangePassword: mustChange,
	})
	if err != nil {
		return nil, err
	}

	return &command.CreateUserCommandResult{
		Result: common.NewUserResultFromModel(createdUser)[0],
	}, nil
}

// GetUser processes the query to get a user
func (s *UserApplicationService) GetUser(
	ctx context.Context,
	q query.GetUserQuery,
) (*query.GetUserQueryResult, error) {
	if q.ID == 0 {
		return nil, apperror.InvalidFields("id")
	}

	user, err := s.userRepository.GetByID(ctx, q.ID)
	if err != nil {
		return nil, err
	}

	return &query.GetUserQueryResult{
		Result: common.NewUserResultFromModel(user)[0],
	}, nil
}

// ListUsers processes the query to get all users, applying optional paging.
func (s *UserApplicationService) ListUsers(
	ctx context.Context,
	q query.ListUsersQuery,
) (*query.ListUsersQueryResult, error) {
	allUsers, err := s.userRepository.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// TotalCount reports the unpaged total so a client can size its pager.
	total := uint(len(allUsers))

	return &query.ListUsersQueryResult{
		Result:     common.NewUserResultFromModel(paginate(allUsers, q.Page, q.Limit)...),
		TotalCount: total,
	}, nil
}

// UpdateUser processes the command to update a user
func (s *UserApplicationService) UpdateUser(
	ctx context.Context,
	cmd command.UpdateUserCommand,
) (*command.UpdateUserCommandResult, error) {
	if cmd.ID == 0 {
		return nil, apperror.InvalidFields("id")
	}

	existingUser, err := s.userRepository.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	// An account the directory owns is edited in the directory, with one
	// exception.
	//
	// The name and the address are copied from the entry on every
	// synchronisation, so changing them here lasts until the next run and then
	// silently reverts - which is worse than being refused, because it looks like
	// it worked. The role is the exception because the directory has no opinion
	// about it: it is decided here, for a person the directory only says exists.
	if existingUser.IsExternal {
		if cmd.Name != nil && *cmd.Name != existingUser.Name {
			return nil, apperror.Conflictf(
				"%q comes from the directory; the name is kept in step with the entry "+
					"there and changing it here would last until the next "+
					"synchronisation", existingUser.Email).
				WithCode("directoryAccountReadOnly", existingUser.Email)
		}

		if cmd.Email != nil && normalizeEmail(*cmd.Email) != existingUser.Email {
			return nil, apperror.Conflictf(
				"%q comes from the directory; the address is what the synchronisation "+
					"matches on and is not changed here", existingUser.Email).
				WithCode("directoryAccountReadOnly", existingUser.Email)
		}
	}

	if cmd.Name != nil {
		existingUser.Name = *cmd.Name
	}

	if cmd.Email != nil {
		existingUser.Email = normalizeEmail(*cmd.Email)
	}

	if cmd.Role != nil {
		role, roleErr := s.resolveRole(ctx, *cmd.Role)
		if roleErr != nil {
			return nil, roleErr
		}

		// Same protection as the domain service: the built-in administrator
		// must keep a role that can still administer.
		if existingUser.IsSystem && !role.Has(model.PermRoleWrite) {
			return nil, apperror.Conflictf(
				"the built-in administrator cannot be moved to role %q, which lacks %q",
				role.Name, model.PermRoleWrite).
				WithCode("adminRoleMustAdminister", role.Name, model.PermRoleWrite)
		}

		existingUser.RoleID = role.ID
	}

	if err := validateUser(existingUser.Name, existingUser.Email); err != nil {
		return nil, err
	}

	updatedUser, err := s.userRepository.Update(ctx, existingUser)
	if err != nil {
		return nil, err
	}

	return &command.UpdateUserCommandResult{
		Result: common.NewUserResultFromModel(updatedUser)[0],
	}, nil
}

// DeleteUser processes the command to delete a user
func (s *UserApplicationService) DeleteUser(ctx context.Context, cmd command.DeleteUserCommand) error {
	if cmd.ID == 0 {
		return apperror.InvalidFields("id")
	}

	user, err := s.userRepository.GetByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	// Deleting the built-in administrator would leave an installation with no
	// guaranteed way back in.
	if user.IsSystem {
		return apperror.Conflictf("the built-in administrator cannot be deleted").
			WithCode("adminUndeletable")
	}

	// An account the directory owns is removed by removing the entry there.
	//
	// Deleting it here removes a person, and the next synchronisation creates the
	// account again from the entry that is still in the directory - as a new
	// account, which owns none of the tickets the old one did. So the tickets
	// have lost their owner, the person is back, and nothing about the directory
	// has changed. Whoever meant to remove this person did not mean that.
	if user.IsExternal {
		return apperror.Conflictf(
			"%q comes from the directory; remove the entry there and the next "+
				"synchronisation removes this account", user.Email).
			WithCode("directoryAccountUndeletable", user.Email)
	}

	// How many tickets are about to be left without an owner. Counted rather
	// than loaded: the refusal needs the number and nothing else.
	unfinished, err := s.owned.CountUnfinishedOwnedBy(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if unfinished > 0 && !cmd.Confirmed {
		return apperror.Conflictf(
			"%q owns %d tickets that are not finished, which would be left without an "+
				"owner; confirm to proceed",
			user.Email, unfinished).
			WithCode("deletionNeedsConfirming", user.Email, unfinished)
	}

	return s.purger.PurgeUser(ctx, cmd.ID)
}

// resolveRole accepts a role name, defaulting to the least privileged role of
// the staff when none is given.
func (s *UserApplicationService) resolveRole(ctx context.Context, name string) (*model.Role, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = model.RoleAgent
	}

	role, err := s.roleRepository.GetByName(ctx, name)
	if err != nil {
		return nil, apperror.InvalidFields("role")
	}

	return role, nil
}

func validateUser(name, email string) error {
	var invalid []string

	// Empty and too long are both "not a name", and both have to be caught
	// here: the column enforces the second only on PostgreSQL and MySQL, so on
	// SQLite it is stored and everywhere else it is a driver error the caller
	// reads as a broken application.
	if strings.TrimSpace(name) == "" || model.TooLong(name, model.MaxNameLength) {
		invalid = append(invalid, "name")
	}

	if !validEmail(email) || model.TooLong(email, model.MaxEmailLength) {
		invalid = append(invalid, "email")
	}

	if len(invalid) > 0 {
		return apperror.InvalidFields(invalid...)
	}

	return nil
}
