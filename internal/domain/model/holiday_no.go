package model

import "time"

// holidaysNO are Norway's public holidays: the church's holy days, and 1 May and
// Constitution Day, which an act of their own makes holidays. Christmas Eve
// closes the banks and the shops at noon and is not one.
var holidaysNO = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "maundyThursday", on: afterEaster(-3)},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "constitutionDay", on: fixed(time.May, 17)},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
