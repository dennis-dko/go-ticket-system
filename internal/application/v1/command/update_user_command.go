package command

import "github.com/dennis-dko/go-ticket-system/internal/application/v1/common"

// UpdateUserCommand command to update existing user
type UpdateUserCommand struct {
	ID    uint
	Name  *string
	Email *string

	// Role is the role name to move the user to.
	Role *string
}

// UpdateUserCommandResult command to get update result of existing user
type UpdateUserCommandResult struct {
	Result *common.UserResult
}
