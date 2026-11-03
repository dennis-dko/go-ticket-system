package model

import "time"

// holidaysBE are Belgium's ten public holidays, as the royal decree of 18 April
// 1974 names them. Good Friday, the day after Ascension and Boxing Day close the
// banks rather than the country, and Easter and Whit Sunday are Sundays the
// decree does not name. A holiday on a Sunday is not moved: the employer gives
// a day in its place, and chooses which.
var holidaysBE = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "nationalDay", on: fixed(time.July, 21)},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "allSaints", on: fixed(time.November, 1)},
		{name: "armisticeDay", on: fixed(time.November, 11)},
		{name: "christmasDay", on: fixed(time.December, 25)},
	},
}
