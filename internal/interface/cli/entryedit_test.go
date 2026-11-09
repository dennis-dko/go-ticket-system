package cli

import (
	"strings"
	"testing"
)

// twoEntries answers the list with entry 11 on Consulting, saved again since it
// was booked, and entry 12 with neither project nor description.
const twoEntries = `{"items":[` +
	`{"id":11,"projectId":4,"date":"2026-10-22","durationHours":2.5,"description":"review",` +
	`"createdAt":"2026-10-22T08:00:00Z","updatedAt":"2026-10-22T09:30:00Z"},` +
	`{"id":12,"projectId":null,"date":"2026-10-21","durationHours":0.5,` +
	`"createdAt":"2026-10-21T08:00:00Z","updatedAt":"2026-10-21T08:00:00Z"}],"totalCount":2}`

// writer is an installation answering two entries to an account that may write
// its own time.
func writer(t *testing.T) (*fakeInstallation, *watched) {
	t.Helper()

	f, server := newFakeInstallation(t)
	f.permissions = []string{"settings:write:own", "timesheets:read:own", "timesheets:write:own"}
	f.answers = map[string]string{"GET /timesheets": twoEntries}

	return f, watch(t, server.URL, f.token)
}

// picking is a screen waiting for an entry to be picked for doing.
func picking(doing string) func(screen) bool {
	return func(m screen) bool { return settled(m) && m.picking == doing }
}

// An entry is corrected in the booking form, filled with what it says now: the
// entry is picked in the card, the form asks what every booking asks, and what
// it was answered with is sent to the entry.
func TestAnEntryIsCorrectedInTheBookingFormFilledWithIt(t *testing.T) {
	f, w := writer(t)

	shows := w.entriesOpen()
	if !offered(shows, "Correct an entry", "e") {
		t.Fatalf("correcting an entry is not offered:\n%s", shows)
	}

	w.press("e")
	shows = w.until("an entry is to be picked", picking("edit"))

	if lineWith(shows, "›") != "Time entries › Entries › Correct an entry" {
		t.Errorf("the breadcrumb does not say an entry is picked to be corrected: %q", lineWith(shows, "›"))
	}

	if row := lineWith(shows, "> 2026-10-22"); !strings.Contains(row, "Consulting") {
		t.Errorf("the cursor is not on the first entry:\n%s", shows)
	}

	w.press("enter")
	shows = w.until("the hours are asked", asking("hours"))

	if !strings.Contains(shows, "2.50") {
		t.Errorf("the form is not filled with the entry's hours:\n%s", shows)
	}

	w.press("backspace", "backspace", "backspace", "backspace", "3", "enter")
	w.until("the project is asked", asking("project"))
	w.press("enter")
	w.until("the day is asked", asking("day"))
	w.press("enter")
	w.until("the description is asked", asking("description"))
	w.press("enter")

	shows = w.until("the correction is saved", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "PUT /timesheets/11")
	if sent["durationHours"] != 3.0 || sent["projectId"] != 4.0 || sent["date"] != "2026-10-22" || sent["description"] != "review" {
		t.Errorf("the correction was sent as %v", sent)
	}

	if !strings.Contains(shows, "✓ Entry saved") {
		t.Errorf("the screen does not say the entry was saved:\n%s", shows)
	}

	w.close()
}

// Picking an entry is put away with escape, and nothing is changed.
func TestPickingAnEntryIsPutAwayWithEscape(t *testing.T) {
	f, w := writer(t)

	w.entriesOpen()
	w.press("e")
	w.until("an entry is to be picked", picking("edit"))
	w.press("esc")

	shows := w.until("the entries' menu is back", func(m screen) bool { return settled(m) && m.picking == "" && m.sub == subEntries })
	if !offered(shows, "Correct an entry", "e") {
		t.Errorf("escape does not go back to the entries' menu:\n%s", shows)
	}

	if len(f.sentTo("PUT /timesheets/11")) > 0 {
		t.Error("an entry was changed although picking it was put away")
	}

	w.close()
}

// An entry is deleted only once the screen has asked, in the page's words, and
// the answer no deletes nothing.
func TestAnEntryIsDeletedAfterAsking(t *testing.T) {
	f, w := writer(t)

	shows := w.entriesOpen()
	if !offered(shows, "Delete an entry", "d") {
		t.Fatalf("deleting an entry is not offered:\n%s", shows)
	}

	w.press("d")
	w.until("an entry is to be picked", picking("delete"))
	w.press("down", "enter")

	shows = prose(w.until("the deletion is asked about", asking("confirm")))
	if !strings.Contains(shows, "Delete for good?") || !strings.Contains(shows, "2026-10-21, 0.50 h will be deleted. This cannot be undone.") {
		t.Errorf("the screen does not ask before deleting, in the page's words:\n%s", shows)
	}

	w.press("n")
	w.until("the entries' menu is back", func(m screen) bool { return settled(m) && m.form == nil })

	if len(f.sentTo("DELETE /timesheets/12")) > 0 {
		t.Fatal("an entry was deleted although the answer was no")
	}

	w.press("d")
	w.until("an entry is to be picked", picking("delete"))
	w.press("down", "enter")
	w.until("the deletion is asked about", asking("confirm"))
	w.press("y")

	shows = w.until("the entry is deleted", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /timesheets/12")) != 1 {
		t.Errorf("the entry was not deleted once: %v", f.sentTo("DELETE /timesheets/12"))
	}

	if !strings.Contains(shows, "✓ Entry deleted") {
		t.Errorf("the screen does not say the entry was deleted:\n%s", shows)
	}

	w.close()
}

// What an entry said before it was corrected is shown until it is closed, for
// the entries saved again since they were booked - the only ones with a history.
func TestTheHistoryOfACorrectedEntryIsShownUntilItIsClosed(t *testing.T) {
	f, w := writer(t)
	f.answers["GET /timesheets/11/revisions"] = `{"items":[{"date":"2026-10-22","durationHours":1.5,"projectId":1,` +
		`"description":"first go","replacedAt":"2026-10-22T09:30:00Z","replacedBy":{"id":3,"name":"Max Manager"}}]}`

	shows := w.entriesOpen()
	if !offered(shows, "Earlier versions of an entry", "h") {
		t.Fatalf("the history of an entry is not offered:\n%s", shows)
	}

	w.press("h")
	w.until("an entry is to be picked", picking("history"))

	// The entry never corrected has no history, so the cursor does not stop on it.
	w.press("down", "enter")

	shows = w.until("the history is shown", func(m screen) bool { return m.past != nil })

	for _, want := range []string{"Earlier versions of the entry from 2026-10-22", "Replaced", "by Max Manager",
		"Website", "1.50", "first go"} {
		if !strings.Contains(shows, want) {
			t.Errorf("the history does not show %q:\n%s", want, shows)
		}
	}

	w.press("esc")
	w.until("the history is closed", func(m screen) bool { return settled(m) && m.past == nil && m.sub == subEntries })

	if len(f.sentTo("GET /timesheets/12/revisions")) > 0 {
		t.Error("the history of an entry never corrected was asked for")
	}

	w.close()
}
