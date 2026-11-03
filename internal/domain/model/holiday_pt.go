package model

import "time"

// holidaysPT are Portugal's public holidays (Código do Trabalho, art. 234): the
// four that were suspended from 2013 to 2015 hold outside those years. The
// Azores keep their day on the Monday after Pentecost (Decreto Regional n.º
// 13/80), Madeira 1 July and Boxing Day besides. A municipality's day is its own,
// and recorded as an absence; Carnival is a day off only where an employer gives
// it.
//
// Azores Day is not on 1 June, which is what a widely used holiday dataset has
// every year: the decree and the Azores' own calendar put it on the Monday after
// Pentecost, 25 May in 2026.
var holidaysPT = holidayRules{
	regions: []string{"20", "30"},
	rules: []holidayRule{
		{name: "newYear", on: fixed(time.January, 1)},
		{name: "goodFriday", on: afterEaster(-2)},
		{name: "easterSunday", on: afterEaster(0)},
		{name: "freedomDay", on: fixed(time.April, 25)},
		{name: "labourDay", on: fixed(time.May, 1)},
		{name: "azoresDay", on: afterEaster(50), regions: "20"},
		{name: "corpusChristi", on: afterEaster(60), except: suspendedInPortugal},
		{name: "nationalDay", on: fixed(time.June, 10)},
		{name: "madeiraDay", on: fixed(time.July, 1), regions: "30"},
		{name: "assumption", on: fixed(time.August, 15)},
		{name: "republicDay", on: fixed(time.October, 5), except: suspendedInPortugal},
		{name: "allSaints", on: fixed(time.November, 1), except: suspendedInPortugal},
		{name: "restorationOfIndependence", on: fixed(time.December, 1), except: suspendedInPortugal},
		{name: "immaculateConception", on: fixed(time.December, 8)},
		{name: "christmasDay", on: fixed(time.December, 25)},
		{name: "stStephensDay", on: fixed(time.December, 26), regions: "30"},
	},
}

// suspendedInPortugal are the years four of Portugal's holidays were not kept.
var suspendedInPortugal = []int{2013, 2014, 2015}
