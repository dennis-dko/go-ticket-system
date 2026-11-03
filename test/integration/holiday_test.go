//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

type myHolidaysOnTheWire struct {
	Place          string `json:"place"`
	InstancePlace  string `json:"instancePlace"`
	EffectivePlace string `json:"effectivePlace"`
	Countries      []struct {
		Code           string   `json:"code"`
		Regions        []string `json:"regions"`
		RegionRequired bool     `json:"regionRequired"`
	} `json:"countries"`

	// What a client written before there were countries reads.
	Region          string   `json:"region"`
	InstanceRegion  string   `json:"instanceRegion"`
	EffectiveRegion string   `json:"effectiveRegion"`
	Regions         []string `json:"regions"`
}

type installationHolidaysOnTheWire struct {
	Place   string   `json:"place"`
	Region  string   `json:"region"`
	Regions []string `json:"regions"`
}

func holidaysOf(t *testing.T, c *client) myHolidaysOnTheWire {
	t.Helper()

	var answer myHolidaysOnTheWire

	c.must(c.api(http.MethodGet, "/me/holidays", nil), http.StatusOK).Data(t, &answer)

	return answer
}

// refusedFields is what a refusal names, and its status.
func refusedFields(answer response) (int, []string) {
	var body struct {
		Error struct {
			Param []string `json:"param"`
		} `json:"error"`
	}

	_ = json.Unmarshal(answer.Body, &body)

	return answer.Status, body.Error.Param
}

// An installation keeps no holidays until somebody chooses a state. Once it has
// one, everybody following it keeps that state's: the days name them, and leave
// over Whit week is four days of it.
//
// Chosen and read the way a client written before there were countries does,
// by a German state's code alone, which is the contract such a client has.
func TestTheInstallationsHolidaysAreEverybodys(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	if fresh := holidaysOf(t, worker); fresh.EffectiveRegion != "" || len(fresh.Regions) != 16 {
		t.Errorf("a fresh installation reads %+v, want no holidays and the sixteen states to choose from", fresh)
	}

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"region": "BY"}), http.StatusOK)

	if mine := holidaysOf(t, worker); mine.Region != "" || mine.EffectiveRegion != "BY" || mine.EffectivePlace != "DE-BY" {
		t.Errorf("following a Bavarian installation reads %+v, want Bavaria in effect", mine)
	}

	var week daysOnTheWire

	// 25 May 2026 is Whit Monday.
	worker.must(worker.api(http.MethodGet, "/me/days?from=2026-05-25&to=2026-05-31", nil),
		http.StatusOK).Data(t, &week)

	if week.Days[0].Holiday != "whitMonday" || week.Days[1].Holiday != "" {
		t.Errorf("Whit week reads %+v, want the Monday named and the Tuesday not", week.Days[:2])
	}

	var recorded listOf[absenceOnTheWire]

	worker.must(worker.api(http.MethodPost, "/me/absences", map[string]any{
		"from": "2026-05-25", "to": "2026-05-29", "kind": "vacation",
	}), http.StatusCreated, http.StatusOK).Data(t, &recorded)

	if recorded.TotalCount != 4 {
		t.Errorf("Whit week was recorded as %d days of leave, want the four that are not Whit Monday", recorded.TotalCount)
	}
}

// A person's own choice wins: none at all, or back to following the
// installation - said, again, the way it was said before there were countries.
func TestAPersonChoosesTheirOwnHolidays(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"region": "BY"}), http.StatusOK)
	worker.must(worker.api(http.MethodPut, "/me/holidays", map[string]any{"region": "none"}), http.StatusOK)

	if mine := holidaysOf(t, worker); mine.Region != "none" || mine.EffectiveRegion != "" || mine.InstanceRegion != "BY" {
		t.Errorf("keeping none in a Bavarian installation reads %+v", mine)
	}

	worker.must(worker.api(http.MethodPut, "/me/holidays", map[string]any{"region": ""}), http.StatusOK)

	if mine := holidaysOf(t, worker); mine.EffectiveRegion != "BY" {
		t.Errorf("following the installation again reads %+v, want Bavaria", mine)
	}
}

