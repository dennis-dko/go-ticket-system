//go:build browser

package browser

import (
	"context"
	"slices"
	"testing"

	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// thisBrowsersChannel is the push channel the stand-in below hands out: RFC 8291's
// example browser, subscribed through Google's push service.
const thisBrowsersChannel = "https://fcm.googleapis.com/fcm/send/browser-case"

// standInForThePushService replaces the browser's service worker and push manager
// before the page loads, because a headless browser reaches no push service: what
// the page asks of them is counted, and the subscription it is handed is the
// RFC's example browser, which the real installation behind it accepts.
var standInForThePushService = chromedp.ActionFunc(func(ctx context.Context) error {
	_, err := cdppage.AddScriptToEvaluateOnNewDocument(`(() => {
		let held = null;
		window.pushAsked = { subscribe: 0, unsubscribe: 0, keyLength: 0 };
		const channel = {
			endpoint: '` + thisBrowsersChannel + `',
			toJSON: () => ({ endpoint: channel.endpoint, keys: {
				p256dh: 'BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4',
				auth: 'BTBZMqHH6r4Tts7J_aSIgg' } }),
			unsubscribe: () => { window.pushAsked.unsubscribe += 1; held = null; return Promise.resolve(true); },
		};
		const registration = {
			pushManager: {
				getSubscription: () => Promise.resolve(held),
				subscribe: (options) => {
					window.pushAsked.subscribe += 1;
					window.pushAsked.keyLength = options.applicationServerKey.length;
					held = channel;
					return Promise.resolve(channel);
				},
			},
			showNotification: () => Promise.resolve(),
		};
		Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: {
			ready: Promise.resolve(registration),
			getRegistration: () => Promise.resolve(registration),
			register: () => Promise.resolve(registration),
		} });
		window.Notification = class {
			static permission = 'granted';
			static requestPermission() { return Promise.resolve('granted'); }
		};
	})()`).Do(ctx)

	return err
})

// Where the installation pushes, the notification card subscribes the browser it is
// open in and takes it back again, and says which it is; and signing out of the
// browser takes it out of the pushes, so whoever uses it next is not pushed the
// notifications of whoever left.
func TestTheNotificationCardSubscribesThisBrowserAndSigningOutTakesItBack(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()
	p.awaitJSON(`api('/settings/push', { method: 'PUT', body: JSON.stringify({ enabled: true }) })`, nil)
	p.becomeWorker()

	p.run("load the page with the push service stood in for", standInForThePushService, chromedp.Reload())
	p.settled()
	p.showView("settings")
	p.waitShown("#push-subscribe")

	mine := func() []string {
		var state struct {
			Endpoints []string `json:"endpoints"`
		}

		p.awaitJSON(`api('/me/push')`, &state)

		return state.Endpoints
	}

	p.run("push to this browser", p.click("#push-subscribe"))
	p.waitShown("#push-state")
	p.waitShown("#push-unsubscribe")

	var asked struct {
		Subscribe   int `json:"subscribe"`
		Unsubscribe int `json:"unsubscribe"`
		KeyLength   int `json:"keyLength"`
	}

	p.awaitJSON(`window.pushAsked`, &asked)

	if asked.Subscribe != 1 || asked.KeyLength != 65 {
		t.Errorf("the browser was asked to subscribe %d time(s) with a key of %d bytes, want once with 65",
			asked.Subscribe, asked.KeyLength)
	}

	if !slices.Contains(mine(), thisBrowsersChannel) {
		t.Fatalf("after pushing to this browser the installation has %v", mine())
	}

	p.run("stop pushing to this browser", p.click("#push-unsubscribe"))
	p.waitShown("#push-subscribe")
	p.waitGone("#push-state")

	if got := mine(); len(got) != 0 {
		t.Errorf("after stopping, the installation still pushes to %v", got)
	}

	p.run("push to this browser again", p.click("#push-subscribe"))
	p.waitShown("#push-unsubscribe")

	p.run("sign out", chromedp.Click("#logout", chromedp.ByID),
		chromedp.WaitVisible("#form-login", chromedp.ByID))

	p.signIn(workerEmail, workerPassword)
	p.waitGone("#login-screen")
	p.settled()

	if got := mine(); len(got) != 0 {
		t.Errorf("after signing out of this browser, the installation still pushes to %v", got)
	}

	p.awaitJSON(`window.pushAsked`, &asked)

	if asked.Unsubscribe != 2 {
		t.Errorf("the browser dropped its subscription %d time(s), want twice: once by the button, once at signing out",
			asked.Unsubscribe)
	}
}
