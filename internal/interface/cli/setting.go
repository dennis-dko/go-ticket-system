package cli

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/support/i18n"
)

// The settings the screen offers are the account's own, and a change is saved to
// the account as My account in the browser saves it - decided by the maintainer
// on 2026-11-03 - so that what is chosen here holds in the browser and on the
// phone, and what is chosen there reaches this screen at its next read. Each is
// sent to the route the browser sends it to, so the screen can set nothing the
// browser cannot.
const (
	settingLanguage  = "language"
	settingTheme     = "theme"
	settingZone      = "timezone"
	settingHolidays  = "holidays"
	settingReminders = "reminders"
	settingBounds    = "bounds"
	settingHours     = "workingHours"
)

// settingAnswers are the settings form's answers before anybody has answered:
// each setting as it stands, so that what is not changed is sent as it was.
func (m screen) settingAnswers() *answers {
	a := m.shown.account

	asked := &answers{
		setting:     settingLanguage,
		language:    a.language,
		theme:       a.theme,
		zone:        a.timezone,
		unbookedDay: a.unbookedDay,
		alertHours:  strconv.Itoa(a.alertHours),
		targetHours: figureField(a.targetHours),
		maxHours:    figureField(a.maxHours),
		countsFrom:  a.countsFrom,
	}

	if asked.language == "" {
		asked.language = m.s.lang.code()
	}

	asked.country, asked.region = model.SplitHolidayPlace(m.places.place)

	// A bound of zero is a bound, unlike the working hours' zero, which is the
	// installation's own.
	if b := m.told.balance; b != nil {
		if b.upper != nil {
			asked.upper = strconv.FormatFloat(*b.upper, 'f', -1, 64)
		}

		if b.lower != nil {
			asked.lower = strconv.FormatFloat(*b.lower, 'f', -1, 64)
		}
	}

	return asked
}

// settingSteps is the settings form: which setting, and then that setting's own
// questions, one at a time as every form here asks them. The others are hidden,
// so the breadcrumb names only the step somebody is at.
func (m screen) settingSteps(a *answers) []*huh.Group {
	l := m.s.lang
	other := func(setting string) func() bool { return func() bool { return a.setting != setting } }

	which := huh.NewSelect[string]().Key("setting").Title(l.text("track.settings.which", "What do you want to set?")).
		Options(m.settingOptions()...).Value(&a.setting)

	languages := make([]huh.Option[string], 0, len(i18n.Languages()))
	for _, code := range i18n.Languages() {
		languages = append(languages, huh.NewOption(i18n.Name(code), code))
	}

	return []*huh.Group{
		huh.NewGroup(which),
		huh.NewGroup(huh.NewSelect[string]().Key("language").Title(l.text("app.language", "Language")).
			Options(languages...).Value(&a.language)).WithHideFunc(other(settingLanguage)),
		huh.NewGroup(huh.NewSelect[string]().Key("theme").Title(l.text("app.theme", "Appearance")).
			Description(l.text("track.settings.themeNote",
				"The browser and the app follow it. This screen keeps to the colours the terminal's background calls for, wherever the terminal says what that is.")).
			Options(huh.NewOption(l.text("track.theme.auto", "Automatic"), ""),
				huh.NewOption(l.text("theme.light", "Light"), model.ThemeLight),
				huh.NewOption(l.text("theme.dark", "Dark"), model.ThemeDark)).
			Value(&a.theme)).WithHideFunc(other(settingTheme)),
		huh.NewGroup(huh.NewInput().Key("timezone").Title(l.text("tz.myTitle", "My timezone")).
			Description(l.text("track.settings.zoneNote",
				"Written as the tz database writes it, such as Europe/Berlin. Empty follows the installation ({0}).",
				m.shown.installationZone)).
			Placeholder(m.shown.installationZone).Validate(l.readableZone).Value(&a.zone)).WithHideFunc(other(settingZone)),
		huh.NewGroup(m.countryField(a)).WithHideFunc(other(settingHolidays)),
		huh.NewGroup(m.regionField(a)).WithHideFunc(func() bool {
			c, _ := m.places.country(a.country)

			return a.setting != settingHolidays || len(c.Regions) == 0
		}),
		huh.NewGroup(huh.NewConfirm().Key("unbookedDay").
			Title(l.text("remind.unbookedDay", "Mention the last working day when nothing is booked on it")).
			Affirmative(l.text("track.huh.yes", "Yes")).Negative(l.text("track.huh.no", "No")).
			Value(&a.unbookedDay)).WithHideFunc(other(settingReminders)),
		huh.NewGroup(huh.NewInput().Key("alertHours").Title(l.text("field.timerAlertHours", "Mention the stopwatch after (hours)")).
			Description(l.text("remind.hint",
				"Zero means the stopwatch is never mentioned. Past a day, it says by itself that it holds more than one entry can.")).
			Validate(l.readableAlertHours).Value(&a.alertHours)).WithHideFunc(other(settingReminders)),
		huh.NewGroup(huh.NewInput().Key("upper").Title(l.text("field.upperBound", "Upper bound of the balance")).
			Placeholder(l.text("track.form.optional", "optional")).Validate(l.readableFigure).Value(&a.upper)).
			WithHideFunc(other(settingBounds)),
		huh.NewGroup(huh.NewInput().Key("lower").Title(l.text("field.lowerBound", "Lower bound of the balance")).
			Placeholder(l.text("track.form.optional", "optional")).Validate(l.readableFigure).Value(&a.lower)).
			WithHideFunc(other(settingBounds)),
		huh.NewGroup(huh.NewInput().Key("targetHours").Title(l.text("settings.targetHours", "Target hours per day")).
			Placeholder(figureField(model.DefaultDailyTargetHours)).Validate(l.readableFigure).Value(&a.targetHours)).
			WithHideFunc(other(settingHours)),
		huh.NewGroup(huh.NewInput().Key("maxHours").Title(l.text("settings.maxHours", "Maximum hours per day")).
			Placeholder(l.text("track.form.optional", "optional")).Validate(l.readableFigure).Value(&a.maxHours)).
			WithHideFunc(other(settingHours)),
		huh.NewGroup(huh.NewInput().Key("countsFrom").Title(l.text("settings.everyDayCountsFrom", "Every working day counts from")).
			Description(l.text("settings.everyDayCountsFromHint",
				"Until you choose a day, only the days you booked on count towards your balance. From the day you choose, every working day up to today counts, booked or not, less what a public holiday or an absence takes off it. Empty it to go back.")).
			Placeholder(l.text("track.form.dayHint", "YYYY-MM-DD")).Validate(l.readableDayOrNone).Value(&a.countsFrom)).
			WithHideFunc(other(settingHours)),
	}
}

