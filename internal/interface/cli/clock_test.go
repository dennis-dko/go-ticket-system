package cli

import (
	"strings"
	"testing"
)

// clockOpen opens the stopwatch card's menu once the day is read.
func (w *watched) clockOpen() string {
	w.t.Helper()

	w.until("the day is read", settled)
	w.press("u")

	return w.until("the stopwatch's menu is open", func(m screen) bool { return settled(m) && m.sub == subClock })
}

// The start of the running stopwatch is corrected to a day and a time on the
// account's own clock - the one somebody started late is the one most often
// wrong - and sent as the page sends it, without an offset.
func TestTheStartOfTheRunningStopwatchIsCorrected(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.clockOpen()

	if !offered(shows, "Correct the start", "c") {
		t.Fatalf("correcting the start is not offered:\n%s", shows)
	}

	w.press("c")
	shows = w.until("the day is asked", asking("clockDay"))

	// Started at 07:12 UTC, which is 09:12 in Berlin.
	if !strings.Contains(shows, "2026-10-22") {
		t.Errorf("the day is not the one the stopwatch was started on:\n%s", shows)
	}

	w.press("enter")
	shows = w.until("the time is asked", asking("clockTime"))

	if !strings.Contains(shows, "09:12") {
		t.Errorf("the time is not the one the stopwatch was started at:\n%s", shows)
	}

	w.press("backspace", "backspace", "backspace", "backspace", "backspace", "08:45", "enter")
	shows = w.until("the start is corrected", func(m screen) bool { return settled(m) && m.news != "" })

	if sent := sentOnce(t, f, "PUT /me/timer"); sent["startedAt"] != "2026-10-22T08:45" {
		t.Errorf("the start was sent as %v, want 2026-10-22T08:45", sent)
	}

	if !strings.Contains(shows, "✓ Start corrected") {
		t.Errorf("the screen does not say the start was corrected:\n%s", shows)
	}

	w.close()
}

// A stopwatch stopped earlier is booked up to the time given, on the day it
// was started.
func TestAStopwatchStoppedEarlierIsBookedUpToThen(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.clockOpen()

	if !offered(shows, "Stopped earlier", "e") {
		t.Fatalf("stopping earlier is not offered:\n%s", shows)
	}

	w.press("e")
	w.until("the day is asked", asking("clockDay"))
	w.press("enter")
	w.until("the time is asked", asking("clockTime"))
	w.press("backspace", "backspace", "backspace", "backspace", "backspace", "10:00", "enter")
	w.until("the stopwatch is booked", func(m screen) bool { return settled(m) && m.news != "" })

	if ended, _ := sentOnce(t, f, "POST /me/timer/stop")["endedAt"].(string); !strings.HasSuffix(ended, "T10:00") {
		t.Errorf("the end was sent as %q, want ten o'clock", ended)
	}

	w.close()
}

// The stopwatch is discarded only once the screen has asked, in the page's
// words, and nothing is booked.
func TestTheStopwatchIsDiscardedAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.clockOpen()

	if !offered(shows, "Discard", "d") {
		t.Fatalf("discarding the stopwatch is not offered:\n%s", shows)
	}

	w.press("d")
	shows = prose(w.until("the discarding is asked about", asking("confirm")))

	if !strings.Contains(shows, "Discard the measured time?") ||
		!strings.Contains(shows, "The clock is stopped and nothing is recorded. This cannot be undone.") {
		t.Errorf("the screen does not ask in the page's words:\n%s", shows)
	}

	w.press("y")
	shows = w.until("the stopwatch is discarded", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /me/timer")) != 1 || len(f.sentTo("POST /me/timer/stop")) != 0 {
		t.Errorf("discarding sent %v and stopped %v", f.sentTo("DELETE /me/timer"), f.sentTo("POST /me/timer/stop"))
	}

	if !strings.Contains(shows, "✓ The clock was discarded") {
		t.Errorf("the screen does not say the stopwatch was discarded:\n%s", shows)
	}

	w.close()
}

// A stopwatch running longer than one entry can hold - one somebody forgot - is
// not offered a stop now or a switch, which the installation would refuse, and
// stopping it asks when the work ended, saying why.
func TestAForgottenStopwatchIsBookedOnlyUpToAnEnd(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.answers = map[string]string{"GET /me/timer": `{"running":true,"projectId":4,` +
		`"startedAt":"2026-10-20T07:12:00Z","elapsedHours":30}`}

	w := watch(t, server.URL, f.token)
	shows := w.until("the day is read", settled)

	if offered(shows, "Switch to another project", "w") {
		t.Errorf("a forgotten stopwatch is offered a switch:\n%s", shows)
	}

	w.press("x")
	shows = prose(w.until("the end is asked", asking("clockDay")))

	if !strings.Contains(shows, "longer than one entry can hold") {
		t.Errorf("the screen does not say why it asks when the work ended:\n%s", shows)
	}

	w.press("esc")
	w.close()
}

// Without the right to write its own time, an account is offered neither the
// stopwatch nor booking, as the page hides both, and the entries stay.
func TestWithoutTheRightToWriteOnlyTheEntriesAreOffered(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own", "timesheets:read:own"}

	w := watch(t, server.URL, f.token)
	shows := w.until("the day is read", settled)

	for _, change := range [][2]string{{"Switch to another project", "w"}, {"Stop the stopwatch", "x"},
		{"Book time by hand", "b"}, {"Stopwatch …", "u"}} {
		if offered(shows, change[0], change[1]) {
			t.Errorf("an account that may not write its time is offered %q:\n%s", change[0], shows)
		}
	}

	if strings.Contains(shows, "since 09:12") {
		t.Errorf("an account that may not write its time is shown the stopwatch:\n%s", shows)
	}

	if !offered(shows, "Entries …", "e") {
		t.Errorf("the entries are not offered:\n%s", shows)
	}

	w.close()
}
