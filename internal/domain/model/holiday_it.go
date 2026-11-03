package model

import "time"

// holidaysIT are Italy's public holidays, St Francis of Assisi's day among them
// again from 2026, and Whit Monday besides in South Tyrol, which keeps it as
// the province of Bolzano. A town's patron saint's day is the town's own, and
// recorded as an absence where it is kept.
var holidaysIT = holidayRules{
	regions: []string{"BZ"},
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "epiphany", on: fixed(time.January, 6)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "liberationDay", on: fixed(time.April, 25)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "whitMonday", on: afterEaster(50), regions: "BZ"},
		{name: "republicDay", on: fixed(time.June, 2), from: 2001},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "stFrancisDay", on: fixed(time.October, 4), from: 2026},
		{name: "allSaints", on: fixed(time.November, 1)},
		{name: "immaculateConception", on: fixed(time.December, 8)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
