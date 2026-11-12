package model_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dennis-dko/go-ticket-system/internal/domain/model"
)

// dayNames writes holidays as their day and name, for comparing with a list read
// off a calendar - a substitute day with a star after its name.
func dayNames(holidays []model.Holiday) []string {
	written := make([]string, 0, len(holidays))
	for _, holiday := range holidays {
		name := holiday.Day.Format(time.DateOnly) + " " + holiday.Name
		if holiday.Substitute {
			name += "*"
		}

		written = append(written, name)
	}

	return written
}

// year writes a year's holidays the way the cases below list them: the month and
// day, and the name.
func year(place string, y int) string {
	var written []string

	for _, holiday := range dayNames(model.HolidaysIn(place, y)) {
		written = append(written, strings.TrimPrefix(holiday, strings.Split(holiday, "-")[0]+"-"))
	}

	return strings.Join(written, ", ")
}

// Easter, through the one state that keeps Easter Sunday itself as a holiday,
// against the dates in a calendar.
func TestEasterFallsOnItsDay(t *testing.T) {
	for y, want := range map[int]string{
		2000: "2000-04-23", 2019: "2019-04-21", 2024: "2024-03-31", 2025: "2025-04-20",
		2026: "2026-04-05", 2027: "2027-03-28", 2038: "2038-04-25",
	} {
		found := slices.IndexFunc(model.HolidaysIn("DE-BB", y),
			func(h model.Holiday) bool { return h.Name == "easterSunday" })

		if found < 0 {
			t.Errorf("%d has no Easter Sunday in Brandenburg", y)

			continue
		}

		if got := model.HolidaysIn("DE-BB", y)[found].Day.Format(time.DateOnly); got != want {
			t.Errorf("Easter %d fell on %s, want %s", y, got, want)
		}
	}
}

// A whole year of two states, as their calendars list it.
func TestAStatesYearIsItsCalendars(t *testing.T) {
	for _, tc := range []struct {
		place string
		year  int
		want  []string
	}{
		{"DE-BY", 2026, []string{
			"2026-01-01 newYear", "2026-01-06 epiphany", "2026-04-03 goodFriday",
			"2026-04-06 easterMonday", "2026-05-01 labourDay", "2026-05-14 ascension",
			"2026-05-25 whitMonday", "2026-06-04 corpusChristi", "2026-10-03 germanUnity",
			"2026-11-01 allSaints", "2026-12-25 christmasDay", "2026-12-26 stStephensDay",
		}},
		{"DE-BE", 2025, []string{
			"2025-01-01 newYear", "2025-03-08 womensDay", "2025-04-18 goodFriday",
			"2025-04-21 easterMonday", "2025-05-01 labourDay", "2025-05-08 liberationDay",
			"2025-05-29 ascension", "2025-06-09 whitMonday", "2025-10-03 germanUnity",
			"2025-12-25 christmasDay", "2025-12-26 stStephensDay",
		}},
	} {
		if got := dayNames(model.HolidaysIn(tc.place, tc.year)); !slices.Equal(got, tc.want) {
			t.Errorf("%s %d is\n%v\nwant\n%v", tc.place, tc.year, got, tc.want)
		}
	}
}

// A holiday holds from the year it was introduced, until the year it was
// abolished, and once in a year it was given once - and a state that keeps it
// anyway does not get it twice.
func TestAHolidayHoldsInItsYearsOnly(t *testing.T) {
	has := func(place string, y int, name string) bool {
		return slices.ContainsFunc(model.HolidaysIn(place, y), func(h model.Holiday) bool { return h.Name == name })
	}

	for _, tc := range []struct {
		place, name string
		year        int
		want        bool
	}{
		{"DE-BE", "womensDay", 2018, false},
		{"DE-BE", "womensDay", 2019, true},
		{"DE-MV", "womensDay", 2022, false},
		{"DE-MV", "womensDay", 2023, true},
		{"DE-BE", "liberationDay", 2020, true},
		{"DE-BE", "liberationDay", 2021, false},
		{"DE-BE", "uprisingDay", 2028, true},
		{"DE-BE", "uprisingDay", 2027, false},
		{"DE-NI", "reformationDay", 2017, true},
		{"DE-NI", "reformationDay", 2016, false},
		{"DE-NI", "reformationDay", 2018, true},
		{"DE-BY", "reformationDay", 2017, true},
		{"DE-BY", "reformationDay", 2018, false},
		{"DE-HE", "repentanceDay", 1994, true},
		{"DE-HE", "repentanceDay", 1995, false},
		{"DE-SN", "repentanceDay", 1995, true},
		{"DE-TH", "childrensDay", 2018, false},
		{"DE-TH", "childrensDay", 2019, true},
	} {
		if got := has(tc.place, tc.year, tc.name); got != tc.want {
			t.Errorf("%s keeps %s in %d: %v, want %v", tc.place, tc.name, tc.year, got, tc.want)
		}
	}

	// Saxony keeps the Reformation every year, and 2017 gave it to everybody: once.
	reformations := 0

	for _, holiday := range model.HolidaysIn("DE-SN", 2017) {
		if holiday.Name == "reformationDay" {
			reformations++
		}
	}

	if reformations != 1 {
		t.Errorf("Saxony has %d Reformation Days in 2017, want one", reformations)
	}

	// The Wednesday before the 23rd of November.
	for _, holiday := range model.HolidaysIn("DE-SN", 2026) {
		if holiday.Name == "repentanceDay" && holiday.Day.Format(time.DateOnly) != "2026-11-18" {
			t.Errorf("the Day of Repentance 2026 fell on %s, want 2026-11-18", holiday.Day.Format(time.DateOnly))
		}
	}
}

