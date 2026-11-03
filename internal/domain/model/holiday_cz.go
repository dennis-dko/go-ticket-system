package model

import "time"

// holidaysCZ are the Czech Republic's public holidays, the state holidays and
// the other holidays of zákon č. 245/2000 Sb.: Good Friday among them since 2016.
var holidaysCZ = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "goodFriday", on: afterEaster(-2), from: 2016},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "liberationDay", on: fixed(time.May, 8)},
		{name: "cyrilAndMethodiusDay", on: fixed(time.July, 5)},
		{name: "janHusDay", on: fixed(time.July, 6)},
		{name: "statehoodDay", on: fixed(time.September, 28)},
		{name: "independenceDay", on: fixed(time.October, 28)},
		{name: "freedomAndDemocracyDay", on: fixed(time.November, 17)},
		{name: "christmasEve", on: fixed(time.December, 24)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
