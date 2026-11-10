package service_test

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dennis-dko/go-time-recording/internal/application/v1/service"
	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/infrastructure/persistence/memory"
	"github.com/dennis-dko/go-time-recording/internal/support/apperror"
	"github.com/dennis-dko/go-time-recording/internal/support/i18n"
	"github.com/dennis-dko/go-time-recording/internal/support/webpush"
)

type pushPrincipals map[uint]*service.Principal

func (p pushPrincipals) PrincipalByID(_ context.Context, id uint) (*service.Principal, error) {
	return p[id], nil
}

type pushTimers map[uint]*model.RunningTimer

func (t pushTimers) Running(_ context.Context, id uint) (*model.RunningTimer, error) {
	return t[id], nil
}

type pushNotices map[uint]*service.Notices

func (n pushNotices) Notices(_ context.Context, id uint) (*service.Notices, error) {
	if notices, found := n[id]; found {
		return notices, nil
	}

	return &service.Notices{}, nil
}

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
	Origin:   "https://time.example.com",
}

type pushFixture struct {
	push       *service.PushService
	subs       *memory.PushSubscriptionRepository
	principals pushPrincipals
	timers     pushTimers
	notices    pushNotices
	pushed     *pushed
}

// newPushFixture is an installation with push switched on and one account, 7,
// that keeps its own time and has subscribed one browser.
func newPushFixture(t *testing.T) *pushFixture {
	t.Helper()

	f := &pushFixture{
		subs:       memory.NewPushSubscriptionRepository(),
		principals: pushPrincipals{},
		timers:     pushTimers{},
		notices:    pushNotices{},
		pushed:     &pushed{},
	}

	f.principals[7] = &service.Principal{
		User:        &model.User{ID: 7, Language: "en", Timezone: "Europe/Berlin"},
		Permissions: []string{model.PermTimesheetReadOwn, model.PermTimesheetWriteOwn},
	}

	f.push = service.NewPushService(newStubSettings(), f.subs, f.principals, f.timers, f.notices).WithSender(f.pushed.send)

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
	push := service.NewPushService(newStubSettings(), memory.NewPushSubscriptionRepository(),
		pushPrincipals{}, pushTimers{}, pushNotices{})

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

// A stopwatch is pushed in the minute its hours pass, and in no other.
func TestAStopwatchIsPushedInTheMinuteItsHoursPass(t *testing.T) {
	f := newPushFixture(t)

	hours := 2
	f.principals[7].User.TimerAlertHours = &hours

	started := time.Date(2026, 11, 10, 7, 15, 0, 0, time.UTC)
	f.timers[7] = &model.RunningTimer{UserID: 7, StartedAt: started}

	for _, at := range []time.Time{started.Add(2*time.Hour - time.Minute), started.Add(2*time.Hour + time.Minute)} {
		if err := f.push.PushDue(t.Context(), at); err != nil {
			t.Fatal(err)
		}
	}

	if len(f.pushed.sent) != 0 {
		t.Fatalf("pushed %v outside the minute the hours passed in", f.pushed.sent)
	}

	if err := f.push.PushDue(t.Context(), started.Add(2*time.Hour+30*time.Second)); err != nil {
		t.Fatal(err)
	}

	if len(f.pushed.sent) != 1 || f.pushed.sent[0].Title != "The stopwatch is still running" ||
		f.pushed.sent[0].Tag != "gtr-stopwatch" || !strings.Contains(f.pushed.sent[0].Body, "2 h") {
		t.Errorf("in the minute the hours passed, pushed %+v", f.pushed.sent)
	}
}

// An unbooked day is pushed at eight in the morning on its owner's clock, and
// not at eight anywhere else.
func TestAnUnbookedDayIsPushedAtEightOnTheOwnersClock(t *testing.T) {
	f := newPushFixture(t)

	day := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	f.notices[7] = &service.Notices{UnbookedDay: &day}

	for _, at := range []time.Time{
		time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 10, 6, 59, 0, 0, time.UTC),
		time.Date(2026, 11, 10, 7, 1, 0, 0, time.UTC),
	} {
		if err := f.push.PushDue(t.Context(), at); err != nil {
			t.Fatal(err)
		}
	}

	if len(f.pushed.sent) != 0 {
		t.Fatalf("pushed %v at a time that is not eight in Berlin", f.pushed.sent)
	}

	if err := f.push.PushDue(t.Context(), time.Date(2026, 11, 10, 7, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	if len(f.pushed.sent) != 1 || f.pushed.sent[0].Tag != "gtr-unbooked" {
		t.Errorf("at eight in Berlin, pushed %+v", f.pushed.sent)
	}
}

// An account whose role no longer reads its own time is pushed nothing, as the
// page shows it no banner - whatever it subscribed while it could.
func TestNothingIsPushedToAnAccountThatNoLongerReadsItsOwnTime(t *testing.T) {
	f := newPushFixture(t)

	started := time.Date(2026, 11, 10, 7, 15, 0, 0, time.UTC)
	f.timers[7] = &model.RunningTimer{UserID: 7, StartedAt: started}
	f.principals[7].Permissions = nil

	if err := f.push.PushDue(t.Context(), started.Add(time.Duration(model.DefaultTimerAlertHours)*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if len(f.pushed.sent) != 0 {
		t.Errorf("pushed %v to an account without the right to read its own time", f.pushed.sent)
	}
}

// A browser its push service no longer knows is forgotten, rather than tried at
// every reminder for ever.
func TestABrowserItsPushServiceNoLongerKnowsIsForgotten(t *testing.T) {
	f := newPushFixture(t)
	f.pushed.answer = webpush.ErrGone

	started := time.Date(2026, 11, 10, 7, 15, 0, 0, time.UTC)
	f.timers[7] = &model.RunningTimer{UserID: 7, StartedAt: started}

	if err := f.push.PushDue(t.Context(), started.Add(time.Duration(model.DefaultTimerAlertHours)*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if endpoints, _ := f.push.Endpoints(t.Context(), 7); len(endpoints) != 0 {
		t.Errorf("a browser its push service said is gone is still subscribed: %v", endpoints)
	}
}

// pageEnglish is what the page says in English under key: the literals it hands
// t(), joined where the sentence is written over several, or the markup's own
// text for a key the markup declares.
func pageEnglish(t *testing.T, key string) string {
	t.Helper()

	script, err := os.ReadFile("../../../interface/web/asset/app.js")
	if err != nil {
		t.Fatal(err)
	}

	call := regexp.MustCompile(`\bt\(\s*'` + regexp.QuoteMeta(key) + `',\s*((?:'(?:[^'\\]|\\.)*'\s*\+?\s*)+)`).FindSubmatch(script)
	if call != nil {
		var english strings.Builder

		for _, part := range regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`).FindAllSubmatch(call[1], -1) {
			english.WriteString(strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(string(part[1])))
		}

		return english.String()
	}

	markup, err := os.ReadFile("../../../interface/web/asset/index.html")
	if err != nil {
		t.Fatal(err)
	}

	declared := regexp.MustCompile(`data-i18n="` + regexp.QuoteMeta(key) + `"[^>]*>([^<]*)<`).FindSubmatch(markup)
	if declared == nil {
		t.Fatalf("the page says nothing under %s", key)
	}

	return strings.TrimSpace(string(declared[1]))
}

// A push says what the page's banner and notification say, in the page's own
// English: borrowed rather than copied, so the two cannot come to say the same
// thing two ways.
func TestThePushSaysThePagesWordsInItsEnglish(t *testing.T) {
	f := newPushFixture(t)

	hours := 2
	f.principals[7].User.TimerAlertHours = &hours

	started := time.Date(2026, 11, 10, 7, 0, 0, 0, time.UTC)
	f.timers[7] = &model.RunningTimer{UserID: 7, StartedAt: started}

	day := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	f.notices[7] = &service.Notices{UnbookedDay: &day}

	// Nine in UTC is ten in Berlin: the stopwatch's hours pass then, and the
	// unbooked day is pushed an hour before.
	for _, at := range []time.Time{started, started.Add(2 * time.Hour)} {
		if err := f.push.PushDue(t.Context(), at); err != nil {
			t.Fatal(err)
		}
	}

	words := i18n.For("en")
	berlin, _ := time.LoadLocation("Europe/Berlin")

	want := []webpush.Message{
		{
			Title: pageEnglish(t, "remind.title"),
			Body:  i18n.Fill(pageEnglish(t, "remind.unbooked"), words.Day(day), pageEnglish(t, "nav.timesheets")),
			Tag:   "gtr-unbooked",
		},
		{
			Title: pageEnglish(t, "timer.longTitle"),
			Body: i18n.Fill(pageEnglish(t, "timer.longBody"), "2 "+pageEnglish(t, "unit.hours"),
				words.Moment(started.In(berlin))),
			Tag: "gtr-stopwatch",
		},
	}

	if len(f.pushed.sent) != len(want) {
		t.Fatalf("pushed %+v", f.pushed.sent)
	}

	for i := range want {
		if f.pushed.sent[i] != want[i] {
			t.Errorf("pushed\n%+v\nwhere the page says\n%+v", f.pushed.sent[i], want[i])
		}
	}
}
