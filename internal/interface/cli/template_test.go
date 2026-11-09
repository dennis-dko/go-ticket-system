package cli

import (
	"strings"
	"testing"
	"time"
)

// twoTemplates answers the templates with one on Consulting and one on the
// archived project, which takes no bookings.
const twoTemplates = `{"items":[{"id":5,"projectId":4,"durationHours":0.75,"description":"stand-up"},` +
	`{"id":6,"projectId":3,"durationHours":1,"description":"old"}],"totalCount":2}`

// templatesOpen opens the templates card's menu once the day is read.
func (w *watched) templatesOpen() string {
	w.t.Helper()

	w.until("the day is read", settled)
	w.press("v")

	return w.until("the templates' menu is open", func(m screen) bool { return settled(m) && m.sub == subTemplates })
}

// A template is kept from what the form asks - the hours, the project and the
// description, all but the day - and sent as the page sends one.
func TestATemplateIsKeptFromWhatTheFormAsks(t *testing.T) {
	f, server := newFakeInstallation(t)

	w := watch(t, server.URL, f.token)
	shows := w.templatesOpen()

	if !offered(shows, "Save a new template", "n") {
		t.Fatalf("saving a template is not offered:\n%s", shows)
	}

	if !strings.Contains(shows, "No templates yet.") {
		t.Errorf("the templates card does not say there are none:\n%s", shows)
	}

	w.press("n")
	w.until("the hours are asked", asking("hours"))
	w.press("1.5", "enter")
	w.until("the project is asked", asking("project"))
	w.press("down", "enter")
	w.until("the description is asked", asking("description"))
	w.press("daily", "enter")

	shows = w.until("the template is saved", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "POST /me/templates")
	if sent["projectId"] != 1.0 || sent["durationHours"] != 1.5 || sent["description"] != "daily" {
		t.Errorf("the template was sent as %v", sent)
	}

	if !strings.Contains(shows, "✓ Saved as a template") {
		t.Errorf("the screen does not say the template was saved:\n%s", shows)
	}

	w.close()
}

// Booking by hand starts with the templates, where there are any it can use, and
// the one chosen fills the form - nothing is booked until the form is answered.
// A template whose project takes no bookings is not offered, as the page does
// not let it be chosen.
func TestBookingIsFilledInFromATemplate(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.answers = map[string]string{"GET /me/templates": twoTemplates}

	w := watch(t, server.URL, f.token)
	w.until("the day is read", settled)
	w.press("b")

	shows := w.until("a template is offered", asking("template"))
	if !strings.Contains(shows, "Consulting · 0.75 h · stand-up") || strings.Contains(shows, "old") {
		t.Errorf("the templates offered are not the usable ones, written as the page writes them:\n%s", shows)
	}

	w.press("down", "enter")
	shows = w.until("the hours are asked", asking("hours"))

	if !strings.Contains(shows, "0.75") {
		t.Errorf("the template did not fill in the hours:\n%s", shows)
	}

	if len(f.sentTo("POST /timesheets")) > 0 {
		t.Fatal("choosing a template booked something")
	}

	w.press("enter")
	w.until("the project is asked", asking("project"))
	w.press("enter")
	w.until("the day is asked", asking("day"))
	w.press("enter")
	w.until("the description is asked", asking("description"))
	w.press("enter")
	w.until("the booking is sent", func(m screen) bool { return settled(m) && m.news != "" })

	sent := sentOnce(t, f, "POST /timesheets")
	if sent["projectId"] != 4.0 || sent["durationHours"] != 0.75 || sent["description"] != "stand-up" {
		t.Errorf("the booking from the template was sent as %v", sent)
	}

	if today := time.Now().In(berlin(t)).Format(time.DateOnly); sent["date"] != today {
		t.Errorf("the booking was sent for %v, want today, %s", sent["date"], today)
	}

	w.close()
}

// A template is deleted once the screen has asked, naming it as the page names
// it; the templates card lists them as the page's card does.
func TestATemplateIsDeletedAfterAsking(t *testing.T) {
	f, server := newFakeInstallation(t)
	f.answers = map[string]string{"GET /me/templates": twoTemplates}

	w := watch(t, server.URL, f.token)
	shows := w.templatesOpen()

	if row := lineWith(shows, "Consulting", "stand-up"); !strings.Contains(row, "0.75") {
		t.Errorf("the templates card does not list a template's project, hours and description:\n%s", shows)
	}

	if !offered(shows, "Delete a template", "d") {
		t.Fatalf("deleting a template is not offered:\n%s", shows)
	}

	w.press("d")
	w.until("a template is to be picked", picking("tplDelete"))
	w.press("enter")

	shows = prose(w.until("the deletion is asked about", asking("confirm")))
	if !strings.Contains(shows, `Template "Consulting · 0.75 h · stand-up" will be deleted. This cannot be undone.`) {
		t.Errorf("the screen does not name the template it asks about:\n%s", shows)
	}

	w.press("y")
	shows = w.until("the template is deleted", func(m screen) bool { return settled(m) && m.news != "" })

	if len(f.sentTo("DELETE /me/templates/5")) != 1 {
		t.Errorf("the template was not deleted once: %v", f.sentTo("DELETE /me/templates/5"))
	}

	if !strings.Contains(shows, "✓ Template deleted") {
		t.Errorf("the screen does not say the template was deleted:\n%s", shows)
	}

	w.close()
}

// berlin is the zone the fake installation's account keeps its days in.
func berlin(t *testing.T) *time.Location {
	t.Helper()

	zone, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	return zone
}
