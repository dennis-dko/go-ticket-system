package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// saved is a screen that has sent a change and read the day again since.
func saved(m screen) bool { return settled(m) && m.news != "" }

// settingsOpen moves to My account, whose menu is the settings: the last tab, so
// the left arrow goes round to it from the first, whichever tabs stand between.
func (w *watched) settingsOpen() {
	w.t.Helper()

	w.until("the day is read", settled)
	w.press("left")
	w.until("the settings are offered", onTab(viewAccount))
}

// sentOnce is what the installation was sent on a route, which it was sent
// once.
func sentOnce(t *testing.T, f *fakeInstallation, route string) map[string]any {
	t.Helper()

	sent := f.sentTo(route)
	if len(sent) != 1 {
		t.Fatalf("%s was sent %d times, want once: %v", route, len(sent), sent)
	}

	return sent[0]
}

// The settings say what each is set to now, as the account and the installation
// have them - the browser's choices, read with the day.
func TestTheSettingsSayWhatEachIsSetToNow(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.alertHours = 10

	w := watch(t, server.URL, f.token)
	w.settingsOpen()

	shows := prose(w.until("the settings are offered", onTab(viewAccount)))

	for _, want := range []string{
		"Language: English", "Appearance: Automatic", "My timezone: follows the installation (Europe/Berlin)",
		"My public holidays: follows the installation (Germany, Bavaria)",
		"Reminders: stopwatch after 10 h, last working day on", "My working hours: 8.00 h a day",
	} {
		if !strings.Contains(shows, want) {
			t.Errorf("the settings do not say %q:\n%s", want, shows)
		}
	}

	// The bounds only where the installation answers the balance they bound.
	if strings.Contains(shows, "Bounds of my balance") {
		t.Errorf("the settings offer bounds for a balance nobody answered:\n%s", shows)
	}

	w.press("esc")
	w.close()
}

// The appearance is saved to the account, and the line under the screen says
// that the browser and the app follow it.
func TestTheAppearanceIsSavedToTheAccount(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("down", "enter")
	w.until("the appearance is asked", asking("theme"))
	w.press("down", "down", "enter")

	shows := prose(w.until("the change is saved", saved))

	if sent := sentOnce(t, f, "PUT /me/theme"); sent["theme"] != "dark" {
		t.Errorf("the appearance was sent as %v, want dark", sent)
	}

	if !strings.Contains(shows, "Saved for your account - the browser and the app follow it too.") {
		t.Errorf("the screen does not say where the change went:\n%s", shows)
	}

	w.close()
}

// A language chosen here is saved to the account and spoken from the next read
// on, the line saying so in it.
func TestALanguageChosenHereIsSpokenAtOnce(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("enter")
	w.until("the language is asked", asking("language"))
	w.press("down", "enter")

	shows := prose(w.until("the screen speaks German", func(m screen) bool { return saved(m) && m.s.lang.code() == "de" }))

	if sent := sentOnce(t, f, "PUT /me/language"); sent["language"] != "de" {
		t.Errorf("the language was sent as %v, want de", sent)
	}

	for _, want := range []string{"Was möchtest du einstellen?", "In deinem Konto gespeichert"} {
		if !strings.Contains(shows, want) {
			t.Errorf("the screen does not say %q:\n%s", want, shows)
		}
	}

	w.close()
}

// The public holidays are chosen as in the browser: a country, and the part of
// it that decides where its parts divide it - which has to be chosen, and is
// called what that country calls them.
func TestThePublicHolidaysAreChosenByCountryAndState(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("down", "down", "down", "enter")
	w.until("the country is asked", asking("country"))

	// Following the installation, none, then Austria, Germany, the United Kingdom.
	//
	// Waited for until the states are there: huh reads a select's options from
	// its function in a command of its own, so for a moment after the field takes
	// the focus it holds none, and a key pressed then acts on nothing. Under the
	// race detector that moment was long enough for this case's enter to land in
	// it.
	w.press("down", "down", "down", "enter")
	shows := prose(w.until("the states are offered", func(m screen) bool {
		return asking("region")(m) && strings.Contains(prose(m.View().Content), "Bavaria")
	}))

	if !strings.Contains(shows, "My account › My public holidays › State") {
		t.Errorf("Germany's parts are not asked for as states:\n%s", shows)
	}

	// Choose one is no answer for a country whose states divide it.
	w.press("enter")
	w.until("the state is still asked", func(m screen) bool { return asking("region")(m) && m.form.Errors() != nil })

	w.press("down", "down", "enter")
	w.until("the change is saved", saved)

	if sent := sentOnce(t, f, "PUT /me/holidays"); sent["place"] != "DE-BY" {
		t.Errorf("the holidays were sent as %v, want DE-BY", sent)
	}

	w.close()
}

