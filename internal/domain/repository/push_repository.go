package repository

import (
	"context"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// PushSubscriptionRepository keeps the browsers people asked for the reminders
// to be pushed to.
type PushSubscriptionRepository interface {
	// Save keeps a subscription in place of any with the same endpoint, whoever's
	// it was: the browser it belongs to is the account's that subscribed it last.
	Save(ctx context.Context, sub *model.PushSubscription) error

	// ForUser returns somebody's subscriptions, one a browser.
	ForUser(ctx context.Context, userID uint) ([]model.PushSubscription, error)

	// Subscribed returns everybody with a subscription: whom the reminders are
	// pushed to at all.
	Subscribed(ctx context.Context) ([]uint, error)

	// Delete forgets one of somebody's subscriptions by its endpoint, and reports
	// whether there was one.
	Delete(ctx context.Context, userID uint, endpoint string) (bool, error)

	// Forget drops a subscription by its endpoint, whoever's it is: its push
	// service said it no longer exists.
	Forget(ctx context.Context, endpoint string) error
}