// A year in every country, as its law has it - each checked against an
// independent list of public holidays, and against gov.uk's own for Britain,
// where they differ on a point the law settles the comment of the country's file
// says which way and why. The years are picked for what they hold: a holiday
// abolished or moved, a substitute, a day given once.
func TestEachCountrysYearIsItsLaws(t *testing.T) {
	for _, tc := range []struct {
		place string
		year  int
		want  string
	}{
		{"AT", 2026, "01-01 newYear, 01-06 epiphany, 04-05 easterSunday, 04-06 easterMonday, 05-01 labourDay, " +
			"05-14 ascension, 05-24 whitSunday, 05-25 whitMonday, 06-04 corpusChristi, 08-15 assumption, " +
			"10-26 nationalDay, 11-01 allSaints, 12-08 immaculateConception, 12-25 christmasDay, 12-26 stStephensDay"},
		{"BE", 2026, "01-01 newYear, 04-06 easterMonday, 05-01 labourDay, 05-14 ascension, 05-25 whitMonday, " +
			"07-21 nationalDay, 08-15 assumption, 11-01 allSaints, 11-11 armisticeDay, 12-25 christmasDay"},
		{"CZ", 2026, "01-01 newYear, 04-03 goodFriday, 04-06 easterMonday, 05-01 labourDay, 05-08 liberationDay, " +
			"07-05 cyrilAndMethodiusDay, 07-06 janHusDay, 09-28 statehoodDay, 10-28 independenceDay, " +
			"11-17 freedomAndDemocracyDay, 12-24 christmasEve, 12-25 christmasDay, 12-26 stStephensDay"},
		{"DK", 2023, "01-01 newYear, 04-06 maundyThursday, 04-07 goodFriday, 04-09 easterSunday, 04-10 easterMonday, " +
			"05-05 greatPrayerDay, 05-18 ascension, 05-28 whitSunday, 05-29 whitMonday, 12-25 christmasDay, 12-26 stStephensDay"},
		{"DK", 2024, "01-01 newYear, 03-28 maundyThursday, 03-29 goodFriday, 03-31 easterSunday, 04-01 easterMonday, " +
			"05-09 ascension, 05-19 whitSunday, 05-20 whitMonday, 12-25 christmasDay, 12-26 stStephensDay"},
		{"FI", 2026, "01-01 newYear, 01-06 epiphany, 04-03 goodFriday, 04-05 easterSunday, 04-06 easterMonday, " +
			"05-01 labourDay, 05-14 ascension, 05-24 whitSunday, 06-19 midsummerEve, 06-20 midsummerDay, " +
			"10-31 allSaints, 12-06 independenceDay, 12-24 christmasEve, 12-25 christmasDay, 12-26 stStephensDay"},
		{"FR", 2026, "01-01 newYear, 04-06 easterMonday, 05-01 labourDay, 05-08 victoryInEurope, 05-14 ascension, " +
			"05-25 whitMonday, 07-14 nationalDay, 08-15 assumption, 11-01 allSaints, 11-11 armisticeDay, 12-25 christmasDay"},
		{"FR-57", 2026, "01-01 newYear, 04-03 goodFriday, 04-06 easterMonday, 05-01 labourDay, 05-08 victoryInEurope, " +
			"05-14 ascension, 05-25 whitMonday, 07-14 nationalDay, 08-15 assumption, 11-01 allSaints, " +
			"11-11 armisticeDay, 12-25 christmasDay, 12-26 stStephensDay"},
		{"GB-ENG", 2022, "01-01 newYear, 01-03 newYear*, 04-15 goodFriday, 04-18 easterMonday, " +
			"05-02 earlyMayBankHoliday, 06-02 springBankHoliday, 06-03 platinumJubilee, 08-29 summerBankHoliday, " +
			"09-19 stateFuneral, 12-25 christmasDay, 12-26 stStephensDay, 12-27 christmasDay*"},
		{"GB-SCT", 2026, "01-01 newYear, 01-02 secondJanuary, 04-03 goodFriday, 05-04 earlyMayBankHoliday, " +
			"05-25 springBankHoliday, 06-15 worldCup, 08-03 summerBankHoliday, 11-30 stAndrewsDay, " +
			"12-25 christmasDay, 12-26 stStephensDay, 12-28 stStephensDay*"},
		{"GB-NIR", 2026, "01-01 newYear, 03-17 stPatricksDay, 04-03 goodFriday, 04-06 easterMonday, " +
			"05-04 earlyMayBankHoliday, 05-25 springBankHoliday, 07-12 battleOfTheBoyne, 07-13 battleOfTheBoyne*, " +
			"08-31 summerBankHoliday, 12-25 christmasDay, 12-26 stStephensDay, 12-28 stStephensDay*"},
		{"IE", 2026, "01-01 newYear, 02-02 stBrigidsDay, 03-17 stPatricksDay, 04-06 easterMonday, " +
			"05-04 mayBankHoliday, 06-01 juneBankHoliday, 08-03 augustBankHoliday, 10-26 octoberBankHoliday, " +
			"12-25 christmasDay, 12-26 stStephensDay"},
		{"IT-BZ", 2026, "01-01 newYear, 01-06 epiphany, 04-05 easterSunday, 04-06 easterMonday, 04-25 liberationDay, " +
			"05-01 labourDay, 05-25 whitMonday, 06-02 republicDay, 08-15 assumption, 10-04 stFrancisDay, " +
			"11-01 allSaints, 12-08 immaculateConception, 12-25 christmasDay, 12-26 stStephensDay"},
		{"LU", 2026, "01-01 newYear, 04-06 easterMonday, 05-01 labourDay, 05-09 europeDay, 05-14 ascension, " +
			"05-25 whitMonday, 06-23 nationalDay, 08-15 assumption, 11-01 allSaints, 12-25 christmasDay, 12-26 stStephensDay"},
		{"NL", 2025, "01-01 newYear, 04-20 easterSunday, 04-21 easterMonday, 04-26 kingsDay, 05-29 ascension, " +
			"06-08 whitSunday, 06-09 whitMonday, 12-25 christmasDay, 12-26 stStephensDay"},
		{"NO", 2026, "01-01 newYear, 04-02 maundyThursday, 04-03 goodFriday, 04-05 easterSunday, 04-06 easterMonday, " +
			"05-01 labourDay, 05-14 ascension, 05-17 constitutionDay, 05-24 whitSunday, 05-25 whitMonday, " +
			"12-25 christmasDay, 12-26 stStephensDay"},
		{"PL", 2026, "01-01 newYear, 01-06 epiphany, 04-05 easterSunday, 04-06 easterMonday, 05-01 labourDay, " +
			"05-03 constitutionDay, 05-24 whitSunday, 06-04 corpusChristi, 08-15 assumption, 11-01 allSaints, " +
			"11-11 independenceDay, 12-24 christmasEve, 12-25 christmasDay, 12-26 stStephensDay"},
		{"PT-20", 2026, "01-01 newYear, 04-03 goodFriday, 04-05 easterSunday, 04-25 freedomDay, 05-01 labourDay, " +
			"05-25 azoresDay, 06-04 corpusChristi, 06-10 nationalDay, 08-15 assumption, 10-05 republicDay, " +
			"11-01 allSaints, 12-01 restorationOfIndependence, 12-08 immaculateConception, 12-25 christmasDay"},
		{"PT", 2014, "01-01 newYear, 04-18 goodFriday, 04-20 easterSunday, 04-25 freedomDay, 05-01 labourDay, " +
			"06-10 nationalDay, 08-15 assumption, 12-08 immaculateConception, 12-25 christmasDay"},
		{"SE", 2026, "01-01 newYear, 01-06 epiphany, 04-03 goodFriday, 04-05 easterSunday, 04-06 easterMonday, " +
			"05-01 labourDay, 05-14 ascension, 05-24 whitSunday, 06-06 nationalDay, 06-19 midsummerEve, " +
			"06-20 midsummerDay, 10-31 allSaints, 12-24 christmasEve, 12-25 christmasDay, 12-26 stStephensDay, " +
			"12-31 newYearsEve"},
		{"US", 2026, "01-01 newYear, 01-19 martinLutherKingDay, 02-16 washingtonsBirthday, 05-25 memorialDay, " +
			"06-19 juneteenth, 07-03 independenceDay*, 07-04 independenceDay, 09-07 labourDay, 10-12 columbusDay, " +
			"11-11 veteransDay, 11-26 thanksgiving, 12-25 christmasDay"},
	} {
		if got := year(tc.place, tc.year); got != tc.want {
			t.Errorf("%s %d is\n%s\nwant\n%s", tc.place, tc.year, got, tc.want)
		}
	}
}

