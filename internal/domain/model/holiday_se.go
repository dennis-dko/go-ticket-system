package model

import "time"

// holidaysSE are Sweden's public holidays (lag om allmänna helgdagar), and the
// three eves the Annual Leave Act counts as Sundays (semesterlagen, § 3 a):
// Midsummer Eve, Christmas Eve and New Year's Eve. National Day took Whit
// Monday's place in 2005. Midsummer Day and All Saints' Day are Saturdays.
var holidaysSE = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "epiphany", on: fixed(time.January, 6)},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "whitMonday", on: afterEaster(50), until: 2004},
		{name: "nationalDay", on: fixed(time.June, 6), from: 2005},
		{name: "midsummerEve", on: weekdayAfter(time.Friday, time.June, 19)},
		{name: "midsummerDay", on: weekdayAfter(time.Saturday, time.June, 20)},
		{name: "allSaints", on: weekdayAfter(time.Saturday, time.October, 31)},
		{name: "christmasEve", on: fixed(time.December, 24)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
		{name: "newYearsEve", on: fixed(time.December, 31)},
	},
}
