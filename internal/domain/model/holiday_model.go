package model

import (
	"maps"
	"slices"
	"strings"
	"time"
)

// Holiday is a public holiday: its day, and which one it is, by a name the
// interface translates. Substitute marks the weekday a country keeps a holiday
// on that fell on a weekend - Britain's Christmas on a Saturday is taken on the
// Monday - which is a holiday in its own right and keeps the name of the one it
// stands in for.
type Holiday struct {
	Day        time.Time
	Name       string
	Substitute bool
}

// HolidayCountry is a country whose public holidays are worked out here: its
// ISO 3166-1 code, the regions that keep holidays of their own by the rest of
// their ISO 3166-2 code, in the order a picker offers them, and whether
// somebody there has to name one.
//
// A region is required where the regions divide the country, each keeping a
// year of its own - Germany's states, Britain's nations. It is not where a few
// regions keep a day or two more than the rest of the country - Alsace,
// Madeira - and the country without one is everywhere else.
type HolidayCountry struct {
	Code           string
	Regions        []string
	RegionRequired bool
}

// holidayRules is one country's public holidays: the rules, its regions, and
// what it does with a holiday that falls on a weekend.
//
// Worked out rather than looked up: every holiday kept here is a fixed date, a
// fixed distance from Easter or a weekday counted in a month, so a calendar
// service would be a dependency, and a network call under every deadline, for
// something the binary can compute. A country whose holidays are set by decree
// every year, or follow the moon, is not kept here rather than kept wrongly.
type holidayRules struct {
	regions        []string
	regionRequired bool
	rules          []holidayRule

	// substitute, where a country moves a holiday off a weekend, is the weekday
	// it is kept on instead, given the holidays of the year so far; nil keeps
	// every holiday where it falls.
	substitute func(day time.Time, taken map[time.Time]bool) (time.Time, bool)
}

// holidaysByCountry is every country kept here, by its ISO 3166-1 code - each
// in a file of its own, holiday_<code>.go.
var holidaysByCountry = map[string]holidayRules{
	"AT": holidaysAT, "BE": holidaysBE, "CZ": holidaysCZ, "DE": holidaysDE,
	"DK": holidaysDK, "FI": holidaysFI, "FR": holidaysFR, "GB": holidaysGB,
	"IE": holidaysIE, "IT": holidaysIT, "LU": holidaysLU, "NL": holidaysNL,
	"NO": holidaysNO, "PL": holidaysPL, "PT": holidaysPT, "SE": holidaysSE,
	"US": holidaysUS,
}

// HolidayCountries returns the countries a picker offers, by their code, as
// copies nobody can change the originals through.
func HolidayCountries() []HolidayCountry {
	countries := make([]HolidayCountry, 0, len(holidaysByCountry))

	for _, code := range slices.Sorted(maps.Keys(holidaysByCountry)) {
		rules := holidaysByCountry[code]
		countries = append(countries, HolidayCountry{
			Code: code, Regions: slices.Clone(rules.regions), RegionRequired: rules.regionRequired,
		})
	}

	return countries
}

// HolidayPlace is where somebody keeps public holidays, written as an ISO 3166-2
// code: the country, and its region after a dash where there is one - DE-NI,
// AT, GB-SCT.
func HolidayPlace(country, region string) string {
	if region == "" {
		return country
	}

	return country + "-" + region
}

// SplitHolidayPlace is a place's country and region.
func SplitHolidayPlace(place string) (country, region string) {
	country, region, _ = strings.Cut(place, "-")

	return country, region
}

// ValidHolidayPlace reports whether a place is one the rules know: a country,
// with one of its regions where it has to have one.
func ValidHolidayPlace(place string) bool {
	country, region := SplitHolidayPlace(place)

	rules, ok := holidaysByCountry[country]
	if !ok {
		return false
	}

	if region == "" {
		return !rules.regionRequired && !strings.Contains(place, "-")
	}

	return slices.Contains(rules.regions, region)
}

