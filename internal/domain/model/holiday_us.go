package model

import "time"

// holidaysUS are the United States' federal holidays (5 U.S.C. 6103), kept on
// the Friday before when they fall on a Saturday and on the Monday after on a
// Sunday, as the federal government keeps them. A state's own holidays, and
// whether an employer keeps the federal ones, vary from place to place; this is
// the federal list, and a day of a state's own is recorded as an absence.
var holidaysUS = holidayRules{
	substitute: nearestWeekday,
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "martinLutherKingDay", on: nthWeekday(3, time.Monday, time.January), from: 1986},
		{name: "washingtonsBirthday", on: nthWeekday(3, time.Monday, time.February)},
		{name: "memorialDay", on: lastWeekday(time.Monday, time.May)},
		{name: "juneteenth", on: fixed(time.June, 19), from: 2021},
		{name: "independenceDay", on: fixed(time.July, 4)},
		{name: "labourDay", on: nthWeekday(1, time.Monday, time.September)},
		{name: "columbusDay", on: nthWeekday(2, time.Monday, time.October)},
		{name: "veteransDay", on: fixed(time.November, 11)},
		{name: "thanksgiving", on: nthWeekday(4, time.Thursday, time.November)},
		{name: "christmasDay", on: fixed(time.December, 25)},
	},
}
