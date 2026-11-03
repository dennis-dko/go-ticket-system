//go:build browser

package browser

import (
	"fmt"
	"testing"

	"github.com/chromedp/chromedp"
)

// pickHoliday chooses a value in one of a holiday card's pickers the way a person
// does, with the change event the card listens for - chooseOption sets the value
// once it is there to set, which fires nothing.
func (p *page) pickHoliday(selector, value string) chromedp.Action {
	return chromedp.Tasks{
		p.chooseOption(selector, value),
		chromedp.Evaluate(fmt.Sprintf(
			`document.querySelector(%q).dispatchEvent(new Event('change', { bubbles: true }))`, selector), nil),
	}
}

// regionPicker is what a holiday card's second picker shows: whether it is out,
// what it is called, and whether it has to be answered.
type regionPicker struct {
	Shown    bool   `json:"shown"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

func (p *page) regionPicker(selector string) regionPicker {
	p.t.Helper()

	var picker regionPicker

	p.run("read "+selector, chromedp.Evaluate(fmt.Sprintf(`(() => {
		const select = document.querySelector(%q);
		const label = select.closest('label');

		return { shown: !label.hidden, label: label.firstChild.textContent.trim(), required: select.required };
	})()`, selector), &picker))

	return picker
}

// A state chosen under "My account" puts its holidays in the calendar, by name:
// Germany first, and then Bavaria from the second picker Germany brings out.
func TestAChosenStatesHolidaysAreMarkedInTheCalendar(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.run("open my account", p.click(`.tab[data-view="settings"]`),
		chromedp.WaitVisible("#form-my-holidays", chromedp.ByID))

	p.waitForNode(`#my-holiday-country option[value="DE"]`)

	if picker := p.regionPicker("#my-holidays"); picker.Shown {
		t.Errorf("following the installation shows a second picker: %+v", picker)
	}

	p.run("choose Germany", p.pickHoliday("#my-holiday-country", "DE"))

	if picker := p.regionPicker("#my-holidays"); !picker.Shown || picker.Label != "State" || !picker.Required {
		t.Errorf("Germany brings out %+v, want a state that has to be chosen", picker)
	}

	p.run("choose Bavaria", p.pickHoliday("#my-holidays", "BY"))
	p.run("save it", p.click(`#form-my-holidays button[type="submit"]`))

	p.waitFor("Bavaria to be in effect", func() bool {
		var place string

		p.awaitJSON(`api('/me/holidays').then((mine) => mine.effectivePlace)`, &place)

		return place == "DE-BY"
	})

	p.run("open December", p.click(`.tab[data-view="calendar"]`),
		chromedp.WaitVisible("#calendar-days", chromedp.ByID),
		chromedp.Evaluate(`calendarMonth = monthOf('2026-12-01'); void loadCalendar()`, nil))

	p.waitForText("#calendar-days", "Christmas Day")
	p.waitForText("#calendar-days", "Boxing Day")
}

// The installation's card starts on Germany with the state still to choose,
// because nobody having chosen keeps none and Germany is where this application's
// own users keep their holidays. The second picker takes the word each country
// uses for its parts, has to be answered where they divide it, and is put away
// for a country without any.
func TestTheInstallationsHolidaysStartOnGermany(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyAdmin()

	p.run("open Settings", p.click(`.tab[data-view="admin"]`),
		chromedp.WaitVisible("#form-holidays", chromedp.ByID))

	p.waitForNode(`#instance-holidays option[value="BY"]`)

	if country, state := p.value("#instance-holiday-country"), p.value("#instance-holidays"); country != "DE" || state != "" {
		t.Errorf("a fresh installation's card stands on %q and %q, want Germany and no state yet", country, state)
	}

	if picker := p.regionPicker("#instance-holidays"); !picker.Shown || picker.Label != "State" || !picker.Required {
		t.Errorf("Germany's second picker is %+v", picker)
	}

	p.run("choose the United Kingdom", p.pickHoliday("#instance-holiday-country", "GB"))

	if picker := p.regionPicker("#instance-holidays"); !picker.Shown || picker.Label != "Nation" || !picker.Required {
		t.Errorf("the United Kingdom's second picker is %+v, want a nation that has to be chosen", picker)
	}

	p.run("choose France", p.pickHoliday("#instance-holiday-country", "FR"))

	if picker := p.regionPicker("#instance-holidays"); !picker.Shown || picker.Label != "Region" || picker.Required {
		t.Errorf("France's second picker is %+v, want a region that may be left on everywhere else", picker)
	}

	p.run("choose Austria", p.pickHoliday("#instance-holiday-country", "AT"))

	if picker := p.regionPicker("#instance-holidays"); picker.Shown || picker.Required {
		t.Errorf("Austria, which has no regions, shows %+v", picker)
	}

	p.run("save it", p.click(`#form-holidays button[type="submit"]`))

	p.waitFor("Austria to be the installation's", func() bool {
		var place string

		p.awaitJSON(`api('/settings/holidays').then((holidays) => holidays.place)`, &place)

		return place == "AT"
	})
}

// A holiday a country keeps on a weekday because it fell on a weekend says so in
// the calendar: Christmas 2027 is a Saturday, and Scotland keeps it on the
// Monday.
func TestADayKeptInPlaceOfAHolidaySaysSoInTheCalendar(t *testing.T) {
	t.Parallel()

	p := open(t)
	p.readyWorker()

	p.awaitJSON(`api('/me/holidays', { method: 'PUT', body: JSON.stringify({ place: 'GB-SCT' }) })`, nil)

	p.run("open December 2027", p.click(`.tab[data-view="calendar"]`),
		chromedp.WaitVisible("#calendar-days", chromedp.ByID),
		chromedp.Evaluate(`calendarMonth = monthOf('2027-12-01'); void loadCalendar()`, nil))

	p.waitForText("#calendar-days", "Christmas Day (substitute day)")
}
