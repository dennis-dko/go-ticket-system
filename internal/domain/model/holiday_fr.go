package model

import "time"

// holidaysFR are France's public holidays (Code du travail, art. L3133-1), and
// Good Friday and St Stephen's Day besides in Alsace and the Moselle, which keep
// them under their local law (art. L3134-13) - by the ISO 3166-2 codes of the
// European Collectivity of Alsace and of the Moselle. Whit Monday is still a
// public holiday, whatever an employer makes of the solidarity day.
var holidaysFR = holidayRules{
	regions: []string{"6AE", "57"},
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "goodFriday", on: afterEaster(-2), regions: "6AE 57"},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "victoryInEurope", on: fixed(time.May, 8), from: 1982},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "nationalDay", on: fixed(time.July, 14)},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "allSaints", on: fixed(time.November, 1)},
		{name: "armisticeDay", on: fixed(time.November, 11)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26), regions: "6AE 57"},
	},
}
