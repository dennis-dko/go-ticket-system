package io.github.dennisdko.goticketsystem

import java.time.Duration
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId

/**
 * Reminders is what the installation says its owner chose to be reminded of, as
 * the app asks it with its token: after how many hours a running stopwatch is
 * mentioned - zero for never - when the one running started, the last working
 * day with nothing booked on it, which the installation leaves out when its
 * owner asked not to be told, and whether they asked to be told of one at all.
 *
 * The rules are the installation's, and the app keeps no copy of them: it shows
 * what the installation answered a moment before, and nothing it has not just
 * been told.
 */
data class Reminders(
    val alertHours: Int,
    val startedAt: Instant?,
    val unbookedDay: LocalDate?,
    val unbookedReminder: Boolean = false,
)

/** Notice is one notification the app shows: what it is about, so it is shown once, and its words. */
data class Notice(val key: String, val title: String, val body: String)

/**
 * noticesDue is what is to be shown now and has not been already: the stopwatch,
 * once it has run past the hours chosen, and the unbooked day - not before
 * [morning] on the phone's clock, because the installation names yesterday as
 * soon as it is past midnight, and nobody is to be woken for it.
 */
fun noticesDue(reminders: Reminders, now: Instant, zone: ZoneId, told: Set<String>, words: Language): List<Notice> {
    val due = mutableListOf<Notice>()

    val started = reminders.startedAt
    val alertAt = stopwatchDue(reminders)
    if (started != null && alertAt != null && !now.isBefore(alertAt)) {
        val key = "stopwatch:$started"
        val at = started.atZone(zone)

        if (key !in told) {
            due += Notice(
                key,
                words.text("timer.longTitle", "The stopwatch is still running"),
                words.text(
                    "timer.longBody",
                    "Since {1}, longer than the {0} you chose.",
                    "${reminders.alertHours} ${words.text("unit.hours", "h")}",
                    words.day(at.toLocalDate()) + " " + words.time(at),
                ),
            )
        }
    }

    val day = reminders.unbookedDay
    if (day != null && now.atZone(zone).hour >= morning) {
        val key = "unbooked:$day"

        if (key !in told) {
            due += Notice(
                key,
                words.text("remind.title", "Reminders"),
                words.text(
                    "remind.unbooked",
                    "Nothing is booked on {0}, a working day by your plan. It can be added under {1}.",
                    words.day(day),
                    words.text("nav.timesheets", "Time entries"),
                ),
            )
        }
    }

    return due
}

/**
 * stillTold is which of the notices already shown are still about something
 * true, so that a stopwatch that was stopped, or a day booked since, is forgotten
 * and the list does not grow for ever.
 */
fun stillTold(reminders: Reminders, told: Set<String>): Set<String> = told.filterTo(mutableSetOf()) {
    it == "stopwatch:${reminders.startedAt}" || it == "unbooked:${reminders.unbookedDay}"
}

/** stopwatchDue is when the running stopwatch passes the hours chosen, or null where it is not running or nothing was chosen. */
fun stopwatchDue(reminders: Reminders): Instant? {
    val started = reminders.startedAt ?: return null
    if (reminders.alertHours <= 0) return null

    return started.plus(Duration.ofHours(reminders.alertHours.toLong()))
}

/**
 * nextCheck is when the installation is to be asked again rather than at the
 * next quarter-hourly check, which a dozing phone holds back until it wakes: the
 * moment the running stopwatch passes the hours chosen, or the next [morning]
 * on the phone's clock for somebody reminded of an unbooked day - whichever
 * comes first, and null where neither lies ahead.
 */
fun nextCheck(reminders: Reminders, now: Instant, zone: ZoneId): Instant? {
    val stopwatch = stopwatchDue(reminders)?.takeIf { it.isAfter(now) }
    val unbooked = if (reminders.unbookedReminder) nextMorning(now, zone) else null

    return listOfNotNull(stopwatch, unbooked).minOrNull()
}

private fun nextMorning(now: Instant, zone: ZoneId): Instant {
    val today = now.atZone(zone).toLocalDate()
    val atMorning = today.atTime(morning, 0).atZone(zone).toInstant()

    return if (atMorning.isAfter(now)) atMorning else today.plusDays(1).atTime(morning, 0).atZone(zone).toInstant()
}

/** morning is the hour on the phone's clock before which an unbooked day is not mentioned. */
const val morning = 8
