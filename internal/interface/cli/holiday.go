package cli

// holidayOf is the public holiday a day is, in words, as the page's holidayOf
// says it: one kept in place of a holiday that fell on a weekend is said to be,
// which would otherwise read as Christmas on the 27th.
func (l language) holidayOf(day weekDay) string {
	name := l.pageSays("holiday."+day.Holiday, day.Holiday)

	if day.HolidaySubstitute {
		return l.text("holiday.substitute", "{0} (substitute day)", name)
	}

	return name
}

// absenceName is what a kind of absence is called, as the page calls it.
func (l language) absenceName(kind string) string {
	return l.pageSays("absence."+kind, kind)
}
