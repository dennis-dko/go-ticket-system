package sqldb

import (
	"context"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
)

const pushSubscriptionColumns = "id, user_id, endpoint, p256dh, auth, origin, created_at"

// PushSubscriptionRepository keeps the browsers the reminders are pushed to in a
// SQL database.
type PushSubscriptionRepository struct {
	base
}

// NewPushSubscriptionRepository creates a push subscription repository for the given dialect.
func NewPushSubscriptionRepository(db DB, dialect string) *PushSubscriptionRepository {
	return &PushSubscriptionRepository{base{db: db, dialect: dialect}}
}

var _ repository.PushSubscriptionRepository = (*PushSubscriptionRepository)(nil)

// Save keeps the subscription over any with its endpoint - one statement in each
// engine's own words - and hands it to whoever subscribed it last.
func (r *PushSubscriptionRepository) Save(ctx context.Context, sub *model.PushSubscription) error {
	query := `INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, origin, created_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (endpoint) DO UPDATE SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth, origin = EXCLUDED.origin, created_at = EXCLUDED.created_at`

	if r.dialect == DialectMySQL {
		query = `INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, origin, created_at) VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE user_id = VALUES(user_id), p256dh = VALUES(p256dh),
				auth = VALUES(auth), origin = VALUES(origin), created_at = VALUES(created_at)`
	}

	if _, err := r.exec(ctx, query, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth, sub.Origin, sub.CreatedAt); err != nil {
		return apperror.Internal(err)
	}

	return nil
}

func (r *PushSubscriptionRepository) ForUser(ctx context.Context, userID uint) ([]model.PushSubscription, error) {
	rows, err := r.db.QueryContext(ctx,
		r.rebind("SELECT "+pushSubscriptionColumns+" FROM push_subscriptions WHERE user_id = ? ORDER BY id"), userID)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	defer func() { _ = rows.Close() }()

	subs := make([]model.PushSubscription, 0)

	for rows.Next() {
		var (
			sub       model.PushSubscription
			createdAt dateTime
		)

		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth, &sub.Origin, &createdAt); err != nil {
			return nil, apperror.Internal(err)
		}

		sub.CreatedAt = createdAt.Time
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, apperror.Internal(err)
	}

	return subs, nil
}

func (r *PushSubscriptionRepository) Delete(ctx context.Context, userID uint, endpoint string) (bool, error) {
	affected, err := r.exec(ctx, "DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?", userID, endpoint)
	if err != nil {
		return false, apperror.Internal(err)
	}

	return affected > 0, nil
}

func (r *PushSubscriptionRepository) Forget(ctx context.Context, endpoint string) error {
	if _, err := r.exec(ctx, "DELETE FROM push_subscriptions WHERE endpoint = ?", endpoint); err != nil {
		return apperror.Internal(err)
	}

	return nil
}
