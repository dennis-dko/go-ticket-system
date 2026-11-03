package cli

import (
	"context"
	"net/http"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
)

// holidayPlaces is where the owner keeps public holidays, as /me/holidays
// answers: their own choice - empty for following the installation, "none" for
// none - the installation's, and the countries a choice can name.
type holidayPlaces struct {
	place, installation string
	countries           []holidayCountry
}

// holidayCountry is a country whose holidays can be kept: its code, the regions
// that keep holidays of their own, and whether one has to be named.
type holidayCountry struct {
	Code           string   `json:"code"`
	Regions        []string `json:"regions"`
	RegionRequired bool     `json:"regionRequired"`
}

// holidayPlaces reads where the owner keeps public holidays.
func (s *session) holidayPlaces(ctx context.Context) (holidayPlaces, error) {
	var answer struct {
		Place         string           `json:"place"`
		InstancePlace string           `json:"instancePlace"`
		Countries     []holidayCountry `json:"countries"`
	}

	if err := s.api.call(ctx, http.MethodGet, "/me/holidays", nil, &answer); err != nil {
		return holidayPlaces{}, err
	}

	return holidayPlaces{place: answer.Place, installation: answer.InstancePlace, countries: answer.Countries}, nil
}

// country is the country with a code, and false where the installation offers
// none by it.
func (p holidayPlaces) country(code string) (holidayCountry, bool) {
	for _, c := range p.countries {
		if c.Code == code {
			return c, true
		}
	}

	return holidayCountry{}, false
}

// placeName writes a choice of public holidays in words: a place as its country
// and region, and what stands in for one as the page says it.
func (l language) placeName(place string) string {
	switch place {
	case "", model.NoHolidays:
		return l.text("holiday.noneKept", "none")
	}

	country, region := model.SplitHolidayPlace(place)
	if region == "" {
		return l.countryName(country)
	}

	return l.countryName(country) + ", " + l.regionName(place)
}

// regionsCalled is what a country calls the parts its holidays differ by, as the
// page's picker names them.
func (l language) regionsCalled(country string) string {
	switch country {
	case "DE":
		return l.text("holiday.region", "State")
	case "GB":
		return l.text("holiday.nation", "Nation")
	}

	return l.text("holiday.area", "Region")
}

// countryName is what a country holidays can be kept in is called, as the page
// calls it.
func (l language) countryName(code string) string {
	switch code {
	case "AT":
		return l.text("country.AT", "Austria")
	case "BE":
		return l.text("country.BE", "Belgium")
	case "CZ":
		return l.text("country.CZ", "Czechia")
	case "DE":
		return l.text("country.DE", "Germany")
	case "DK":
		return l.text("country.DK", "Denmark")
	case "FI":
		return l.text("country.FI", "Finland")
	case "FR":
		return l.text("country.FR", "France")
	case "GB":
		return l.text("country.GB", "United Kingdom")
	case "IE":
		return l.text("country.IE", "Ireland")
	case "IT":
		return l.text("country.IT", "Italy")
	case "LU":
		return l.text("country.LU", "Luxembourg")
	case "NL":
		return l.text("country.NL", "Netherlands")
	case "NO":
		return l.text("country.NO", "Norway")
	case "PL":
		return l.text("country.PL", "Poland")
	case "PT":
		return l.text("country.PT", "Portugal")
	case "SE":
		return l.text("country.SE", "Sweden")
	case "US":
		return l.text("country.US", "United States")
	}

	return code
}

// regionName is what a region that keeps holidays of its own is called, by its
// place, as the page calls it.
func (l language) regionName(place string) string {
	switch place {
	case "DE-BW":
		return l.text("region.DE-BW", "Baden-Wuerttemberg")
	case "DE-BY":
		return l.text("region.DE-BY", "Bavaria")
	case "DE-BE":
		return l.text("region.DE-BE", "Berlin")
	case "DE-BB":
		return l.text("region.DE-BB", "Brandenburg")
	case "DE-HB":
		return l.text("region.DE-HB", "Bremen")
	case "DE-HH":
		return l.text("region.DE-HH", "Hamburg")
	case "DE-HE":
		return l.text("region.DE-HE", "Hesse")
	case "DE-MV":
		return l.text("region.DE-MV", "Mecklenburg-Western Pomerania")
	case "DE-NI":
		return l.text("region.DE-NI", "Lower Saxony")
	case "DE-NW":
		return l.text("region.DE-NW", "North Rhine-Westphalia")
	case "DE-RP":
		return l.text("region.DE-RP", "Rhineland-Palatinate")
	case "DE-SL":
		return l.text("region.DE-SL", "Saarland")
	case "DE-SN":
		return l.text("region.DE-SN", "Saxony")
	case "DE-ST":
		return l.text("region.DE-ST", "Saxony-Anhalt")
	case "DE-SH":
		return l.text("region.DE-SH", "Schleswig-Holstein")
	case "DE-TH":
		return l.text("region.DE-TH", "Thuringia")
	case "FR-6AE":
		return l.text("region.FR-6AE", "Alsace")
	case "FR-57":
		return l.text("region.FR-57", "Moselle")
	case "GB-ENG":
		return l.text("region.GB-ENG", "England")
	case "GB-WLS":
		return l.text("region.GB-WLS", "Wales")
	case "GB-SCT":
		return l.text("region.GB-SCT", "Scotland")
	case "GB-NIR":
		return l.text("region.GB-NIR", "Northern Ireland")
	case "IT-BZ":
		return l.text("region.IT-BZ", "South Tyrol")
	case "PT-20":
		return l.text("region.PT-20", "Azores")
	case "PT-30":
		return l.text("region.PT-30", "Madeira")
	}

	return place
}
