//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

// installationHolidaysOnTheWire is what GET /settings/holidays answers.
type installationHolidaysOnTheWire struct {
	Place     string `json:"place"`
	Countries []struct {
		Code           string   `json:"code"`
		Regions        []string `json:"regions"`
		RegionRequired bool     `json:"regionRequired"`
	} `json:"countries"`
}

func installationHolidays(t *testing.T, c *client) installationHolidaysOnTheWire {
	t.Helper()

	var answer installationHolidaysOnTheWire

	c.must(c.api(http.MethodGet, "/settings/holidays", nil), http.StatusOK).Data(t, &answer)

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

// The installation keeps the public holidays of one place, which its service
// hours leave out. It is chosen as a country and, where the country's holidays
// differ by region, the region - and read back with every country it could have
// chosen instead.
func TestTheInstallationsHolidaysAreChosenAndReadBack(t *testing.T) {
	t.Parallel()

	_, admin, _ := startWithWorker(t)

	if fresh := installationHolidays(t, admin); fresh.Place != "" {
		t.Errorf("a fresh installation keeps the holidays of %q; nobody has chosen any", fresh.Place)
	}

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "DE-BY"}), http.StatusOK)

	chosen := installationHolidays(t, admin)

	if chosen.Place != "DE-BY" {
		t.Errorf("the installation reads %q, want Bavaria", chosen.Place)
	}

	offered := map[string]bool{}

	for _, country := range chosen.Countries {
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
}

// The installation can say it keeps none as a choice of its own, told apart from
// nobody having chosen.
func TestTheInstallationCanChooseNoHolidays(t *testing.T) {
	t.Parallel()

	_, admin, _ := startWithWorker(t)

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "none"}), http.StatusOK)

	if chosen := installationHolidays(t, admin); chosen.Place != "none" {
		t.Errorf("an installation that chose none reads %q", chosen.Place)
	}

	admin.must(admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": ""}), http.StatusOK)

	if cleared := installationHolidays(t, admin); cleared.Place != "" {
		t.Errorf("an installation whose choice was taken back reads %q", cleared.Place)
	}
}

// What cannot be chosen is refused, naming the half to change - the country
// where it is unknown, the region where the country needs one or has no such
// one - and the installation's holidays by somebody who does not administer it.
func TestAHolidayPlaceThatCannotBeChosenIsRefused(t *testing.T) {
	t.Parallel()

	_, admin, worker := startWithWorker(t)

	for _, refused := range []struct {
		place string
		field string
	}{
		{"XX", "country"},
		{"XX-BY", "country"},
		{"GB", "region"},
		{"DE", "region"},
		{"FR-75", "region"},
	} {
		answer := admin.api(http.MethodPut, "/settings/holidays", map[string]any{"place": refused.place})

		if status, fields := refusedFields(answer); status != http.StatusBadRequest ||
			!slices.Equal(fields, []string{refused.field}) {
			t.Errorf("%q answered %d naming %v, want 400 naming %s",
				refused.place, status, fields, refused.field)
		}
	}

	if refused := worker.api(http.MethodPut, "/settings/holidays", map[string]any{"place": "DE-BY"}); refused.Status != http.StatusForbidden {
		t.Errorf("somebody who does not administer set the installation's holidays: %d, want 403", refused.Status)
	}
}
