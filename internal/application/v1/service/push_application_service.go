package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
	"github.com/dennis-dko/go-ticket-system/internal/domain/repository"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
	"github.com/dennis-dko/go-ticket-system/internal/support/security"
	"github.com/dennis-dko/go-ticket-system/internal/support/webpush"
)

// PushService pushes what somebody is told about their tickets to the browsers
// they subscribed, where the installation allows it at all.
//
// The installation's switch ships off: every push leaves through the push
// service of the browser's maker - Google's for Chrome, Mozilla's, Apple's - and
// whether this installation talks to them is its administrator's to decide.
// Each person then subscribes their own browsers under My account.
//
// What is pushed is decided elsewhere, by whatever has news for an account; this
// carries it. Nothing is remembered here about what was pushed.
type PushService struct {
	settings repository.SettingsRepository
	subs     repository.PushSubscriptionRepository
	secrets  *security.Sealer
	send     pushSender
}

// pushSender posts one message to one subscription.
type pushSender func(ctx context.Context, keys webpush.Keys, sub webpush.Subscription, message webpush.Message) error

// pushTTL is how long a push service may hold a message for a browser that is
// not online. Later than that it is about a moment that has passed.
const pushTTL = time.Hour

// pushEndpointLength bounds a subscription's address as push_subscriptions does.
const pushEndpointLength = 700

// NewPushService starts with a sealer that has no key, which keeps the
// installation's push key as it is; WithSecrets attaches the configured one.
func NewPushService(
	settings repository.SettingsRepository,
	subs repository.PushSubscriptionRepository,
) *PushService {
	empty, _ := security.NewSealer("")
	client := &http.Client{Timeout: 15 * time.Second}

	return &PushService{
		settings: settings, subs: subs, secrets: empty,
		send: func(ctx context.Context, keys webpush.Keys, sub webpush.Subscription, message webpush.Message) error {
			return keys.Send(ctx, client, sub, message, pushTTL)
		},
	}
}

// WithSecrets attaches the key that seals the installation's push key.
func (s *PushService) WithSecrets(secrets *security.Sealer) *PushService {
	if secrets != nil {
		s.secrets = secrets
	}

	return s
}

// WithSender delivers through send instead of posting to the push services.
func (s *PushService) WithSender(send func(ctx context.Context, keys webpush.Keys, sub webpush.Subscription, message webpush.Message) error) *PushService {
	if send != nil {
		s.send = send
	}

	return s
}

// Enabled says whether the installation pushes notifications.
func (s *PushService) Enabled(ctx context.Context) (bool, error) {
	value, err := s.settings.Get(ctx, model.SettingWebPush)

	return value == "true", err
}

// SetEnabled switches pushing on or off. On makes the installation's key the
// first time; off keeps it and every subscription, so switching back on reaches
// the same browsers again.
func (s *PushService) SetEnabled(ctx context.Context, on bool) error {
	value := ""

	if on {
		if _, _, err := s.keys(ctx, true); err != nil {
			return err
		}

		value = "true"
	}

	return s.settings.Set(ctx, model.SettingWebPush, value)
}

// PublicKey is the key a browser subscribes with, or empty where the
// installation does not push.
func (s *PushService) PublicKey(ctx context.Context) (string, error) {
	on, err := s.Enabled(ctx)
	if err != nil || !on {
		return "", err
	}

	keys, found, err := s.keys(ctx, false)
	if err != nil || !found {
		return "", err
	}

	return keys.Public()
}

// Endpoints are the addresses of somebody's subscribed browsers, so a page can
// tell whether the one it is open in is among them.
func (s *PushService) Endpoints(ctx context.Context, userID uint) ([]string, error) {
	subs, err := s.subs.ForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	endpoints := make([]string, 0, len(subs))
	for _, sub := range subs {
		endpoints = append(endpoints, sub.Endpoint)
	}

	return endpoints, nil
}

