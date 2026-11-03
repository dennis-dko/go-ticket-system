package model

import "time"

// holidaysDK are Denmark's public holidays, the church's holy days. The General
// Prayer Day, the fourth Friday after Easter, was one until 2023. Constitution
// Day, Christmas Eve and New Year's Eve are kept by many employers and are not
// public holidays.
var holidaysDK = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "maundyThursday", on: afterEaster(-3)},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "greatPrayerDay", on: afterEaster(26), until: 2023},
		{name: "ascension", on: afterEaster(39)},
		{name: "whitSunday", on: afterEaster(49)},
		{name: "whitMonday", on: afterEaster(50)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}
