package model

import "time"

// holidaysLU are Luxembourg's public holidays (Code du travail, art. L.232-2),
// Europe Day among them since 2019. Good Friday closes the banks rather than
// the country.
var holidaysLU = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "europeDay", on: fixed(time.May, 9), from: 2019},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "nationalDay", on: fixed(time.June, 23)},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "allSaints", on: fixed(time.November, 1)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