// The reminders are sent together, as the browser sends them, and the hours
// are a whole number from none to a day.
func TestTheRemindersAreSavedToTheAccount(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("down", "down", "down", "down", "enter")
	w.until("the unbooked day is asked", asking("unbookedDay"))
	w.press("n")
	w.until("the hours are asked", asking("alertHours"))

	w.press("backspace", "25", "enter")
	w.until("too many hours are refused", func(m screen) bool { return asking("alertHours")(m) && m.form.Errors() != nil })

	w.press("backspace", "backspace", "5", "enter")
	w.until("the change is saved", saved)

	want := map[string]any{"unbookedDay": false, "timerAlertHours": 5.0}
	if sent := sentOnce(t, f, "PUT /me/reminders"); !reflect.DeepEqual(sent, want) {
		t.Errorf("the reminders were sent as %v, want %v", sent, want)
	}

	w.close()
}

// A zone this does not know is refused where it is typed, and one it knows is
// saved.
func TestAZoneIsCheckedBeforeItIsSaved(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("down", "down", "enter")
	w.until("the zone is asked", asking("timezone"))
	w.press("Mars/Base", "enter")

	shows := prose(w.until("the zone is refused", func(m screen) bool { return asking("timezone")(m) && m.form.Errors() != nil }))
	if !strings.Contains(shows, "Mars/Base is not a time zone") {
		t.Errorf("the screen does not say why the zone was refused:\n%s", shows)
	}

	w.press(slices.Repeat([]string{"backspace"}, len("Mars/Base"))...)
	w.press("Europe/Vienna", "enter")
	w.until("the change is saved", saved)

	if sent := sentOnce(t, f, "PUT /me/timezone"); sent["timezone"] != "Europe/Vienna" {
		t.Errorf("the zone was sent as %v, want Europe/Vienna", sent)
	}

	w.close()
}

// The working hours are sent to the route the browser sends them to: a figure
// written with a decimal comma, empty for the installation's own, and the day
// every working day counts from.
func TestTheWorkingHoursAreSavedToTheAccount(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("down", "down", "down", "down", "down", "enter")
	w.until("the target is asked", asking("targetHours"))
	w.press("backspace", "7,5", "enter")
	w.until("the maximum is asked", asking("maxHours"))
	w.press("enter")
	w.until("the day is asked", asking("countsFrom"))
	w.press("2026-11-02", "enter")
	w.until("the change is saved", saved)

	want := map[string]any{"dailyTargetHours": 7.5, "maxDailyHours": 0.0, "everyDayCountsFrom": "2026-11-02"}
	if sent := sentOnce(t, f, "PUT /users/7/working-times"); !reflect.DeepEqual(sent, want) {
		t.Errorf("the working hours were sent as %v, want %v", sent, want)
	}

	w.close()
}

// The bounds of the balance are offered where the installation answers the
// balance, and a bound of zero is sent as the bound it is rather than as none.
func TestABoundOfZeroIsKept(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.answers = map[string]string{
		"GET /me/notices": `{"balance":{"hours":2,"since":"2026-01-05","upperBound":0,"lowerBound":null,"outside":"above"}}`,
	}

	w := watch(t, server.URL, f.token)
	w.settingsOpen()
	w.press("down", "down", "down", "down", "down", "enter")
	w.until("the upper bound is asked", asking("upper"))
	w.press("enter")
	w.until("the lower bound is asked", asking("lower"))
	w.press("-5", "enter")
	w.until("the change is saved", saved)

	want := map[string]any{"upperBound": 0.0, "lowerBound": -5.0}
	if sent := sentOnce(t, f, "PUT /me/balance-limits"); !reflect.DeepEqual(sent, want) {
		t.Errorf("the bounds were sent as %v, want %v", sent, want)
	}

	w.close()
}
