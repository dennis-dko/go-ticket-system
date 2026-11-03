package model

import "time"

// holidaysPL are Poland's public holidays (ustawa o dniach wolnych od pracy),
// Epiphany among them again since 2011 and Christmas Eve since 2025.
var holidaysPL = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "epiphany", on: fixed(time.January, 6), from: 2011},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "constitutionDay", on: fixed(time.May, 3)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "corpusChristi", on: afterEaster(60)},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "allSaints", on: fixed(time.November, 1)},
		{name: "independenceDay", on: fixed(time.November, 11)},
		{name: "christmasEve", on: fixed(time.December, 24), from: 2025},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