// settingOptions are the settings the screen offers, each with what it is set to
// now. The bounds only where the installation answers the balance they bound,
// the holidays only where it names the countries, and the working hours only
// for an account that may set its own - which the browser asks too.
func (m screen) settingOptions() []huh.Option[string] {
	l, a := m.s.lang, m.shown.account

	option := func(name, now, setting string) huh.Option[string] {
		return huh.NewOption(name+": "+now, setting)
	}

	options := []huh.Option[string]{
		option(l.text("app.language", "Language"), m.languageNow(), settingLanguage),
		option(l.text("app.theme", "Appearance"), l.themeName(a.theme), settingTheme),
		option(l.text("tz.myTitle", "My timezone"), m.zoneNow(), settingZone),
	}

	if len(m.places.countries) > 0 {
		options = append(options, option(l.text("holiday.myTitle", "My public holidays"), m.holidaysNow(), settingHolidays))
	}

	options = append(options, option(l.text("remind.title", "Reminders"), m.remindersNow(), settingReminders))

	if b := m.told.balance; b != nil {
		options = append(options, option(l.text("bounds.title", "Bounds of my balance"),
			l.text("track.settings.boundsNow", "upper {0}, lower {1}", l.bound(b.upper), l.bound(b.lower)), settingBounds))
	}

	if slices.Contains(a.permissions, model.PermSettingsWriteOwn) {
		target := a.targetHours
		if target <= 0 {
			target = model.DefaultDailyTargetHours
		}

		options = append(options, option(l.text("settings.workingTimes", "My working hours"),
			l.text("track.settings.perDay", "{0} a day", l.hours(target)), settingHours))
	}

	return options
}

// languageNow is the account's language by its own name, or the one the client
// speaks where the account chose none.
func (m screen) languageNow() string {
	if m.shown.language != "" {
		return i18n.Name(m.shown.language)
	}

	return i18n.Name(m.s.lang.code())
}

// themeName is an appearance in words.
func (l language) themeName(theme string) string {
	switch theme {
	case model.ThemeLight:
		return l.text("theme.light", "Light")
	case model.ThemeDark:
		return l.text("theme.dark", "Dark")
	}

	return l.text("track.theme.auto", "Automatic")
}

