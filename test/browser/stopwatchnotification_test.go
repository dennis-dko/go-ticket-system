//go:build browser

package browser

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
)

// A stopwatch that crosses the hours its owner chose while the page is open is
// shown as a notification through the page's service worker, which Chrome on a
// phone requires: there the page's own Notification constructor is refused, and
// only the banner was left. The notification belongs to the worker's registration,
// which is what tells the two ways apart - one the page makes itself is not among
// the registration's notifications.
func TestALongStopwatchIsShownThroughTheServiceWorker(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.run("allow notifications from the installation",
		browser.SetPermission(&browser.PermissionDescriptor{Name: "notifications"}, browser.PermissionSettingGranted).
			WithOrigin(p.app.BaseURL()))

	// Mentioned after an hour, and started an hour less a few seconds ago, so the
	// clock crosses its hours while the page is open rather than before it loads -
	// a stopwatch found already past them is left to the banner.
	p.awaitJSON(`(async () => {
		await api('/me/reminders', { method: 'PUT', body: JSON.stringify({ unbookedDay: false, timerAlertHours: 1 }) });
		await api('/me/timer', { method: 'POST', body: '{}' });
		const startedAt = new Date(Date.now() - 3600 * 1000 + 6000).toISOString();
		await api('/me/timer', { method: 'PUT', body: JSON.stringify({ startedAt }) });
		return true;
	})()`, nil)
	p.reload()
	p.settled()

	var titles []string

	for deadline := time.Now().Add(waitPatience); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		p.awaitJSON(`(async () => {
			const registration = await navigator.serviceWorker.ready;
			const shown = await registration.getNotifications({ tag: 'gts-stopwatch' });
			return shown.map((notification) => notification.title);
		})()`, &titles)

		if len(titles) > 0 {
			break
		}
	}

	if len(titles) != 1 || titles[0] != "The stopwatch is still running" {
		t.Errorf("the service worker shows %q once the stopwatch has crossed its hour", titles)
	}
}
