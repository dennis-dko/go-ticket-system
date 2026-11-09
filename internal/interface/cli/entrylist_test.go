package cli

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// listQueries are the queries the entries' list was asked with, leaving out the
// count of the entries waiting for a project, which asks for one entry.
func listQueries(f *fakeInstallation) []url.Values {
	var asked []url.Values

	for _, sent := range f.sentTo("GET /timesheets") {
		if q, ok := sent["?"].(url.Values); ok && q.Get("limit") != "1" {
			asked = append(asked, q)
		}
	}

	return asked
}

// lastList is the query the entries' list was last asked with.
func lastList(t *testing.T, f *fakeInstallation) url.Values {
	t.Helper()

	asked := listQueries(f)
	if len(asked) == 0 {
		t.Fatal("the entries were never asked for")
	}

	return asked[len(asked)-1]
}

// numbered answers the list with total entries, "entry 01" first, as many from
// the offset on as the limit asks for; the count of entries without a project
// is answered with none.
func numbered(total int) func(url.Values) string {
	return func(q url.Values) string {
		if q.Get("projectId") == "none" {
			return `{"items":[],"totalCount":0}`
		}

		offset, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("limit"))

		var items []string

		for i := offset; i < min(offset+limit, total); i++ {
			items = append(items, fmt.Sprintf(`{"id":%d,"projectId":1,"date":"2026-09-%02d","durationHours":1,"description":"entry %02d"}`,
				i+1, 30-i%30, i+1))
		}

		return `{"items":[` + strings.Join(items, ",") + `],"totalCount":` + strconv.Itoa(total) + `}`
	}
}

// entriesOpen opens the entries card's menu once the day is read.
func (w *watched) entriesOpen() string {
	w.t.Helper()

	w.until("the day is read", settled)
	w.press("e")

	return w.until("the entries' menu is open", func(m screen) bool { return settled(m) && m.sub == subEntries })
}

// The entries are listed as the page lists them: under its columns, the hours
// as a figure, no project and no description in words the page uses, a page of
// them at a time from the first.
func TestTheEntriesAreListedAsThePageListsThem(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.until("the entries are read", func(m screen) bool { return settled(m) && len(m.card.items) == 2 })

	if lineWith(shows, "Entries", "All projects") == "" || lineWith(shows, "Date", "Project", "Hours", "Description") == "" {
		t.Errorf("the entries are not headed as the page heads them:\n%s", shows)
	}

	if row := lineWith(shows, "2026-10-22", "Consulting"); !strings.Contains(row, "2.50") || !strings.Contains(row, "review") {
		t.Errorf("an entry is not listed with its day, project, hours and description: %q", row)
	}

	if row := lineWith(shows, "2026-10-22", "No project"); !strings.Contains(row, "0.50") || !strings.HasSuffix(row, "–") {
		t.Errorf("an entry with neither project nor description is not listed as the page lists it: %q", row)
	}

	if q := lastList(t, f); q.Get("limit") != "25" || q.Get("offset") != "0" {
		t.Errorf("the entries were asked for as %v, want the first 25", q)
	}

	w.close()
}

// The entries are narrowed as the page's filters narrow them - by the first and
// the last day, the words in the descriptions and the project - the screen says
// what it narrowed them to, and the filter can be taken off again.
func TestTheEntriesAreNarrowedAsThePagesFiltersNarrowThem(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	w.entriesOpen()

	w.press("f")
	w.until("the first day is asked", asking("from"))
	w.press("2026-10-01", "enter")
	w.until("the last day is asked", asking("to"))
	w.press("2026-10-31", "enter")
	w.until("the words are asked", asking("search"))
	w.press("review", "enter")
	w.until("the project is asked", asking("scope"))

	// All projects, Website, Webshop, Archive, Consulting, No project.
	w.press("down", "down", "down", "down", "enter")

	shows := w.until("the entries are read again", func(m screen) bool {
		return settled(m) && m.card.filter.project == "4" && len(m.card.items) > 0
	})

	want := url.Values{"from": {"2026-10-01"}, "to": {"2026-10-31"}, "q": {"review"}, "projectId": {"4"},
		"limit": {"25"}, "offset": {"0"}}
	if q := lastList(t, f); !reflect.DeepEqual(q, want) {
		t.Errorf("the entries were asked for as %v, want %v", q, want)
	}

	if !strings.Contains(shows, "Consulting · 2026-10-01 – 2026-10-31 · “review”") {
		t.Errorf("the screen does not say what the entries are narrowed to:\n%s", shows)
	}

	if !offered(shows, "Show every entry again", "c") {
		t.Fatalf("the filter is not offered to be taken off:\n%s", shows)
	}

	w.press("c")
	w.until("every entry is read again", func(m screen) bool { return settled(m) && m.card.filter == (entryFilter{}) && len(m.card.items) > 0 })

	if q := lastList(t, f); len(q) != 2 {
		t.Errorf("the entries were asked for as %v, want every one", q)
	}

	w.close()
}

