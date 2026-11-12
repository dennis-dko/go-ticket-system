//go:build browser

package browser

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/serviceworker"
	"github.com/chromedp/chromedp"
)

// A reminder pushed to a browser that was closed is held by the push service and
// handed over when the browser is started again, up to an hour later - by when
// the stopwatch it is about may have been stopped and the day booked. So one that
// arrives late is shown only once the installation has confirmed it, as the page
// and the app confirm what they show, and one that no longer holds takes back the
// notification shown for it before; one that arrives on time was confirmed the
// moment it was sent, and is shown as it is.
//
// What is not shown is seen as the earlier notification going away, so a push
// that never arrived fails here rather than passing for one the worker withheld.
func TestAReminderPushedLateIsShownOnlyWhileItStillHolds(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.run("allow notifications from the installation",
		browser.SetPermission(&browser.PermissionDescriptor{Name: "notifications"}, browser.PermissionSettingGranted).
			WithOrigin(p.app.BaseURL()))

	// The leave is the browser's to apply, and a push the worker could not show
	// for want of it would look exactly like one it chose not to show.
	p.waitEvaluates("the page's leave to notify", `Notification.permission`, "granted")

	push := p.pusher()
	onTime := time.Now().UTC().Format(time.RFC3339)
	late := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)

	push.until("on time, about no stopwatch", "gts-stopwatch", "on time", onTime, "2026-01-05T08:00:00Z")
	push.once("late, about a stopwatch since stopped", "gts-stopwatch", "stopped", late, "2026-01-05T08:00:00Z")
	p.waitForNoNotification("gts-stopwatch", "a late reminder of a stopwatch that is not running")

	// The stopwatch as the installation reports it, and as the push names it: in
	// UTC, whatever offset the report writes it with.
	var reported string

	p.awaitJSON(`api('/me/timer', { method: 'POST', body: '{}' }).then(() => api('/me/timer')).then((timer) => timer.startedAt)`, &reported)

	started, err := time.Parse(time.RFC3339Nano, reported)
	if err != nil {
		t.Fatalf("the stopwatch reports starting at %q: %v", reported, err)
	}

	push.once("late, about the stopwatch running", "gts-stopwatch", "running", late, started.UTC().Format(time.RFC3339))
	p.waitForNotification("gts-stopwatch", "running")

	push.until("on time, about a day", "gts-unbooked", "unbooked", onTime, "2026-01-02")
	push.once("late, about a day since booked", "gts-unbooked", "booked", late, "2000-01-03")
	p.waitForNoNotification("gts-unbooked", "a late reminder of a day that is no longer unbooked")
}

// pusher delivers pushes to the page's service worker, as the browser's own tools
// do.
type pusher struct {
	p            *page
	origin       string
	registration serviceworker.RegistrationID
}

func (p *page) pusher() pusher {
	p.t.Helper()

	return pusher{p: p, origin: strings.TrimSuffix(p.app.BaseURL(), "/"), registration: p.serviceWorkerRegistration()}
}

// once pushes a reminder one time.
func (s pusher) once(what, tag, title, at, about string) {
	s.p.t.Helper()

	data, err := json.Marshal(map[string]string{"title": title, "body": what, "tag": tag, "at": at, "about": about})
	if err != nil {
		s.p.t.Fatal(err)
	}

	s.p.run("push "+what, serviceworker.DeliverPushMessage(s.origin, s.registration, string(data)))
}

// until pushes a reminder that is shown without asking until it is shown. A
// browser now and then loses the first push its tools deliver to a new profile -
// measured: one run in three, while the second always arrived - and this is what
// makes sure the worker is reached before anything is concluded from its silence.
func (s pusher) until(what, tag, title, at, about string) {
	s.p.t.Helper()

	for deadline := time.Now().Add(waitPatience); time.Now().Before(deadline); {
		s.once(what, tag, title, at, about)

		for wait := time.Now().Add(3 * time.Second); time.Now().Before(wait); time.Sleep(200 * time.Millisecond) {
			for _, shown := range s.p.notifications(tag) {
				if shown == title {
					return
				}
			}
		}
	}

	s.p.t.Fatalf("pushed %q again and again, and the service worker never showed it", title)
}

// serviceWorkerRegistration is the registration of the page's service worker, as
// the browser's own tools name it to deliver a push to - once a version of it is
// activated, because a push delivered to one still installing is lost.
func (p *page) serviceWorkerRegistration() serviceworker.RegistrationID {
	p.t.Helper()

	p.awaitJSON(`navigator.serviceWorker.ready.then(() => true)`, nil)

	scope := strings.TrimSuffix(p.app.BaseURL(), "/") + "/"

	// The listener runs on the browser's event loop and outlives this call, so it
	// only records and never waits.
	var (
		mu        sync.Mutex
		ours      serviceworker.RegistrationID
		activated = map[serviceworker.RegistrationID]bool{}
		changed   = make(chan struct{}, 1)
	)

	chromedp.ListenTarget(p.ctx, func(event any) {
		mu.Lock()

		switch updated := event.(type) {
		case *serviceworker.EventWorkerRegistrationUpdated:
			for _, registration := range updated.Registrations {
				if registration.ScopeURL == scope && !registration.IsDeleted {
					ours = registration.RegistrationID
				}
			}
		case *serviceworker.EventWorkerVersionUpdated:
			for _, version := range updated.Versions {
				activated[version.RegistrationID] = version.Status == serviceworker.VersionStatusActivated
			}
		}

		mu.Unlock()

		select {
		case changed <- struct{}{}:
		default:
		}
	})

	p.run("watch the service workers", serviceworker.Enable())

	deadline := time.After(waitPatience)

	for {
		mu.Lock()
		id, ready := ours, ours != "" && activated[ours]
		mu.Unlock()

		if ready {
			return id
		}

		select {
		case <-changed:
		case <-deadline:
			p.t.Fatalf("no activated service worker is registered for %s", scope)
		}
	}
}

// notifications are the titles of what the page's service worker shows under tag.
func (p *page) notifications(tag string) []string {
	p.t.Helper()

	var titles []string

	p.awaitJSON(`navigator.serviceWorker.ready
		.then((registration) => registration.getNotifications({ tag: '`+tag+`' }))
		.then((shown) => shown.map((notification) => notification.title))`, &titles)

	return titles
}

// waitForNotification waits until the service worker shows title under tag.
func (p *page) waitForNotification(tag, title string) {
	p.t.Helper()

	var shown []string

	for deadline := time.Now().Add(waitPatience); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		shown = p.notifications(tag)

		for _, got := range shown {
			if got == title {
				return
			}
		}
	}

	p.t.Fatalf("the service worker shows %q under %s, not %q", shown, tag, title)
}

// waitForNoNotification waits until the service worker shows nothing under tag,
// and says what it shows instead where that does not happen.
func (p *page) waitForNoNotification(tag, what string) {
	p.t.Helper()

	var shown []string

	for deadline := time.Now().Add(waitPatience); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if shown = p.notifications(tag); len(shown) == 0 {
			return
		}
	}

	p.t.Errorf("%s is not taken back: the service worker shows %q", what, shown)
}
