package model

import "time"

// holidaysAT are Austria's public holidays, as the Arbeitsruhegesetz names them
// (§ 7 Abs. 2), the same in every state. Good Friday is not among them: it was a
// holiday for the members of a few churches only, and stopped being one in
// 2019. Nor are the states' patron saints' days, which close schools rather
// than workplaces.
var holidaysAT = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "epiphany", on: fixed(time.January, 6)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "corpusChristi", on: afterEaster(60)},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "nationalDay", on: fixed(time.October, 26), from: 1967},
		{name: "allSaints", on: fixed(time.November, 1)},
		{name: "immaculateConception", on: fixed(time.December, 8)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
