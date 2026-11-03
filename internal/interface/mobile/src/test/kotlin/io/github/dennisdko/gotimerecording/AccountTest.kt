package io.github.dennisdko.gotimerecording

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId

class AccountTest {

    private val english = Language("en", emptyMap())
    private val berlin = ZoneId.of("Europe/Berlin")

    private fun day(running: Boolean, alertHours: Int) = Snapshot(
        name = "Dana Example",
        zone = berlin,
        language = "",
        projects = emptyList(),
        clock = Stopwatch(running, null, Instant.parse("2026-10-22T07:12:00Z"), 0.0),
        today = LocalDate.parse("2026-11-03"),
        entries = emptyList(),
        version = "v9.9.9",
        account = Account(alertHours = alertHours),
    )

    // What /me answers is read as the account's own settings, the reminders and
    // the rights beside the user.
    @Test
    fun anAccountIsReadAsMeAnswersIt() {
        val account = Account.of(
            JSONObject(
                """{"user":{"id":7,"theme":"dark","timezone":"","instanceTimezone":"Europe/Berlin",""" +
                    """"dailyTargetHours":7.5,"maxDailyHours":0,"everyDayCountsFrom":null},""" +
                    """"permissions":["settings:write:own"],"reminders":{"unbookedDay":true,"timerAlertHours":10}}""",
            ),
        )

        assertEquals(Account(7, "dark", "", "Europe/Berlin", true, 10, 7.5, 0.0, "", listOf("settings:write:own")), account)
        assertTrue(account.mayKeepWorkingHours)
        assertFalse(Account().mayKeepWorkingHours)
    }

    // The installation's name and banner in the app's language where it wrote
    // them for it, and its own otherwise.
    @Test
    fun theBrandingIsSaidInTheLanguageItWasWrittenFor() {
        val branding = Branding.of(
            JSONObject("""{"title":"Time Recording","banner":"Maintenance on Friday","translations":{"de":{"title":"Zeiterfassung","banner":""}}}"""),
        )

        assertEquals("Zeiterfassung", branding.titleIn("de"))
        assertEquals("Maintenance on Friday", branding.bannerIn("de"))
        assertEquals("Time Recording", branding.titleIn("en"))
    }

    // The notices are said in the terminal client's sentences: the stopwatch
    // past the hours its owner chose, the unbooked day, and a balance past a bound.
    @Test
    fun theNoticesSayWhatTheBrowsersBannersSay() {
        val notices = Notices.of(
            JSONObject("""{"unbookedDay":"2026-10-21","balance":{"hours":12.5,"upperBound":10,"lowerBound":null,"outside":"above"}}"""),
        )

        assertEquals(
            listOf(
                "The stopwatch has been running since 2026-10-22 09:12, longer than the 1 h you chose to be told after.",
                "Nothing is booked on 2026-10-21, a working day by your plan.",
                "Your running balance is +12.50 h, above the upper bound of +10.00 h you set.",
            ),
            english.notices(day(running = true, alertHours = 1), notices, elapsedHours = 1.5),
        )

        // Nothing about the stopwatch before its hours are up, or for somebody who never wants to hear it.
        assertEquals(2, english.notices(day(running = true, alertHours = 2), notices, elapsedHours = 1.5).size)
        assertEquals(2, english.notices(day(running = true, alertHours = 0), notices, elapsedHours = 30.0).size)
        assertEquals(emptyList<String>(), english.notices(day(running = false, alertHours = 1), Notices(), elapsedHours = 0.0))
    }

    // A figure is read with a point or a comma, nothing is none, and anything else is refused.
    @Test
    fun aFigureIsReadTheWaysPeopleWriteIt() {
        assertEquals(7.5, figure("7,5").getOrNull()!!, 0.0)
        assertEquals(-5.0, figure(" -5 ").getOrNull()!!, 0.0)
        assertNull(figure("").getOrNull())
        assertTrue(figure("").isSuccess)
        assertTrue(figure("eight").isFailure)
        assertTrue(figure("NaN").isFailure)
    }

    // A bound of zero is shown as the bound it is; a working hour of zero is the installation's own and shown as nothing.
    @Test
    fun aBoundOfZeroIsShownAndAWorkingHourOfZeroIsNot() {
        assertEquals("0", boundField(0.0))
        assertEquals("-2.5", boundField(-2.5))
        assertEquals("", figureField(0.0))
        assertEquals("8", figureField(8.0))
    }

    // A region is sent only with the country it belongs to.
    @Test
    fun aRegionIsSentOnlyWithItsCountry() {
        val places = HolidayPlaces.of(
            JSONObject(
                """{"place":"","instancePlace":"DE-BY","countries":[{"code":"AT","regions":[],"regionRequired":false},""" +
                    """{"code":"DE","regions":["BW","BY"],"regionRequired":true}]}""",
            ),
        )

        assertEquals("DE-BY", places.chosen("DE", "BY"))
        assertEquals("AT", places.chosen("AT", "BY"))
        assertEquals("", places.chosen("", "BY"))
        assertEquals("none", places.chosen("none", ""))
        assertEquals("Germany, Bavaria", english.placeName(places.installation))
        assertEquals("none", english.placeName("none"))
        assertEquals("State", english.regionsCalled("DE"))
    }
}
