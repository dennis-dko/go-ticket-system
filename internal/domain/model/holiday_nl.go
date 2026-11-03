package model

import "time"

// holidaysNL are the Netherlands' public holidays, as the Algemene termijnenwet
// names them, but for Liberation Day: the act counts 5 May, and whether it is a
// day off is left to each collective agreement, most of which give it only every
// fifth year - so it is recorded as an absence where it is one. Good Friday is
// not a public holiday at all. King's Day moves to the Saturday before when the
// 27th of April is a Sunday, as Queen's Day did from the 30th until 2013.
var holidaysNL = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "queensDay", on: saturdayForSunday(time.April, 30), from: 1980, until: 2013},
		{name: "kingsDay", on: saturdayForSunday(time.April, 27), from: 2014},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}

// saturdayForSunday is a holiday on a date, kept on the Saturday before where
// the date is a Sunday.
func saturdayForSunday(month time.Month, day int) func(int) time.Time {
	return func(year int) time.Time {
		date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if date.Weekday() == time.Sunday {
			return date.AddDate(0, 0, -1)
		}

		return date
	}
}