// zoneNow is the account's own zone, or the installation's it follows.
func (m screen) zoneNow() string {
	if m.shown.timezone != "" {
		return m.shown.timezone
	}

	return m.s.lang.text("track.settings.follows", "follows the installation ({0})", m.shown.installationZone)
}

// holidaysNow is where the account keeps public holidays, in words.
func (m screen) holidaysNow() string {
	l := m.s.lang

	switch m.places.place {
	case "":
		return l.text("track.settings.follows", "follows the installation ({0})", l.placeName(m.places.installation))
	case model.NoHolidays:
		return l.text("holiday.noRegion", "No public holidays")
	}

	return l.placeName(m.places.place)
}

// remindersNow is both reminder choices, in words.
func (m screen) remindersNow() string {
	l := m.s.lang

	stopwatch := l.text("track.settings.stopwatchNever", "stopwatch never")
	if m.shown.alertHours > 0 {
		stopwatch = l.text("track.settings.stopwatchAfter", "stopwatch after {0}",
			strconv.Itoa(m.shown.alertHours)+" "+l.text("unit.hours", "h"))
	}

	unbooked := l.text("track.settings.unbookedOff", "last working day off")
	if m.shown.unbookedDay {
		unbooked = l.text("track.settings.unbookedOn", "last working day on")
	}

	return stopwatch + ", " + unbooked
}

// bound is a bound of the balance in words, none for nil.
func (l language) bound(b *float64) string {
	if b == nil {
		return l.text("track.settings.noBound", "none")
	}

	return l.signedHours(*b)
}

// countryField asks where the account keeps public holidays: following the
// installation, none, or a country - each named as the browser's picker names
// it, the countries by their names in the client's language.
func (m screen) countryField(a *answers) *huh.Select[string] {
	l := m.s.lang

	options := []huh.Option[string]{
		huh.NewOption(l.text("holiday.inherit", "Follow the installation ({0})", l.placeName(m.places.installation)), ""),
		huh.NewOption(l.text("holiday.noRegion", "No public holidays"), model.NoHolidays),
	}

	countries := slices.Clone(m.places.countries)
	slices.SortFunc(countries, func(x, y holidayCountry) int {
		return strings.Compare(l.countryName(x.Code), l.countryName(y.Code))
	})

	for _, c := range countries {
		options = append(options, huh.NewOption(l.countryName(c.Code), c.Code))
	}

	return huh.NewSelect[string]().Key("country").Title(l.text("holiday.country", "Country")).
		Options(options...).Height(projectRows + 1).Value(&a.country)
}

// regionField asks for the region of the country chosen, called what that
// country calls its parts: one is needed where they divide it, and where a few
// keep a day or two more, the rest of the country comes first.
func (m screen) regionField(a *answers) *huh.Select[string] {
	l := m.s.lang

	return huh.NewSelect[string]().Key("region").
		TitleFunc(func() string { return l.regionsCalled(a.country) }, &a.country).
		OptionsFunc(func() []huh.Option[string] {
			c, _ := m.places.country(a.country)

			first := huh.NewOption(l.text("holiday.elsewhere", "Everywhere else"), "")
			if c.RegionRequired {
				first = huh.NewOption(l.text("holiday.chooseRegion", "Choose one"), "")
			}

			options := []huh.Option[string]{first}
			for _, region := range c.Regions {
				options = append(options, huh.NewOption(l.regionName(model.HolidayPlace(c.Code, region)), region))
			}

			return options
		}, &a.country).
		Validate(func(region string) error {
			if c, _ := m.places.country(a.country); c.RegionRequired && region == "" {
				return errors.New(l.text("holiday.chooseRegion", "Choose one"))
			}

			return nil
		}).
		Height(projectRows + 1).Value(&a.region)
}

// settingStep is what a field of the settings form is called in the
// breadcrumb, nothing for the question which setting it is.
func (m screen) settingStep(field string) string {
	l := m.s.lang

	switch field {
	case "language":
		return l.text("app.language", "Language")
	case "theme":
		return l.text("app.theme", "Appearance")
	case "timezone":
		return l.text("tz.myTitle", "My timezone")
	case "country":
		return l.text("holiday.country", "Country")
	case "region":
		return l.regionsCalled(m.asked.country)
	case "unbookedDay", "alertHours":
		return l.text("remind.title", "Reminders")
	case "upper":
		return l.text("field.upperBound", "Upper bound of the balance")
	case "lower":
		return l.text("field.lowerBound", "Lower bound of the balance")
	case "targetHours":
		return l.text("settings.targetHours", "Target hours per day")
	case "maxHours":
		return l.text("settings.maxHours", "Maximum hours per day")
	case "countsFrom":
		return l.text("settings.everyDayCountsFrom", "Every working day counts from")
	}

	return ""
}

