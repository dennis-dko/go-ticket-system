package model

import "time"

// holidaysGB are the United Kingdom's bank holidays by nation, as the Banking and
// Financial Dealings Act 1971 and the proclamations under it set them, and as
// gov.uk publishes them: England and Wales keep the same, Scotland and Northern
// Ireland their own. A holiday on a weekend is kept on the next weekday that is
// not one already.
//
// The days proclaimed for once are those since 2011, each where it was kept:
// Scotland's for the 2026 World Cup is the first that was not the whole
// kingdom's. An earlier year has the standing days only.
var holidaysGB = holidayRules{
	regions:        []string{"ENG", "WLS", "SCT", "NIR"},
	regionRequired: true,
	substitute:     nextFreeWeekday,
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "secondJanuary", on: fixed(time.January, 2), regions: "SCT"},
		{name: "stPatricksDay", on: fixed(time.March, 17), regions: "NIR"},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterMonday", on: afterEaster(1), regions: "ENG WLS NIR"},
		{name: "earlyMayBankHoliday", on: nthWeekday(1, time.Monday, time.May), from: 1978, except: []int{2020}},
		{name: "earlyMayBankHoliday", on: fixed(time.May, 8), only: []int{2020}},
		{name: "springBankHoliday", on: lastWeekday(time.Monday, time.May), except: []int{2012, 2022}},
		{name: "springBankHoliday", on: fixed(time.June, 4), only: []int{2012}},
		{name: "springBankHoliday", on: fixed(time.June, 2), only: []int{2022}},
		{name: "battleOfTheBoyne", on: fixed(time.July, 12), regions: "NIR"},
		{name: "summerBankHoliday", on: nthWeekday(1, time.Monday, time.August), regions: "SCT"},
		{name: "summerBankHoliday", on: lastWeekday(time.Monday, time.August), regions: "ENG WLS NIR"},
		{name: "stAndrewsDay", on: fixed(time.November, 30), regions: "SCT", from: 2007},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26)},
		{name: "royalWedding", on: fixed(time.April, 29), only: []int{2011}},
		{name: "diamondJubilee", on: fixed(time.June, 5), only: []int{2012}},
		{name: "platinumJubilee", on: fixed(time.June, 3), only: []int{2022}},
		{name: "stateFuneral", on: fixed(time.September, 19), only: []int{2022}},
		{name: "coronation", on: fixed(time.May, 8), only: []int{2023}},
		{name: "worldCup", on: fixed(time.June, 15), regions: "SCT", only: []int{2026}},
	},
}
