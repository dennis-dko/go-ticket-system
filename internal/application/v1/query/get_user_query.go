package query

import "github.com/dennis-dko/go-ticket-system/internal/application/v1/common"

// GetUserQuery query to get existing user
type GetUserQuery struct {
	ID uint
}

// GetUserQueryResult query to get data result of existing user
type GetUserQueryResult struct {
	Result *common.UserResult
}
