package model

import "time"

// holidaysIE are Ireland's public holidays (Organisation of Working Time Act
// 1997, Schedule 2), St Brigid's Day among them since 2023. A holiday on a
// weekend is not moved: whoever it falls to is owed a day in its place, and the
// employer chooses which.
var holidaysIE = holidayRules{
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "stBrigidsDay", on: stBrigidsDay, from: 2023},
		{name: "stPatricksDay", on: fixed(time.March, 17)},
		{name: "dayOfRemembrance", on: fixed(time.March, 18), only: []int{2022}},
		{name: "easterMonday", on: afterEaster(1)},
		{name: "mayBankHoliday", on: nthWeekday(1, time.Monday, time.May), from: 1994},
		{name: "juneBankHoliday", on: nthWeekday(1, time.Monday, time.June)},
		{name: "augustBankHoliday", on: nthWeekday(1, time.Monday, time.August)},
		{name: "octoberBankHoliday", on: lastWeekday(time.Monday, time.October), from: 1977},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
	},
}

// stBrigidsDay is the first Monday in February, or the 1st where that is a
// Friday.
func stBrigidsDay(year int) time.Time {
	first := time.Date(year, time.February, 1, 0, 0, 0, 0, time.UTC)
	if first.Weekday() == time.Friday {
		return first
	}

	return weekdayFrom(time.Monday, first)
}