// Subscribe keeps a browser to push somebody's notifications to. It refuses where
// the installation does not push, an address that is not a push service a browser
// subscribes through, and keys no message could be encrypted for.
func (s *PushService) Subscribe(ctx context.Context, userID uint, sub webpush.Subscription) error {
	on, err := s.Enabled(ctx)
	if err != nil {
		return err
	}

	if !on {
		return apperror.Conflictf("this installation does not push notifications").WithCode("pushOff")
	}

	if len(sub.Endpoint) > pushEndpointLength || !webpush.KnownService(sub.Endpoint) {
		return apperror.Invalidf("that is not the address of a push service a browser subscribes through").
			WithCode("pushServiceUnknown")
	}

	// Tried rather than measured: keys a message cannot be encrypted for would
	// only be found out at the first push, when there is nobody to tell.
	if _, err := webpush.Encrypt(nil, sub); err != nil || len(sub.P256dh) > 100 || len(sub.Auth) > 50 || len(sub.Origin) > 255 {
		return apperror.Invalidf("these are not the keys of a browser's push channel").WithCode("pushSubscriptionInvalid")
	}

	return s.subs.Save(ctx, &model.PushSubscription{
		UserID: userID, Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth, Origin: sub.Origin,
		CreatedAt: time.Now().UTC(),
	})
}

// Unsubscribe forgets one of somebody's browsers. One that was not theirs, or
// not subscribed, is no failure: the browser is not pushed to either way.
func (s *PushService) Unsubscribe(ctx context.Context, userID uint, endpoint string) error {
	_, err := s.subs.Delete(ctx, userID, endpoint)

	return err
}

// Push sends messages to every browser an account subscribed. Where the
// installation does not push, or the account subscribed none, nothing is sent
// and that is no failure: a push is a courtesy beside what the screen already
// says. A browser its push service no longer knows is forgotten.
func (s *PushService) Push(ctx context.Context, userID uint, messages ...webpush.Message) error {
	if len(messages) == 0 {
		return nil
	}

	on, err := s.Enabled(ctx)
	if err != nil || !on {
		return err
	}

	keys, found, err := s.keys(ctx, false)
	if err != nil || !found {
		return err
	}

	return errors.Join(s.pushTo(ctx, keys, userID, messages)...)
}

// pushTo sends messages to every browser account id subscribed, forgetting one
// its push service no longer knows.
func (s *PushService) pushTo(ctx context.Context, keys webpush.Keys, id uint, messages []webpush.Message) []error {
	subs, err := s.subs.ForUser(ctx, id)
	if err != nil {
		return []error{err}
	}

	var failed []error

	for _, sub := range subs {
		to := webpush.Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth, Origin: sub.Origin}

		for _, message := range messages {
			err := s.send(ctx, keys, to, message)

			switch {
			case errors.Is(err, webpush.ErrGone):
				if forgotten := s.subs.Forget(ctx, sub.Endpoint); forgotten != nil {
					failed = append(failed, forgotten)
				}
			case err != nil:
				failed = append(failed, fmt.Errorf("pushing to a browser of account %d: %w", id, err))
			}

			if err != nil {
				break
			}
		}
	}

	return failed
}

// keys is the installation's key pair - made and kept sealed the first time it is
// wanted, where create is true - and whether there is one.
func (s *PushService) keys(ctx context.Context, create bool) (webpush.Keys, bool, error) {
	stored, err := s.settings.Get(ctx, model.SettingWebPushKey)
	if err != nil {
		return webpush.Keys{}, false, err
	}

	if stored != "" {
		private, err := s.secrets.Open(stored)
		if err != nil {
			return webpush.Keys{}, false, fmt.Errorf("opening the push key: %w", err)
		}

		keys, err := webpush.ParseKeys(private)

		return keys, err == nil, err
	}

	if !create {
		return webpush.Keys{}, false, nil
	}

	keys, err := webpush.GenerateKeys()
	if err != nil {
		return webpush.Keys{}, false, err
	}

	private, err := keys.Private()
	if err != nil {
		return webpush.Keys{}, false, err
	}

	sealed, err := s.secrets.Seal(private)
	if err != nil {
		return webpush.Keys{}, false, fmt.Errorf("sealing the push key: %w", err)
	}

	if err := s.settings.Set(ctx, model.SettingWebPushKey, sealed); err != nil {
		return webpush.Keys{}, false, err
	}

	return keys, true, nil
}