// The entries still waiting for a project are one choice away while there are
// any, as the page's link is, and choosing it narrows the list to them.
func TestTheEntriesWithoutAProjectAreOneChoiceAway(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.list = func(q url.Values) string {
		if q.Get("projectId") == "none" {
			return `{"items":[{"projectId":null,"date":"2026-10-22","durationHours":0.5}],"totalCount":3}`
		}

		return `{"items":[{"projectId":4,"date":"2026-10-22","durationHours":2.5}],"totalCount":1}`
	}

	w := watch(t, server.URL, f.token)
	shows := w.entriesOpen()

	if !offered(shows, "3 without a project", "o") {
		t.Fatalf("the entries without a project are not offered:\n%s", shows)
	}

	w.press("o")
	shows = w.until("the entries without a project are read", func(m screen) bool {
		return settled(m) && m.card.filter.project == "none" && len(m.card.items) == 1
	})

	if lineWith(shows, "Entries", "No project") == "" {
		t.Errorf("the screen does not say the entries are the ones without a project:\n%s", shows)
	}

	if offered(shows, "3 without a project", "o") {
		t.Errorf("the entries without a project are offered while they are the ones shown:\n%s", shows)
	}

	w.close()
}

// More entries are read a page at a time, as the page's "Show more" reads them,
// and reading the day again keeps every entry read so far.
func TestMoreEntriesAreReadAPageAtATime(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.list = numbered(30)

	w := watch(t, server.URL, f.token)
	shows := w.until("the first page is read", func(m screen) bool { return settled(m) && len(m.card.items) == 25 })

	if !strings.Contains(shows, "Showing 25 of 30 entries") {
		t.Errorf("the screen does not say how many of the entries it shows:\n%s", shows)
	}

	shows = w.entriesOpen()
	if !offered(shows, "Show more", "m") {
		t.Fatalf("more entries are not offered:\n%s", shows)
	}

	w.press("m")
	shows = w.until("the next page is read", func(m screen) bool { return settled(m) && len(m.card.items) == 30 })

	if q := lastList(t, f); q.Get("offset") != "25" || q.Get("limit") != "25" {
		t.Errorf("the next page was asked for as %v, want 25 from the 26th", q)
	}

	if strings.Contains(shows, "Showing") || offered(shows, "Show more", "m") {
		t.Errorf("the screen offers more where every entry is shown:\n%s", shows)
	}

	asked := len(listQueries(f))

	w.press("r")
	w.until("the day is read again", func(m screen) bool { return settled(m) && len(listQueries(f)) > asked })

	if q := lastList(t, f); q.Get("offset") != "0" || q.Get("limit") != "30" {
		t.Errorf("reading the day again asked for %v, want the 30 shown from the first", q)
	}

	w.close()
}

// A screen with more entries than its window has room for is drawn no taller
// than the window, the menu and the keys under it included: the entries give
// way, and the rest of the screen leaves them room.
func TestTheScreenIsNoTallerThanItsWindow(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.list = numbered(25)

	w := watch(t, server.URL, f.token)
	shows := w.until("the entries are read", func(m screen) bool { return settled(m) && len(m.card.items) == 25 })

	if lines := strings.Count(shows, "\n") + 1; lines > 40 {
		t.Errorf("the screen is %d lines in a window of 40:\n%s", lines, shows)
	}

	if rows := strings.Count(shows, "entry "); rows < 5 {
		t.Errorf("the screen leaves room for %d entries in a window of 40:\n%s", rows, shows)
	}
}

// Entries the window has no room for are paged through, a window's worth at a
// time, and the card says which of them it shows.
func TestEntriesTheWindowHasNoRoomForArePagedThrough(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.list = numbered(25)

	w := watch(t, server.URL, f.token)
	shows := w.until("the entries are read", func(m screen) bool { return settled(m) && len(m.card.items) == 25 })

	rows := strings.Count(shows, "entry ")
	if rows >= 25 || rows < listFewest {
		t.Fatalf("a window of 40 lines shows %d of 25 entries:\n%s", rows, shows)
	}

	if !strings.Contains(shows, fmt.Sprintf("1–%d of 25", rows)) || !strings.Contains(shows, "entry 01") {
		t.Errorf("the card does not say it shows the first %d:\n%s", rows, shows)
	}

	w.press("pgdown")
	shows = w.until("the next entries are shown", func(m screen) bool { return m.top > 0 })

	if strings.Contains(shows, "entry 01") || !strings.Contains(shows, fmt.Sprintf("entry %02d", rows+1)) {
		t.Errorf("paging down does not show the entries after the first %d:\n%s", rows, shows)
	}

	w.press("pgup")
	shows = w.until("the first entries are shown again", func(m screen) bool { return m.top == 0 })

	if !strings.Contains(shows, "entry 01") {
		t.Errorf("paging up does not go back to the first entry:\n%s", shows)
	}

	w.close()
}
