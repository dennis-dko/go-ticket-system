package mobile

import (
	"regexp"
	"strings"
	"testing"
)

// The page puts away its card about installing it as an app when it is shown in
// the Android app, which is one already: there the card told somebody to look in
// a browser's menu that is not on the screen. The page knows the app by the
// bridge the app registers for saving files, so the name the page asks for is
// the name Web.kt registers.
func TestThePageKnowsTheAppByTheBridgeItRegisters(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	registered := regexp.MustCompile(`\bconst\s+val\s+filesInterface\s*=\s*"([A-Za-z]+)"`).FindStringSubmatch(code)
	if registered == nil {
		t.Fatal("Web.kt names no filesInterface, so the page cannot know it is in the app")
	}

	script := read(t, "../web/asset/app.js")

	// Each place the page behaves differently inside the app, and what it does
	// there when it no longer recognises the app.
	for function, otherwise := range map[string]string{
		"wireInstall":             "shows a card about installing the page as an app",
		"passkeysWithheldBecause": "says the phone's browser cannot use passkeys, where the app signs in with its token",
	} {
		start := strings.Index(script, "function "+function+"()")
		if start < 0 {
			t.Fatalf("app.js has no %s", function)
		}

		body := script[start:]
		body = body[:strings.Index(body, "\n}\n")]

		if !strings.Contains(body, "'"+registered[1]+"' in window") {
			t.Errorf("%s does not ask for %s, the bridge the app registers, so the app %s", function, registered[1], otherwise)
		}
	}
}

// The page asks the phone for the notifications the app shows its reminders as,
// through the bridge the app registers for them - a WebView has no
// Notification, and without the bridge the Reminders card says no browser can
// show any. So the name the page asks for is the name Web.kt registers, and the
// app declares the permission it asks the phone for.
func TestThePageAsksForTheAppsNotificationsByTheBridgeItRegisters(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	registered := regexp.MustCompile(`\bconst\s+val\s+notificationsInterface\s*=\s*"([A-Za-z]+)"`).FindStringSubmatch(code)
	if registered == nil {
		t.Fatal("Web.kt names no notificationsInterface, so the page cannot ask the phone for notifications")
	}

	if !strings.Contains(read(t, "../web/asset/app.js"), "window."+registered[1]) {
		t.Errorf("app.js does not ask for %s, the bridge the app registers, so in the app the "+
			"Reminders card says that no browser can show notifications", registered[1])
	}

	if !strings.Contains(read(t, "src/main/AndroidManifest.xml"), `android:name="android.permission.POST_NOTIFICATIONS"`) {
		t.Error("the app does not declare POST_NOTIFICATIONS, so the phone is never asked and no reminder is shown")
	}
}

// The page tells the app when its session ended without its being signed out
// of, through the bridge the app registers for it - the app cannot see that for
// itself, and without it a token taken back left the app showing the page's own
// sign-in form while it went on asking about the reminders with a token the
// installation no longer knew. So the name the page tells is the name Web.kt
// registers.
func TestThePageTellsTheAppItsSessionEndedByTheBridgeItRegisters(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	registered := regexp.MustCompile(`\bconst\s+val\s+sessionInterface\s*=\s*"([A-Za-z]+)"`).FindStringSubmatch(code)
	if registered == nil {
		t.Fatal("Web.kt names no sessionInterface, so the page cannot tell the app its session ended")
	}

	if !strings.Contains(read(t, "../web/asset/app.js"), "window."+registered[1]+"?.ended()") {
		t.Errorf("app.js does not tell %s, the bridge the app registers, that its session ended, so a "+
			"token taken back leaves the app on the page's own sign-in form", registered[1])
	}
}

// Every bridge the app registers is put on the page under the name a page from
// before the rename to TimePulse looks for as well. The app is updated apart from
// the installation whose page it shows, and an older page that finds no bridge
// takes the app for a browser: it offers to install itself, and a token taken
// back leaves it on its own sign-in form.
func TestTheAppPutsEveryBridgeOnThePageUnderTheOldNameToo(t *testing.T) {
	code, _ := lexed(source(t, "Web.kt"))

	for _, bridge := range []string{"filesInterface", "notificationsInterface", "sessionInterface"} {
		put := regexp.MustCompile(`\bbridgeNames\(\s*` + bridge + `\s*\)\.forEach\s*\{\s*(?:view\.)?addJavascriptInterface\(`)
		if !put.MatchString(code) {
			t.Errorf("Web.kt does not put %s on the page through bridgeNames, so a page from before the rename does not find it", bridge)
		}
	}

	if !regexp.MustCompile(`"GoTimeRecording"\s*\+\s*name\.removePrefix\("TimePulse"\)`).MatchString(code) {
		t.Error("bridgeNames no longer gives a bridge the name a page from before the rename looks for")
	}
}