// NoHolidays is a choice to keep no public holidays at all, which an empty one
// cannot say: an empty choice is one nobody has made yet. Both keep none, and
// they are kept apart so a screen can say which of the two it is.
const NoHolidays = "none"

// ValidHolidayChoice reports whether a choice of public holidays may be stored:
// a place, NoHolidays, or empty.
func ValidHolidayChoice(choice string) bool {
	return HolidayChoiceFaultOf(choice) == NoHolidayChoiceFault
}

// HolidayChoiceFault is what is wrong with a choice of public holidays, told
// apart so that whoever refuses it can say which half to change.
type HolidayChoiceFault uint8

const (
	// NoHolidayChoiceFault is a choice that may be stored.
	NoHolidayChoiceFault HolidayChoiceFault = iota

	// UnknownHolidayCountry names a country the rules know nothing of.
	UnknownHolidayCountry

	// UnknownHolidayRegion names a region its country does not have, or a
	// country whose regions divide it without one.
	UnknownHolidayRegion
)

// HolidayChoiceFaultOf says what is wrong with a choice of public holidays.
func HolidayChoiceFaultOf(choice string) HolidayChoiceFault {
	if choice == "" || choice == NoHolidays || ValidHolidayPlace(choice) {
		return NoHolidayChoiceFault
	}

	country, _ := SplitHolidayPlace(choice)
	if _, ok := holidaysByCountry[country]; !ok {
		return UnknownHolidayCountry
	}

	return UnknownHolidayRegion
}

// HolidaysKeptAt returns the place whose public holidays a choice names, empty
// for nowhere: a choice nobody has made, or NoHolidays.
func HolidaysKeptAt(choice string) string {
	if choice == NoHolidays {
		return ""
	}

	return choice
}

// holidayRule is one holiday and where and when it holds.
type holidayRule struct {
	name string
	on   func(year int) time.Time

	// regions it holds in, separated by spaces; empty is the whole country.
	regions string

	// from and until bound the years it holds in, zero leaving that end open.
	from, until int

	// only, when set, are the one-off years it holds in and no others; except
	// are years it does not hold in, where it was moved for once.
	only, except []int
}

// HolidaysIn returns the public holidays of one year in one place, earliest
// first. Anything that is not a place - empty, NoHolidays - has none.
func HolidaysIn(place string, year int) []Holiday {
	if !ValidHolidayPlace(place) {
		return nil
	}

	country, region := SplitHolidayPlace(place)
	rules := holidaysByCountry[country]

	// The years either side as well: a substitute can fall in the next year or,
	// where a country takes a Saturday's holiday on the Friday before, in the
	// last one - the United States kept New Year's Day 2022 on 31 December 2021.
	var holidays []Holiday

	for y := year - 1; y <= year+1; y++ {
		holidays = append(holidays, rules.year(region, y)...)
	}

	return slices.DeleteFunc(holidays, func(h Holiday) bool { return h.Day.Year() != year })
}

// year is one year of a country's holidays in a region, with the substitutes
// for those that fell on a weekend, earliest first.
func (c holidayRules) year(region string, year int) []Holiday {
	var holidays []Holiday

	taken := map[time.Time]bool{}

	for _, rule := range c.rules {
		if !rule.holdsIn(region, year) {
			continue
		}

		day := rule.on(year)

		// Once where two rules give the same day - the Reformation's 2017 in a
		// state that keeps it every year.
		if taken[day] {
			continue
		}

		taken[day] = true
		holidays = append(holidays, Holiday{Day: day, Name: rule.name})
	}

	slices.SortFunc(holidays, func(a, b Holiday) int { return a.Day.Compare(b.Day) })

	if c.substitute == nil {
		return holidays
	}

	// In the order the holidays fall, so that Christmas on a Saturday takes the
	// Monday and Boxing Day on the Sunday the Tuesday after it.
	for _, holiday := range slices.Clone(holidays) {
		if instead, moved := c.substitute(holiday.Day, taken); moved {
			taken[instead] = true
			holidays = append(holidays, Holiday{Day: instead, Name: holiday.Name, Substitute: true})
		}
	}

	slices.SortFunc(holidays, func(a, b Holiday) int { return a.Day.Compare(b.Day) })

	return holidays
}

