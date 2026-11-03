package model

import "time"

// holidaysFI are Finland's public holidays, with Midsummer Eve and Christmas
// Eve, which the Annual Holidays Act counts among the weekdays a holiday takes
// (vuosilomalaki, § 4) even though no act makes them holidays. Midsummer Day and
// All Saints' Day are Saturdays.
var holidaysFI = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "epiphany", on: fixed(time.January, 6)},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "midsummerEve", on: weekdayAfter(time.Friday, time.June, 19)},
		{name: "midsummerDay", on: weekdayAfter(time.Saturday, time.June, 20)},
		{name: "allSaints", on: weekdayAfter(time.Saturday, time.October, 31)},
		{name: "independenceDay", on: fixed(time.December, 6)},
		{name: "christmasEve", on: fixed(time.December, 24)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
