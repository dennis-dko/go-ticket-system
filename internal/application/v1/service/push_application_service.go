package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/domain/repository"
	"github.com/dennis-dko/go-time-recording/internal/support/apperror"
	"github.com/dennis-dko/go-time-recording/internal/support/i18n"
	"github.com/dennis-dko/go-time-recording/internal/support/security"
	"github.com/dennis-dko/go-time-recording/internal/support/webpush"
)

// PushService pushes the reminders to the browsers people subscribe - a
// stopwatch run past the hours its owner chose, and the last working day with
// nothing booked on it - where the installation allows it at all.
//
// The installation's switch ships off, decided by the maintainer on 2026-11-10:
// every push leaves through the push service of the browser's maker - Google's
// for Chrome, Mozilla's, Apple's - and whether this installation talks to them
// is its administrator's to decide. Each person then subscribes their own
// browsers from the Reminders card.
//
// The reminders are worked out as the page's banners are, from the same answers,
// in the minute they fall due. Nothing is remembered about what was pushed: each
// is pushed by the run for the minute it falls in, so a minute nobody ran - a
// restart - is a reminder not pushed rather than one pushed twice.
type PushService struct {
	settings   repository.SettingsRepository
	subs       repository.PushSubscriptionRepository
	principals principalReader
	timers     timerReader
	notices    noticeReader
	secrets    *security.Sealer
	send       pushSender
}

// principalReader is who an account is and what it may do, asked afresh on
// every run: a role changed since a browser was subscribed decides.
type principalReader interface {
	PrincipalByID(ctx context.Context, id uint) (*Principal, error)
}

// timerReader is somebody's running stopwatch, or nil.
type timerReader interface {
	Running(ctx context.Context, userID uint) (*model.RunningTimer, error)
}

// noticeReader is what about somebody's own time deserves a banner.
type noticeReader interface {
	Notices(ctx context.Context, userID uint) (*Notices, error)
}

// pushSender posts one message to one subscription.
type pushSender func(ctx context.Context, keys webpush.Keys, sub webpush.Subscription, message webpush.Message) error

// pushMorning is the hour on somebody's own clock at which an unbooked day is
// pushed: the installation names yesterday from midnight on, and nobody is to be
// woken for it.
const pushMorning = 8

// pushTTL is how long a push service may hold a reminder for a browser that is
// not online. Later than that it is about a moment that has passed.
const pushTTL = time.Hour

// pushEndpointLength bounds a subscription's address as push_subscriptions does.
const pushEndpointLength = 700

// NewPushService starts with a sealer that has no key, which keeps the
// installation's push key as it is; WithSecrets attaches the configured one.
func NewPushService(
	settings repository.SettingsRepository,
	subs repository.PushSubscriptionRepository,
	principals principalReader,
	timers timerReader,
	notices noticeReader,
) *PushService {
	empty, _ := security.NewSealer("")
	client := &http.Client{Timeout: 15 * time.Second}

	return &PushService{
		settings: settings, subs: subs, principals: principals, timers: timers, notices: notices, secrets: empty,
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

// Enabled says whether the installation pushes reminders.
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

// Subscribe keeps a browser to push somebody's reminders to. It refuses where
// the installation does not push, an address that is not a push service a browser
// subscribes through, and keys no message could be encrypted for.
func (s *PushService) Subscribe(ctx context.Context, userID uint, sub webpush.Subscription) error {
	on, err := s.Enabled(ctx)
	if err != nil {
		return err
	}

	if !on {
		return apperror.Conflictf("this installation does not push reminders").WithCode("pushOff")
	}

	if len(sub.Endpoint) > pushEndpointLength || !webpush.KnownService(sub.Endpoint) {
		return apperror.Invalidf("that is not the address of a push service a browser subscribes through").
			WithCode("pushServiceUnknown")
	}

	// Tried rather than measured: keys a message cannot be encrypted for would
	// only be found out at the first reminder, when there is nobody to tell.
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

// PushDue pushes what falls due in the minute that ends at end - which it is
// truncated to - to every subscribed browser of the people it falls due for: a
// stopwatch whose hours pass in that minute, and an unbooked day for whoever's
// own clock reads eight in the morning at its end. A browser its push service no
// longer knows is forgotten.
func (s *PushService) PushDue(ctx context.Context, end time.Time) error {
	on, err := s.Enabled(ctx)
	if err != nil || !on {
		return err
	}

	keys, found, err := s.keys(ctx, false)
	if err != nil || !found {
		return err
	}

	zone, err := s.settings.Get(ctx, model.SettingTimezone)
	if err != nil {
		return err
	}

	users, err := s.subs.Subscribed(ctx)
	if err != nil {
		return err
	}

	end = end.Truncate(time.Minute)

	var failed []error

	for _, id := range users {
		messages, err := s.dueFor(ctx, id, end, zone)
		if err != nil {
			failed = append(failed, fmt.Errorf("working out the reminders of account %d: %w", id, err))

			continue
		}

		if len(messages) == 0 {
			continue
		}

		failed = append(failed, s.pushTo(ctx, keys, id, messages)...)
	}

	return errors.Join(failed...)
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

// dueFor is what falls due for one account in the minute ending at end, in its
// own language: nothing for an account whose role no longer reads its own time,
// as the page's banners are not shown to one.
func (s *PushService) dueFor(ctx context.Context, userID uint, end time.Time, instanceZone string) ([]webpush.Message, error) {
	principal, err := s.principals.PrincipalByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if principal == nil || principal.User == nil || !principal.Can(model.PermTimesheetReadOwn) {
		return nil, nil
	}

	user := principal.User
	words := i18n.For(user.EffectiveLanguage())
	zone := model.EffectiveTimezone(user.Timezone, instanceZone)

	var due []webpush.Message

	if hours := user.EffectiveTimerAlertHours(); hours > 0 && principal.Can(model.PermTimesheetWriteOwn) {
		timer, err := s.timers.Running(ctx, userID)
		if err != nil {
			return nil, err
		}

		if timer != nil {
			at := timer.StartedAt.Add(time.Duration(hours) * time.Hour)

			if at.After(end.Add(-time.Minute)) && !at.After(end) {
				due = append(due, webpush.Message{
					Title: words.Text("timer.longTitle", "The stopwatch is still running"),
					Body: words.Text("timer.longBody", "Since {1}, longer than the {0} you chose.",
						strconv.Itoa(hours)+" "+words.Text("unit.hours", "h"), words.Moment(timer.StartedAt.In(zone))),
					Tag:   "gtr-stopwatch",
					At:    at.UTC().Format(time.RFC3339),
					About: timer.StartedAt.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	if local := end.In(zone); local.Hour() == pushMorning && local.Minute() == 0 {
		notices, err := s.notices.Notices(ctx, userID)
		if err != nil {
			return nil, err
		}

		if notices != nil && notices.UnbookedDay != nil {
			due = append(due, webpush.Message{
				Title: words.Text("remind.title", "Reminders"),
				Body: words.Text("remind.unbooked",
					"Nothing is booked on {0}, a working day by your plan. It can be added under {1}.",
					words.Day(*notices.UnbookedDay), words.Text("nav.timesheets", "Time entries")),
				Tag:   "gtr-unbooked",
				At:    end.UTC().Format(time.RFC3339),
				About: notices.UnbookedDay.Format(time.DateOnly),
			})
		}
	}

	return due, nil
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