// A substitute can fall in another year than the holiday it stands in for, and
// it is that year's: the United States kept New Year's Day 2022, a Saturday, on
// Friday 31 December 2021.
func TestASubstituteBelongsToTheYearItFallsIn(t *testing.T) {
	if got := dayNames(model.HolidaysIn("US", 2021)); !slices.Contains(got, "2021-12-31 newYear*") {
		t.Errorf("2021 in the United States does not end with New Year's Day kept: %v", got)
	}

	if got := dayNames(model.HolidaysIn("US", 2022)); slices.Contains(got, "2021-12-31 newYear*") {
		t.Errorf("2022 in the United States lists a day of 2021: %v", got)
	}
}

// Ireland keeps St Brigid's Day on the first Monday of February, or on the 1st
// where that is a Friday.
func TestStBrigidsDayIsAMondayOrAFriday(t *testing.T) {
	for y, want := range map[int]string{2023: "2023-02-06", 2026: "2026-02-02", 2027: "2027-02-01", 2030: "2030-02-01"} {
		found := slices.IndexFunc(model.HolidaysIn("IE", y), func(h model.Holiday) bool { return h.Name == "stBrigidsDay" })

		if found < 0 || model.HolidaysIn("IE", y)[found].Day.Format(time.DateOnly) != want {
			t.Errorf("St Brigid's Day %d is not on %s: %v", y, want, dayNames(model.HolidaysIn("IE", y)))
		}
	}
}