// HolidaysBetween returns the public holidays of a place from one day to
// another, both included, keyed by their day.
func HolidaysBetween(place string, from, to time.Time) map[time.Time]Holiday {
	from, to = CalendarDay(from), CalendarDay(to)
	found := map[time.Time]Holiday{}

	for year := from.Year(); year <= to.Year(); year++ {
		for _, holiday := range HolidaysIn(place, year) {
			if !holiday.Day.Before(from) && !holiday.Day.After(to) {
				found[holiday.Day] = holiday
			}
		}
	}

	return found
}

func (r holidayRule) holdsIn(region string, year int) bool {
	if r.regions != "" && !slices.Contains(strings.Fields(r.regions), region) {
		return false
	}

	if r.only != nil {
		return slices.Contains(r.only, year)
	}

	if slices.Contains(r.except, year) {
		return false
	}

	return (r.from == 0 || year >= r.from) && (r.until == 0 || year <= r.until)
}

// fixed is a holiday on the same date every year.
func fixed(month time.Month, day int) func(int) time.Time {
	return func(year int) time.Time { return time.Date(year, month, day, 0, 0, 0, 0, time.UTC) }
}

// afterEaster is a holiday a number of days from Easter Sunday.
func afterEaster(days int) func(int) time.Time {
	return func(year int) time.Time { return easterSunday(year).AddDate(0, 0, days) }
}

// nthWeekday is a holiday on the nth weekday of a month: the third Monday of
// January.
func nthWeekday(n int, weekday time.Weekday, month time.Month) func(int) time.Time {
	return func(year int) time.Time {
		return weekdayFrom(weekday, time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)).AddDate(0, 0, 7*(n-1))
	}
}

// lastWeekday is a holiday on the last weekday of a month: the last Monday of
// May.
func lastWeekday(weekday time.Weekday, month time.Month) func(int) time.Time {
	return func(year int) time.Time {
		day := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)

		for day.Weekday() != weekday {
			day = day.AddDate(0, 0, -1)
		}

		return day
	}
}

// weekdayAfter is a holiday on the first weekday on or after a date: Sweden's
// Midsummer Day, the Saturday between the 20th and the 26th of June.
func weekdayAfter(weekday time.Weekday, month time.Month, day int) func(int) time.Time {
	return func(year int) time.Time {
		return weekdayFrom(weekday, time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
	}
}

// weekdayFrom is the first weekday on or after a day.
func weekdayFrom(weekday time.Weekday, day time.Time) time.Time {
	for day.Weekday() != weekday {
		day = day.AddDate(0, 0, 1)
	}

	return day
}

// nextFreeWeekday keeps a holiday that fell on a weekend on the next weekday
// that is not a holiday already, which is how Britain does it.
func nextFreeWeekday(day time.Time, taken map[time.Time]bool) (time.Time, bool) {
	if !weekend(day) {
		return time.Time{}, false
	}

	for weekend(day) || taken[day] {
		day = day.AddDate(0, 0, 1)
	}

	return day, true
}

// nearestWeekday keeps a holiday on a Saturday on the Friday before it and one
// on a Sunday on the Monday after it, which is how the United States does it.
func nearestWeekday(day time.Time, _ map[time.Time]bool) (time.Time, bool) {
	switch day.Weekday() {
	case time.Saturday:
		return day.AddDate(0, 0, -1), true
	case time.Sunday:
		return day.AddDate(0, 0, 1), true
	}

	return time.Time{}, false
}

func weekend(day time.Time) bool {
	return day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
}

// easterSunday is the Western Easter of a year, by the Gregorian computus in the
// form Meeus gives it - which needs nothing but integer arithmetic.
func easterSunday(year int) time.Time {
	a := year % 19
	b, c := year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
