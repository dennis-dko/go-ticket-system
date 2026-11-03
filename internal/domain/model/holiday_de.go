package model

import "time"

// holidaysDE are Germany's public holidays: the ones that hold in the whole of a
// state, by its ISO 3166-2 code without the country.
//
// Not those that hold in part of one - the Assumption in Bavaria's Catholic
// municipalities, Corpus Christi in parts of Saxony and Thuringia, the Peace
// Festival in Augsburg - because a state's answer cannot know which part somebody
// works in, and granting such a day to the whole state is as wrong as granting it
// to nobody. Somebody it applies to records it as an absence, AbsenceLocalHoliday.
//
// A holiday a state introduced counts from the year it was introduced, and one
// that was abolished until the year it was, so working out an earlier year does
// not rewrite what that year asked.
var holidaysDE = holidayRules{
	regions: []string{
		"BW", "BY", "BE", "BB", "HB", "HH", "HE", "MV",
		"NI", "NW", "RP", "SL", "SN", "ST", "SH", "TH",
	},
	regionRequired: true,
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "epiphany", on: fixed(time.January, 6), regions: "BW BY ST"},
		{name: "womensDay", on: fixed(time.March, 8), regions: "BE", from: 2019},
		{name: "womensDay", on: fixed(time.March, 8), regions: "MV", from: 2023},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterSunday", on: afterEaster(0), regions: "BB"},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "liberationDay", on: fixed(time.May, 8), regions: "BE", only: []int{2020, 2025}},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49), regions: "BB"},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "corpusChristi", on: afterEaster(60), regions: "BW BY HE NW RP SL"},
		{name: "uprisingDay", on: fixed(time.June, 17), regions: "BE", only: []int{2028}},
		{name: "assumption", on: fixed(time.August, 15), regions: "SL"},
		{name: "childrensDay", on: fixed(time.September, 20), regions: "TH", from: 2019},
		{name: "germanUnity", on: fixed(time.October, 3), from: 1990},
		{name: "reformationDay", on: fixed(time.October, 31), regions: "BB MV SN ST TH"},
		{name: "reformationDay", on: fixed(time.October, 31), regions: "HB HH NI SH", from: 2018},
		{name: "reformationDay", on: fixed(time.October, 31), only: []int{2017}},
		{name: "allSaints", on: fixed(time.November, 1), regions: "BW BY NW RP SL"},
		{name: "repentanceDay", on: repentanceDay, regions: "SN"},
		{name: "repentanceDay", on: repentanceDay, until: 1994},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}

// repentanceDay is the Wednesday before the 23rd of November.
func repentanceDay(year int) time.Time {
	day := time.Date(year, time.November, 22, 0, 0, 0, 0, time.UTC)

	for day.Weekday() != time.Wednesday {
		day = day.AddDate(0, 0, -1)
	}

	return day
}