// A place this application does not know is refused rather than answered with
// none: a country it has no rules for, a region a country does not have, and a
// country whose regions divide it, named without one. Empty and NoHolidays are
// choices rather than places, and have no holidays either.
func TestAPlaceTheRulesDoNotKnowIsRefused(t *testing.T) {
	for _, place := range []string{"DE-HH", "AT", "FR", "FR-57", "GB-SCT", "IT-BZ", "PT-30"} {
		if !model.ValidHolidayPlace(place) || !model.ValidHolidayChoice(place) {
			t.Errorf("%q is refused", place)
		}
	}

	for _, place := range []string{"", model.NoHolidays, "XX", "DE", "DE-XX", "GB", "AT-1", "FR-", "de-hh", "-HH"} {
		if model.ValidHolidayPlace(place) {
			t.Errorf("%q is taken for a place", place)
		}

		if got := model.HolidaysIn(place, 2026); len(got) != 0 {
			t.Errorf("%q, which is no place, has %v", place, dayNames(got))
		}
	}

	// A refusal says which half to change: the country where it is unknown, the
	// region where the country is known.
	for choice, fault := range map[string]model.HolidayChoiceFault{
		"": model.NoHolidayChoiceFault, model.NoHolidays: model.NoHolidayChoiceFault,
		"AT": model.NoHolidayChoiceFault, "DE-BE": model.NoHolidayChoiceFault,
		"XX": model.UnknownHolidayCountry, "XX-BY": model.UnknownHolidayCountry,
		"de-hh": model.UnknownHolidayCountry, "-HH": model.UnknownHolidayCountry,
		"DE": model.UnknownHolidayRegion, "DE-XX": model.UnknownHolidayRegion, "AT-1": model.UnknownHolidayRegion,
		"FR-": model.UnknownHolidayRegion, "GB": model.UnknownHolidayRegion,
	} {
		if got := model.HolidayChoiceFaultOf(choice); got != fault {
			t.Errorf("%q has the fault %d, want %d", choice, got, fault)
		}

		if model.ValidHolidayChoice(choice) != (fault == model.NoHolidayChoiceFault) {
			t.Errorf("%q may be stored: %v", choice, fault != model.NoHolidayChoiceFault)
		}
	}
}