// BE was Berlin before there were countries and is Belgium now, so a choice
// written the old way is read the old way: a client from then that asks for BE
// keeps Berlin's holidays, and is answered BE again rather than DE-BE.
func TestAStateWrittenTheOldWayIsAGermanState(t *testing.T) {
	t.Parallel()

	_, _, worker := startWithWorker(t)

	worker.must(worker.api(http.MethodPut, "/me/holidays", map[string]any{"region": "BE"}), http.StatusOK)

	if mine := holidaysOf(t, worker); mine.Place != "DE-BE" || mine.Region != "BE" {
		t.Errorf("BE written the old way reads %+v, want Berlin", mine)
	}

	worker.must(worker.api(http.MethodPut, "/me/holidays", map[string]any{"place": "BE"}), http.StatusOK)

	if mine := holidaysOf(t, worker); mine.Place != "BE" || mine.EffectivePlace != "BE" {
		t.Errorf("BE written as a place reads %+v, want Belgium", mine)
	}
}

// A place in another country is kept like a state: everybody following an
// Austrian installation keeps Austria's National Day, which no German state has,
// and is offered every country with the regions it has.
func TestAnInstallationInAnotherCountryKeepsThatCountrysHolidays(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "AT"}), http.StatusOK)

	var installation installationHolidaysOnTheWire

	admin.must(admin.api(http.MethodGet, "/settings/holidays", nil), http.StatusOK).Data(t, &installation)

	if installation.Place != "AT" || installation.Region != "AT" {
		t.Errorf("the installation reads %+v, want Austria", installation)
	}

	mine := holidaysOf(t, worker)

	if mine.InstancePlace != "AT" || mine.EffectivePlace != "AT" || mine.Place != "" {
		t.Errorf("following an Austrian installation reads %+v", mine)
	}

	offered := map[string]bool{}

	for _, country := range mine.Countries {
		offered[country.Code] = country.RegionRequired

		if country.Code == "DE" && len(country.Regions) != 16 {
			t.Errorf("Germany is offered with %v", country.Regions)
		}

		if country.Regions == nil {
			t.Errorf("%s is offered with its regions as null rather than none", country.Code)
		}
	}

	if required, ok := offered["DE"]; !ok || !required {
		t.Errorf("Germany is offered %v, needing a state %v", ok, required)
	}

	if required, ok := offered["AT"]; !ok || required {
		t.Errorf("Austria is offered %v, needing a region %v", ok, required)
	}

	var week daysOnTheWire

	// 26 October 2026, a Monday, is Austria's National Day.
	worker.must(worker.api(http.MethodGet, "/me/days?from=2026-10-26&to=2026-10-27", nil),
		http.StatusOK).Data(t, &week)

	if week.Days[0].Holiday != "nationalDay" || week.Days[1].Holiday != "" {
		t.Errorf("the 26th of October in Austria reads %+v, want National Day", week.Days)
	}
}

// The installation can say it keeps none as a choice of its own, told apart from
// nobody having chosen: both keep none, and a client from before countries reads
// both as the empty choice it knew.
func TestTheInstallationCanChooseNoHolidays(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "none"}), http.StatusOK)

	var installation installationHolidaysOnTheWire

	admin.must(admin.api(http.MethodGet, "/settings/holidays", nil), http.StatusOK).Data(t, &installation)

	if installation.Place != "none" || installation.Region != "" {
		t.Errorf("an installation that chose none reads %+v", installation)
	}

	if mine := holidaysOf(t, worker); mine.EffectivePlace != "" || mine.InstancePlace != "none" || mine.InstanceRegion != "" {
		t.Errorf("following an installation that chose none reads %+v", mine)
	}
}

// What cannot be chosen is refused, naming the half to change - the country
// where it is unknown, the region where the country needs one or has no such
// one - and the installation's holidays by somebody who does not administer it.
func TestAHolidayPlaceThatCannotBeChosenIsRefused(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	for _, refused := range []struct {
		answer response
		field  string
	}{
		{worker.api(http.MethodPut, "/me/holidays", map[string]any{"region": "XX"}), "country"},
		{worker.api(http.MethodPut, "/me/holidays", map[string]any{"place": "XX-BY"}), "country"},
		{worker.api(http.MethodPut, "/me/holidays", map[string]any{"place": "GB"}), "region"},
		{admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "DE"}), "region"},
		{admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "FR-75"}), "region"},
	} {
		if status, fields := refusedFields(refused.answer); status != http.StatusBadRequest ||
			!slices.Equal(fields, []string{refused.field}) {
			t.Errorf("a place that cannot be chosen answered %d naming %v, want 400 naming %s",
				status, fields, refused.field)
		}
	}

	if refused := worker.api(http.MethodPut, "/settings/holidays", map[string]any{"region": "BY"}); refused.Status != http.StatusForbidden {
		t.Errorf("somebody who does not administer set the installation's holidays: %d, want 403", refused.Status)
	}
}