// saveSetting sends the setting the form was answered for to the route the
// browser sends it to, and says so.
func (m screen) saveSetting() tea.Cmd {
	ctx, s, a := m.ctx, m.s, *m.asked

	var (
		route string
		body  any
	)

	switch a.setting {
	case settingLanguage:
		route, body = "/me/language", map[string]string{"language": a.language}
	case settingTheme:
		route, body = "/me/theme", map[string]string{"theme": a.theme}
	case settingZone:
		route, body = "/me/timezone", map[string]string{"timezone": strings.TrimSpace(a.zone)}
	case settingHolidays:
		route, body = "/me/holidays", map[string]string{"place": m.placeChosen(a)}
	case settingReminders:
		hours, _ := strconv.Atoi(strings.TrimSpace(a.alertHours))
		route, body = "/me/reminders", map[string]any{"unbookedDay": a.unbookedDay, "timerAlertHours": hours}
	case settingBounds:
		upper, _ := figure(a.upper)
		lower, _ := figure(a.lower)
		route, body = "/me/balance-limits", map[string]any{"upperBound": upper, "lowerBound": lower}
	case settingHours:
		target, _ := figure(a.targetHours)
		most, _ := figure(a.maxHours)

		// Empty is the installation's own, which the route reads as zero, and an
		// empty day takes it off again - as the browser sends both.
		route = fmt.Sprintf("/users/%d/working-times", m.shown.id)
		body = map[string]any{
			"dailyTargetHours": orNone(target), "maxDailyHours": orNone(most),
			"everyDayCountsFrom": strings.TrimSpace(a.countsFrom),
		}
	default:
		return nil
	}

	// Said in the language just chosen where the screen is about to speak it, so
	// that the line under a German screen is not left in English.
	words := s.lang
	if a.setting == settingLanguage && !s.asked {
		words = speaking(a.language)
	}

	return func() tea.Msg {
		if err := s.api.call(ctx, http.MethodPut, route, body, nil); err != nil {
			return done{err: err}
		}

		return done{said: words.text("track.settings.saved", "Saved for your account - the browser and the app follow it too.")}
	}
}

// placeChosen is the place the holiday questions were answered with: following
// the installation, none, or the country with its region - the region only where
// it is one of that country's, since it may have been chosen for another before
// the country was changed.
func (m screen) placeChosen(a answers) string {
	c, ok := m.places.country(a.country)
	if !ok {
		return a.country
	}

	if !slices.Contains(c.Regions, a.region) {
		return c.Code
	}

	return model.HolidayPlace(c.Code, a.region)
}

// readableZone refuses a zone this client does not know, which the installation
// would refuse too; empty follows the installation.
func (l language) readableZone(raw string) error {
	if raw = strings.TrimSpace(raw); raw == "" || model.IsSupportedTimezone(raw) {
		return nil
	}

	return errors.New(l.text("track.settings.unknownZone", "{0} is not a time zone", raw))
}

// readableAlertHours refuses anything but a whole number of hours from none to
// a day.
func (l language) readableAlertHours(raw string) error {
	if hours, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && hours >= 0 && hours <= model.MaxTimerAlertHours {
		return nil
	}

	return errors.New(l.text("track.settings.alertRange", "write a whole number from 0 to {0}", strconv.Itoa(model.MaxTimerAlertHours)))
}

// readableFigure refuses what is not a number of hours; empty is none.
func (l language) readableFigure(raw string) error {
	if _, ok := figure(raw); ok {
		return nil
	}

	return errors.New(l.text("track.settings.notHours", "{0} is not a number of hours", strings.TrimSpace(raw)))
}

// readableDayOrNone refuses a day written in no way the installation reads;
// empty is none.
func (l language) readableDayOrNone(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	return l.readableDay(strings.TrimSpace(raw))
}

// figure reads a number of hours somebody typed, with a decimal point or a
// decimal comma, and nil for nothing.
func figure(raw string) (*float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}

	v, err := strconv.ParseFloat(strings.Replace(raw, ",", ".", 1), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, false
	}

	return &v, true
}

// figureField is a number as a field shows it to be changed, empty for none.
func figureField(v float64) string {
	if v == 0 {
		return ""
	}

	return strconv.FormatFloat(v, 'f', -1, 64)
}

// orNone is a number to send where empty means the installation's own, which
// the route takes as zero.
func orNone(v *float64) float64 {
	if v == nil {
		return 0
	}

	return *v
}