// A range takes its holidays from every year it touches, and only from inside it.
func TestHolidaysBetweenTwoDaysSpanTheYearsBetween(t *testing.T) {
	found := model.HolidaysBetween("DE-BY",
		time.Date(2025, time.December, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.January, 6, 0, 0, 0, 0, time.UTC))

	var days []string
	for day := range found {
		days = append(days, day.Format(time.DateOnly))
	}

	slices.Sort(days)

	if want := []string{"2025-12-25", "2025-12-26", "2026-01-01", "2026-01-06"}; !slices.Equal(days, want) {
		t.Errorf("the turn of the year in Bavaria has %v, want %v", days, want)
	}
}

// Every place a picker offers is one the rules know, with a year of holidays: a
// country whose regions divide it by each region, any other by itself and by
// each of its regions.
func TestEveryPlaceOfferedHasItsHolidays(t *testing.T) {
	countries := model.HolidayCountries()

	if len(countries) < 17 {
		t.Fatalf("%d countries are offered", len(countries))
	}

	for _, country := range countries {
		places := []string{}
		if !country.RegionRequired {
			places = append(places, country.Code)
		}

		for _, region := range country.Regions {
			places = append(places, model.HolidayPlace(country.Code, region))
		}

		if len(places) == 0 {
			t.Errorf("%s offers no place at all", country.Code)
		}

		for _, place := range places {
			if !model.ValidHolidayPlace(place) {
				t.Errorf("%s is offered and then refused", place)
			}

			if got := len(model.HolidaysIn(place, 2026)); got < 9 {
				t.Errorf("%s has %d holidays in 2026", place, got)
			}
		}
	}

	countries[0].Regions = append(countries[0].Regions, "XX")
	countries[0].Code = "XX"

	if again := model.HolidayCountries(); again[0].Code == "XX" || slices.Contains(again[0].Regions, "XX") {
		t.Error("changing the list handed out changed the countries themselves")
	}
}

// A place written before there were countries is a German state's code, which
// every place was then - Berlin's BE before Belgium's, and Thuringia's TH before
// Thailand's - and is written that way again for a client from then.
func TestAPlaceWrittenBeforeCountriesIsAGermanState(t *testing.T) {
	for legacy, want := range map[string]string{
		"NI": "DE-NI", "BE": "DE-BE", "TH": "DE-TH", "": "", model.NoHolidays: model.NoHolidays, "AT": "AT", "DE-HH": "DE-HH",
	} {
		if got := model.LegacyHolidayPlace(legacy); got != want {
			t.Errorf("%q read as written before countries is %q, want %q", legacy, got, want)
		}
	}

	for _, country := range model.HolidayCountries() {
		if country.Code != "DE" {
			continue
		}

		for _, state := range country.Regions {
			if got := model.LegacyHolidayRegion(model.LegacyHolidayPlace(state)); got != state {
				t.Errorf("%s read and written again as before countries is %q", state, got)
			}
		}
	}

	for _, choice := range []string{"", model.NoHolidays, "AT", "GB-SCT", "FR-57"} {
		if got := model.LegacyHolidayRegion(choice); got != choice {
			t.Errorf("%q written as before countries is %q, want it as it is", choice, got)
		}
	}
}

// A person's place is their own choice, the installation's where they made none,
// and nowhere where the choice that decides is none - theirs, or the
// installation's they follow.
func TestAPersonsHolidayPlaceFollowsTheirChoice(t *testing.T) {
	for _, c := range []struct{ personal, installation, want string }{
		{"", "DE-BY", "DE-BY"},
		{model.NoHolidays, "DE-BY", ""},
		{"DE-HH", "DE-BY", "DE-HH"},
		{"AT", "DE-BY", "AT"},
		{"", model.NoHolidays, ""},
		{"", "", ""},
		{"AT", model.NoHolidays, "AT"},
	} {
		user := model.User{HolidayPlace: c.personal}

		if got := user.HolidayPlaceOf(c.installation); got != c.want {
			t.Errorf("a choice of %q in an installation keeping %q keeps %q, want %q",
				c.personal, c.installation, got, c.want)
		}
	}
}
