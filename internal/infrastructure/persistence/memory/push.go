package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
)

// PushSubscriptionRepository is an in-memory repository.PushSubscriptionRepository.
type PushSubscriptionRepository struct {
	mu     sync.RWMutex
	subs   map[string]model.PushSubscription
	nextID uint
}

// NewPushSubscriptionRepository creates an empty one.
func NewPushSubscriptionRepository() *PushSubscriptionRepository {
	return &PushSubscriptionRepository{subs: make(map[string]model.PushSubscription)}
}

var _ repository.PushSubscriptionRepository = (*PushSubscriptionRepository)(nil)

func (r *PushSubscriptionRepository) Save(_ context.Context, sub *model.PushSubscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored := *sub
	if kept, found := r.subs[sub.Endpoint]; found {
		stored.ID = kept.ID
	} else {
		r.nextID++
		stored.ID = r.nextID
	}

	r.subs[sub.Endpoint] = stored
	sub.ID = stored.ID

	return nil
}

func (r *PushSubscriptionRepository) ForUser(_ context.Context, userID uint) ([]model.PushSubscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subs := make([]model.PushSubscription, 0)

	for _, sub := range r.subs {
		if sub.UserID == userID {
			subs = append(subs, sub)
		}
	}

	slices.SortFunc(subs, func(a, b model.PushSubscription) int { return int(a.ID) - int(b.ID) })

	return subs, nil
}

func (r *PushSubscriptionRepository) Subscribed(_ context.Context) ([]uint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]uint, 0)

	for _, sub := range r.subs {
		if !slices.Contains(users, sub.UserID) {
			users = append(users, sub.UserID)
		}
	}

	slices.Sort(users)

	return users, nil
}

func (r *PushSubscriptionRepository) Delete(_ context.Context, userID uint, endpoint string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sub, found := r.subs[endpoint]
	if !found || sub.UserID != userID {
		return false, nil
	}

	delete(r.subs, endpoint)

	return true, nil
}

func (r *PushSubscriptionRepository) Forget(_ context.Context, endpoint string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.subs, endpoint)

	return nil
}
