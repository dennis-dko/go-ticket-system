package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/dennis-dko/go-ticket-system/internal/application/v1/service"
	"github.com/dennis-dko/go-ticket-system/internal/infrastructure/persistence/memory"
	"github.com/dennis-dko/go-ticket-system/internal/support/apperror"
	"github.com/dennis-dko/go-ticket-system/internal/support/webpush"
)

// pushed records what was sent, and answers with answer.
type pushed struct {
	mu     sync.Mutex
	sent   []webpush.Message
	answer error
}

func (p *pushed) send(_ context.Context, _ webpush.Keys, _ webpush.Subscription, message webpush.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.sent = append(p.sent, message)

	return p.answer
}

// A browser of the RFC 8291 example, subscribed through Google's push service.
var aBrowser = webpush.Subscription{
	Endpoint: "https://fcm.googleapis.com/fcm/send/abc",
	P256dh:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
	Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
	Origin:   "https://tickets.example.com",
}

// aMessage is something to push: what it says matters to none of these cases,
// only whether it leaves.
var aMessage = webpush.Message{Title: "A ticket was answered", Body: "Printer on fire", Tag: "gts-ticket-12"}

type pushFixture struct {
	push   *service.PushService
	subs   *memory.PushSubscriptionRepository
	pushed *pushed
}

// newPushFixture is an installation with push switched on and one account, 7,
// that has subscribed one browser.
func newPushFixture(t *testing.T) *pushFixture {
	t.Helper()

	f := &pushFixture{
		subs:   memory.NewPushSubscriptionRepository(),
		pushed: &pushed{},
	}

	f.push = service.NewPushService(newStubSettings(), f.subs).WithSender(f.pushed.send)

	if err := f.push.SetEnabled(t.Context(), true); err != nil {
		t.Fatal(err)
	}

	if err := f.push.Subscribe(t.Context(), 7, aBrowser); err != nil {
		t.Fatal(err)
	}

	return f
}

// An installation pushes nothing until it is switched on, makes its key then,
// and keeps the key when switched off, so the browsers subscribed before are
// reached again once it is back on.
func TestPushIsOffUntilSwitchedOnAndKeepsItsKey(t *testing.T) {
	push := service.NewPushService(newStubSettings(), memory.NewPushSubscriptionRepository())

	if key, err := push.PublicKey(t.Context()); err != nil || key != "" {
		t.Fatalf("an installation that has never pushed offers the key %q (%v)", key, err)
	}

	if err := push.SetEnabled(t.Context(), true); err != nil {
		t.Fatal(err)
	}

	first, err := push.PublicKey(t.Context())
	if err != nil || first == "" {
		t.Fatalf("switched on, the installation offers no key (%v)", err)
	}

	if err := push.SetEnabled(t.Context(), false); err != nil {
		t.Fatal(err)
	}

	if key, _ := push.PublicKey(t.Context()); key != "" {
		t.Errorf("switched off, the installation still offers a key to subscribe with")
	}

	if err := push.SetEnabled(t.Context(), true); err != nil {
		t.Fatal(err)
	}

	if again, _ := push.PublicKey(t.Context()); again != first {
		t.Errorf("switched on again, the key is %s rather than %s, so every browser has to subscribe anew", again, first)
	}
}

// A subscription is refused where nothing could be pushed to it: an installation
// that does not push, an address that is not a push service, keys no message can
// be encrypted for.
func TestASubscriptionNothingCouldReachIsRefused(t *testing.T) {
	f := newPushFixture(t)

	code := func(err error) string {
		var refused *apperror.Error
		if errors.As(err, &refused) {
			return refused.Code
		}

		return ""
	}

	elsewhere := aBrowser
	elsewhere.Endpoint = "https://intranet.example.com/push"

	if got := code(f.push.Subscribe(t.Context(), 7, elsewhere)); got != "pushServiceUnknown" {
		t.Errorf("an address that is no push service is answered %q", got)
	}

	broken := aBrowser
	broken.P256dh = "not a key"

	if got := code(f.push.Subscribe(t.Context(), 7, broken)); got != "pushSubscriptionInvalid" {
		t.Errorf("keys no message can be encrypted for are answered %q", got)
	}

	if err := f.push.SetEnabled(t.Context(), false); err != nil {
		t.Fatal(err)
	}

	if got := code(f.push.Subscribe(t.Context(), 7, aBrowser)); got != "pushOff" {
		t.Errorf("a subscription to an installation that does not push is answered %q", got)
	}

	if endpoints, _ := f.push.Endpoints(t.Context(), 7); len(endpoints) != 1 || endpoints[0] != aBrowser.Endpoint {
		t.Errorf("the account's browsers are %v", endpoints)
	}
}

// What is pushed to an account reaches the browsers that account subscribed,
// and nobody else's.
func TestAPushReachesTheBrowsersOfTheAccountItIsFor(t *testing.T) {
	f := newPushFixture(t)

	if err := f.push.Push(t.Context(), 8, aMessage); err != nil {
		t.Fatal(err)
	}

	if len(f.pushed.sent) != 0 {
		t.Fatalf("a push for an account with no browser was sent to somebody's: %+v", f.pushed.sent)
	}

	if err := f.push.Push(t.Context(), 7, aMessage); err != nil {
		t.Fatal(err)
	}

	if len(f.pushed.sent) != 1 || f.pushed.sent[0] != aMessage {
		t.Errorf("a push for the subscribed account sent %+v, want the message once", f.pushed.sent)
	}
}

// Switched off, the installation sends nothing, whatever was subscribed while
// it was on - and that is no failure, because a push is a courtesy beside what
// the screen already says.
func TestNothingIsPushedWhileTheInstallationDoesNotPush(t *testing.T) {
	f := newPushFixture(t)

	if err := f.push.SetEnabled(t.Context(), false); err != nil {
		t.Fatal(err)
	}

	if err := f.push.Push(t.Context(), 7, aMessage); err != nil {
		t.Errorf("a push while the installation does not push failed: %v", err)
	}

	if len(f.pushed.sent) != 0 {
		t.Errorf("pushed %+v while the installation was switched off", f.pushed.sent)
	}
}

// A browser its push service no longer knows is forgotten, rather than tried at
// every push for ever.
func TestABrowserItsPushServiceNoLongerKnowsIsForgotten(t *testing.T) {
	f := newPushFixture(t)
	f.pushed.answer = webpush.ErrGone

	if err := f.push.Push(t.Context(), 7, aMessage); err != nil {
		t.Fatal(err)
	}

	if endpoints, _ := f.push.Endpoints(t.Context(), 7); len(endpoints) != 0 {
		t.Errorf("a browser its push service said is gone is still subscribed: %v", endpoints)
	}
}

// A push service that refuses for any other reason is a failure whoever pushed
// is told about, and the browser stays subscribed: it may answer the next one.
func TestAPushThatFailsIsReportedAndKeepsTheBrowser(t *testing.T) {
	f := newPushFixture(t)
	f.pushed.answer = errors.New("the push service answered 503")

	if err := f.push.Push(t.Context(), 7, aMessage); err == nil {
		t.Error("a push the service refused was reported as sent")
	}

	if endpoints, _ := f.push.Endpoints(t.Context(), 7); len(endpoints) != 1 {
		t.Errorf("a browser was forgotten over a failure that says nothing about it: %v", endpoints)
	}
}
