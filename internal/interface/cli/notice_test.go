package cli

import (
	"context"
	"image/color"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// prose is what a screen says with its frame taken away and its wrapped lines
// joined again, so that a sentence can be looked for whatever the width put a
// break into it.
func prose(view string) string {
	var said []string

	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		said = append(said, strings.Trim(line, "│╭╮╰╯─ "))
	}

	return strings.Join(strings.Fields(strings.Join(said, " ")), " ")
}

// readOnce opens a screen on an installation and reads the day into it, the way
// the program does before anybody presses a key.
func readOnce(t *testing.T, s *session) screen {
	t.Helper()

	m := newScreen(context.Background(), s)

	read, _ := m.load()().(loaded)
	if read.err != nil {
		t.Fatalf("reading the day: %v", read.err)
	}

	m, _ = m.read(read)
	m.width = 100

	return m
}

// The screen says what the browser's banners say: the installation's own banner
// under its name, and above the stopwatch whatever about the owner's own time
// deserves a banner - the stopwatch running longer than they wanted to be told
// after, the last working day with nothing booked, and a balance past a bound.
func TestTheScreenSaysWhatTheBrowsersBannersSay(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.alertHours = 1
	f.answers = map[string]string{
		"GET /branding": `{"title":"Time Recording","banner":"Maintenance on Friday from 18:00",` +
			`"translations":{"de":{"title":"Zeiterfassung","banner":"Wartung am Freitag ab 18 Uhr"}},"version":"v9.9.9"}`,
		"GET /me/notices": `{"unbookedDay":"2026-10-21",` +
			`"balance":{"hours":12.5,"since":"2026-01-05","upperBound":10,"lowerBound":-5,"outside":"above"}}`,
	}

	m := readOnce(t, &session{api: newClient(server.URL, f.token, language{}), out: io.Discard, build: testBuild})
	said := prose(m.View().Content)

	for _, want := range []string{
		"Maintenance on Friday from 18:00",
		"! The stopwatch has been running since 2026-10-22 09:12, longer than the 1 h you chose to be told after.",
		"! Nothing is booked on 2026-10-21, a working day by your plan.",
		"! Your running balance is +12.50 h, above the upper bound of +10.00 h you set.",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the screen does not say %q:\n%s", want, ansi.Strip(m.View().Content))
		}
	}

	// In German as the browser says it, the banner in the installation's German.
	de := speaking("de")
	m = readOnce(t, &session{api: newClient(server.URL, f.token, de), out: io.Discard, lang: de, build: testBuild})
	said = prose(m.View().Content)

	for _, want := range []string{
		"Wartung am Freitag ab 18 Uhr",
		"! Die Stoppuhr läuft seit 22.10.2026 09:12, länger als die 1 Std., nach denen du erinnert werden wolltest.",
		"! Am 21.10.2026 ist nichts gebucht, nach deinem Plan ein Arbeitstag.",
		"! Dein laufender Saldo liegt mit +12,50 Std. über der Obergrenze von +10,00 Std., die du gesetzt hast.",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the screen does not say %q:\n%s", want, ansi.Strip(m.View().Content))
		}
	}
}

// Nothing is said where nothing holds: a stopwatch its owner never wants to be
// told about, no banner, and no notices - which is also how an installation from
// before there were notices answers, with a route it does not have.
func TestTheScreenSaysNothingWhereNothingHolds(t *testing.T) {
	f, server := newFakeInstallation(t)

	m := readOnce(t, &session{api: newClient(server.URL, f.token, language{}), out: io.Discard, build: testBuild})

	if said := prose(m.View().Content); strings.Contains(said, "! ") {
		t.Errorf("the screen says something that does not hold:\n%s", ansi.Strip(m.View().Content))
	}
}

// The screen draws for the background the terminal says it has, and for the
// appearance the account chose only where the terminal says nothing: it cannot
// change the background, and the colours for a dark page are unreadable on a
// light one. Read off the frame as it is drawn - in the accent, a lighter blue
// for a dark background - so that what is checked is what the screen uses.
func TestTheTerminalsBackgroundDecidesTheColoursBeforeTheAccount(t *testing.T) {
	const darkAccent, lightAccent = "38;2;91;141;250", "38;2;47;111;235"

	for _, tc := range []struct {
		name           string
		theme          string
		measured, dark bool
		drawnDark      bool
	}{
		{"a dark account on a terminal that says nothing", "dark", false, false, true},
		{"a light account on a terminal that says nothing", "light", false, false, false},
		{"no choice on a terminal that says nothing", "", false, false, false},
		{"a dark account on a light terminal", "dark", true, false, false},
		{"a light account on a dark terminal", "light", true, true, true},
	} {
		f, server := newFakeInstallation(t)
		f.theme = tc.theme

		m := readOnce(t, &session{api: newClient(server.URL, f.token, language{}), out: io.Discard, build: testBuild})

		if tc.measured {
			next, _ := m.Update(tea.BackgroundColorMsg{Color: map[bool]color.Color{true: color.Black, false: color.White}[tc.dark]})
			m = next.(screen)
		}

		drawn := m.View().Content
		if got := strings.Contains(drawn, darkAccent); got != tc.drawnDark || strings.Contains(drawn, lightAccent) == tc.drawnDark {
			t.Errorf("%s is drawn dark: %v, want %v", tc.name, got, tc.drawnDark)
		}
	}
}

// A language the account chose since the screen was opened is spoken from its
// next read on - in the browser, or under Settings here - and the day is read
// again in it. One named when the client was started stays.
func TestAnOpenScreenFollowsTheLanguageTheAccountChose(t *testing.T) {
	f, server := newFakeInstallation(t)

	m := readOnce(t, &session{api: newClient(server.URL, f.token, language{}), out: io.Discard, build: testBuild})
	if !strings.Contains(prose(m.View().Content), "Choose what to do next.") {
		t.Fatalf("the screen is not in English to begin with:\n%s", ansi.Strip(m.View().Content))
	}

	f.mu.Lock()
	f.language = "de"
	f.mu.Unlock()

	read, _ := m.load()().(loaded)
	m, again := m.read(read)

	if m.s.lang.code() != "de" || again == nil {
		t.Fatalf("after the account chose German the screen speaks %q and reads again: %v", m.s.lang.code(), again != nil)
	}

	if said := prose(m.View().Content); !strings.Contains(said, "Wähle, was als Nächstes passieren soll.") {
		t.Errorf("the screen does not speak German:\n%s", ansi.Strip(m.View().Content))
	}

	asked := readOnce(t, &session{api: newClient(server.URL, f.token, language{}), out: io.Discard, build: testBuild, asked: true})
	if asked.s.lang.code() != "en" {
		t.Errorf("a language named at the start gave way to the account's: %q", asked.s.lang.code())
	}
}
