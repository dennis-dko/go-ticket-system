package io.github.dennisdko.goticketsystem

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId

class ReminderTest {

    private val english = Language("en", emptyMap())

    private val german = Language(
        "de",
        mapOf(
            "unit.hours" to "Std.",
            "layout.day" to "02.01.2006",
            "layout.time" to "15:04",
            "timer.longTitle" to "Die Stoppuhr läuft noch",
            "timer.longBody" to "Seit {1}, länger als die {0}, die du gewählt hast.",
            "remind.title" to "Erinnerungen",
            "remind.unbooked" to "Am {0} ist nichts gebucht, nach deinem Plan ein Arbeitstag. Unter {1} lässt es sich nachtragen.",
            "nav.timesheets" to "Zeiteinträge",
        ),
    )

    private val berlin = ZoneId.of("Europe/Berlin")
    private val started = Instant.parse("2026-11-10T07:15:00Z")

    // A stopwatch past the hours its owner chose is mentioned once, in the words
    // the page has for it, with the moment it started on the phone's clock.
    @Test
    fun aStopwatchPastTheHoursChosenIsMentionedOnce() {
        val reminders = Reminders(alertHours = 2, startedAt = started, unbookedDay = null)
        val after = started.plusSeconds(2 * 3600 + 60)

        val due = noticesDue(reminders, after, berlin, emptySet(), german)

        assertEquals(1, due.size)
        assertEquals("Die Stoppuhr läuft noch", due[0].title)
        assertEquals("Seit 10.11.2026 08:15, länger als die 2 Std., die du gewählt hast.", due[0].body)

        assertTrue(noticesDue(reminders, after, berlin, setOf(due[0].key), german).isEmpty())
    }

    // Not before the hours have passed, and never where none were chosen or
    // nothing is running.
    @Test
    fun aStopwatchIsNotMentionedTooEarlyOrUnasked() {
        val reminders = Reminders(alertHours = 2, startedAt = started, unbookedDay = null)

        assertTrue(noticesDue(reminders, started.plusSeconds(2 * 3600 - 1), berlin, emptySet(), english).isEmpty())
        assertTrue(noticesDue(reminders.copy(alertHours = 0), started.plusSeconds(9 * 3600), berlin, emptySet(), english).isEmpty())
        assertTrue(noticesDue(reminders.copy(startedAt = null), started.plusSeconds(9 * 3600), berlin, emptySet(), english).isEmpty())
    }

    // The installation names an unbooked day as soon as it is past midnight;
    // the phone says it from eight in the morning on its own clock.
    @Test
    fun anUnbookedDayIsMentionedFromTheMorningOn() {
        val reminders = Reminders(alertHours = 0, startedAt = null, unbookedDay = LocalDate.parse("2026-11-09"))

        assertTrue(noticesDue(reminders, Instant.parse("2026-11-10T06:59:00Z"), berlin, emptySet(), german).isEmpty())

        val due = noticesDue(reminders, Instant.parse("2026-11-10T07:00:00Z"), berlin, emptySet(), german)

        assertEquals(1, due.size)
        assertEquals("Erinnerungen", due[0].title)
        assertEquals("Am 09.11.2026 ist nichts gebucht, nach deinem Plan ein Arbeitstag. Unter Zeiteinträge lässt es sich nachtragen.", due[0].body)
    }

    // What was shown is forgotten once it stops being true, so a stopwatch
    // stopped and a day booked are not remembered for ever.
    @Test
    fun whatStoppedBeingTrueIsForgotten() {
        val told = setOf("stopwatch:$started", "unbooked:2026-11-09", "unbooked:2026-11-02")

        assertEquals(
            setOf("stopwatch:$started", "unbooked:2026-11-09"),
            stillTold(Reminders(2, started, LocalDate.parse("2026-11-09")), told),
        )
        assertTrue(stillTold(Reminders(2, null, null), told).isEmpty())
    }

    // The moment the stopwatch is due is asked about then, rather than up to a
    // quarter of an hour later.
    @Test
    fun theStopwatchIsDueWhenItsHoursHavePassed() {
        assertEquals(started.plusSeconds(3 * 3600), stopwatchDue(Reminders(3, started, null)))
        assertNull(stopwatchDue(Reminders(0, started, null)))
        assertNull(stopwatchDue(Reminders(3, null, null)))
    }

    // A dozing phone holds the quarter-hourly check back, so the installation is
    // asked again at the next moment a reminder falls due: the stopwatch's, or
    // eight in the morning for somebody reminded of an unbooked day.
    @Test
    fun theInstallationIsAskedAgainWhenTheNextReminderFallsDue() {
        val running = Reminders(alertHours = 2, startedAt = started, unbookedDay = null)

        assertEquals(started.plusSeconds(2 * 3600), nextCheck(running, started.plusSeconds(60), berlin))
        assertNull(nextCheck(running, started.plusSeconds(2 * 3600), berlin))

        val mornings = Reminders(alertHours = 0, startedAt = null, unbookedDay = null, unbookedReminder = true)

        assertEquals(Instant.parse("2026-11-10T07:00:00Z"), nextCheck(mornings, Instant.parse("2026-11-10T06:59:00Z"), berlin))
        assertEquals(Instant.parse("2026-11-11T07:00:00Z"), nextCheck(mornings, Instant.parse("2026-11-10T07:00:00Z"), berlin))
        assertEquals(Instant.parse("2026-11-11T07:00:00Z"), nextCheck(mornings.copy(unbookedDay = LocalDate.parse("2026-11-09")), Instant.parse("2026-11-10T16:00:00Z"), berlin))

        assertNull(nextCheck(Reminders(alertHours = 0, startedAt = null, unbookedDay = null), started, berlin))
    }

    // Both asked for, whichever comes first: the stopwatch during the day, the
    // morning for one started late in the evening.
    @Test
    fun theEarlierOfTwoRemindersIsAskedAboutFirst() {
        val both = Reminders(alertHours = 2, startedAt = started, unbookedDay = null, unbookedReminder = true)

        assertEquals(started.plusSeconds(2 * 3600), nextCheck(both, started.plusSeconds(60), berlin))

        val evening = both.copy(alertHours = 12, startedAt = Instant.parse("2026-11-10T21:00:00Z"))

        assertEquals(Instant.parse("2026-11-11T07:00:00Z"), nextCheck(evening, Instant.parse("2026-11-10T21:30:00Z"), berlin))
    }
}
