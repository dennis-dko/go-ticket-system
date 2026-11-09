package cli

import (
	"strings"
	"testing"
)

// assigner is an installation answering two entries to an account that may
// write its own time and give entries another project.
func assigner(t *testing.T) (*fakeInstallation, *watched) {
	t.Helper()

	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own", "timesheets:read:own", "timesheets:write:own", "timesheets:transfer"}
	f.answers = map[string]string{"GET /timesheets": twoEntries}

	return f, watch(t, server.URL, f.token)
}

// Several entries are given a project at once, as the page's bar gives the
// ticked rows one: each through its own transfer, one after another, to one of
// the active projects, and nothing is asked first - an entry given the wrong
// project is given the right one the same way.
func TestSeveralEntriesAreGivenAProjectOneAfterAnother(t *testing.T) {
	f, w := assigner(t)

	shows := w.entriesOpen()
	if !offered(shows, "Assign entries to a project", "a") {
		t.Fatalf("assigning entries to a project is not offered:\n%s", shows)
	}

	w.press("a")
	w.until("the entries are to be ticked", picking("assign"))
	w.press("space", "down", "space")

	shows = w.until("both are ticked", func(m screen) bool { return len(m.ticked) == 2 })
	if !strings.Contains(shows, "2 selected") {
		t.Errorf("the screen does not say how many are ticked:\n%s", shows)
	}

	w.press("enter")
	shows = w.until("the project is asked", asking("project"))

	if strings.Contains(shows, "No project") || strings.Contains(shows, "Archive") {
		t.Errorf("the entries are offered a project they cannot be given:\n%s", shows)
	}

	// Website, Webshop, Consulting.
	w.press("down", "enter")
	shows = w.until("the entries are given the project", func(m screen) bool { return settled(m) && m.news != "" })

	for _, route := range []string{"POST /timesheets/11/transfer", "POST /timesheets/12/transfer"} {
		if sent := sentOnce(t, f, route); sent["projectId"] != 2.0 {
			t.Errorf("%s was sent %v, want Webshop", route, sent)
		}
	}

	if !strings.Contains(shows, "✓ 2 assigned to Webshop") {
		t.Errorf("the screen does not say the entries were assigned:\n%s", shows)
	}

	w.close()
}

// Without the right to give entries another project, it is not offered, as the
// page's bar does not offer it.
func TestAssigningIsOfferedOnlyWithTheRightToIt(t *testing.T) {
	_, w := writer(t)

	if shows := w.entriesOpen(); offered(shows, "Assign entries to a project", "a") {
		t.Errorf("an account without the right is offered assigning entries:\n%s", shows)
	}

	w.close()
}

// Several entries are deleted once the screen has asked, in the page's words and
// naming them, and enter with nothing ticked takes the entry the cursor is on.
func TestSeveralEntriesAreDeletedAfterAsking(t *testing.T) {
	f, w := assigner(t)

	shows := w.entriesOpen()
	if !offered(shows, "Delete several entries", "x") {
		t.Fatalf("deleting several entries is not offered:\n%s", shows)
	}

	w.press("x")
	w.until("the entries are to be ticked", picking("deleteSome"))
	w.press("enter")

	shows = prose(w.until("the deletion is asked about", asking("confirm")))
	for _, want := range []string{"Delete the selected rows?", "This cannot be undone.", "2026-10-22, 2.50 h"} {
		if !strings.Contains(shows, want) {
			t.Errorf("the question does not say %q:\n%s", want, shows)
		}
	}

	w.press("y")
	shows = w.until("the entry is deleted", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /timesheets/11")) != 1 || len(f.sentTo("DELETE /timesheets/12")) != 0 {
		t.Errorf("deleted 11 %d times and 12 %d times, want 11 once", len(f.sentTo("DELETE /timesheets/11")),
			len(f.sentTo("DELETE /timesheets/12")))
	}

	if !strings.Contains(shows, "✓ 1 deleted") {
		t.Errorf("the screen does not say how many were deleted:\n%s", shows)
	}

	w.close()
}

// What the installation refused of several is said, with the first of them by
// name, as the page says it.
func TestWhatWasRefusedOfSeveralIsSaid(t *testing.T) {
	f, w := assigner(t)
	f.refuseAs = "the entry is locked"

	w.entriesOpen()
	w.press("x")
	w.until("the entries are to be ticked", picking("deleteSome"))
	w.press("space", "down", "space", "enter")
	w.until("the deletion is asked about", asking("confirm"))
	w.press("y")

	shows := prose(w.until("the refusal is said", func(m screen) bool { return settled(m) && m.failure != "" }))
	if !strings.Contains(shows, "✗ 2 could not be deleted – 2026-10-22, 2.50 h: the entry is locked") {
		t.Errorf("the screen does not say what was refused:\n%s", shows)
	}

	w.close()
}
