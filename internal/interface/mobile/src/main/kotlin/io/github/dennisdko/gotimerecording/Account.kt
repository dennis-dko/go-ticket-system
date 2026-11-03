package io.github.dennisdko.gotimerecording

import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId

/**
 * Account is what the token's owner set for themselves, as /me answers - which
 * the app follows, and offers to change. A change is saved to the account, as
 * My account in the browser saves it, so that it holds there and in the terminal
 * as well: the maintainer's rule since 2026-11-03.
 */
data class Account(
    val id: Long = 0,
    /** theme is light or dark, empty for following the phone. */
    val theme: String = "",
    /** timezone is the zone they chose, empty for the installation's, which [installationZone] names. */
    val timezone: String = "",
    val installationZone: String = "",
    val unbookedDay: Boolean = false,
    /** alertHours is after how many hours a running stopwatch is mentioned, zero for never. */
    val alertHours: Int = 0,
    /** targetHours and maxHours are their working hours, zero for the installation's. */
    val targetHours: Double = 0.0,
    val maxHours: Double = 0.0,
    /** countsFrom is the day every working day counts from, empty for none. */
    val countsFrom: String = "",
    val permissions: List<String> = emptyList(),
) {
    /** mayKeepWorkingHours says whether the account may set its own working hours, as the browser asks. */
    val mayKeepWorkingHours: Boolean get() = "settings:write:own" in permissions

    companion object {
        /** of reads an account from /me's answer. */
        fun of(me: JSONObject): Account {
            val user = me.optJSONObject("user") ?: JSONObject()
            val reminders = me.optJSONObject("reminders") ?: JSONObject()
            val permissions = me.optJSONArray("permissions") ?: JSONArray()

            return Account(
                id = user.optLong("id"),
                theme = user.textAt("theme"),
                timezone = user.textAt("timezone"),
                installationZone = user.textAt("instanceTimezone"),
                unbookedDay = reminders.optBoolean("unbookedDay"),
                alertHours = reminders.optInt("timerAlertHours"),
                targetHours = user.optDouble("dailyTargetHours", 0.0),
                maxHours = user.optDouble("maxDailyHours", 0.0),
                countsFrom = user.textAt("everyDayCountsFrom"),
                permissions = (0 until permissions.length()).map { permissions.getString(it) },
            )
        }
    }
}

/** Branding is what the installation calls itself and says to everybody, and what it wrote for each language. */
data class Branding(val title: String = "", val banner: String = "", val translated: Map<String, Pair<String, String>> = emptyMap()) {

    /** titleIn is the name in a language, or the one the installation has. */
    fun titleIn(language: String): String = translated[language]?.first?.takeIf { it.isNotBlank() } ?: title

    /** bannerIn is the banner in a language, or the one the installation has. */
    fun bannerIn(language: String): String = translated[language]?.second?.takeIf { it.isNotBlank() } ?: banner

    companion object {
        /** of reads the branding from its answer. */
        fun of(branding: JSONObject): Branding {
            val translations = branding.optJSONObject("translations") ?: JSONObject()

            return Branding(
                title = branding.textAt("title").trim(),
                banner = branding.textAt("banner").trim(),
                translated = translations.keys().asSequence().associateWith {
                    val texts = translations.getJSONObject(it)
                    texts.textAt("title").trim() to texts.textAt("banner").trim()
                },
            )
        }
    }
}

/**
 * Notices are what about the owner's own time deserves a banner, as the
 * installation works it out for the browser: the last working day with nothing
 * booked on it, and a running balance with the bounds kept around it - the
 * balance only for somebody who may read what it is made of.
 */
data class Notices(val unbookedDay: String = "", val balance: Balance? = null) {

    companion object {
        /** of reads the notices from their answer. */
        fun of(notices: JSONObject): Notices {
            val balance = notices.optJSONObject("balance")

            return Notices(
                unbookedDay = notices.textAt("unbookedDay"),
                balance = balance?.let {
                    Balance(it.optDouble("hours", 0.0), it.numberAt("upperBound"), it.numberAt("lowerBound"), it.textAt("outside"))
                },
            )
        }
    }
}

/** Balance is the running balance, its bounds - null for none - and which one it is past, if any. */
data class Balance(val hours: Double, val upper: Double?, val lower: Double?, val outside: String)

/** HolidayCountry is a country whose holidays can be kept, with its regions and whether one has to be named. */
data class HolidayCountry(val code: String, val regions: List<String>, val regionRequired: Boolean)

/** HolidayPlaces is where the owner keeps public holidays: empty follows the installation, none is none. */
data class HolidayPlaces(val place: String = "", val installation: String = "", val countries: List<HolidayCountry> = emptyList()) {

    /** country is the country with a code, null where none is offered by it. */
    fun country(code: String): HolidayCountry? = countries.firstOrNull { it.code == code }

    /**
     * chosen is the place a country and a region name: following the installation,
     * none, or the country with the region - the region only where it is one of
     * that country's, since it may have been picked for another country first.
     */
    fun chosen(country: String, region: String): String {
        val offered = country(country) ?: return country
        if (region !in offered.regions) return offered.code

        return offered.code + "-" + region
    }

    companion object {
        /** of reads the places from /me/holidays' answer. */
        fun of(holidays: JSONObject): HolidayPlaces {
            val countries = holidays.optJSONArray("countries") ?: JSONArray()

            return HolidayPlaces(
                place = holidays.textAt("place"),
                installation = holidays.textAt("instancePlace"),
                countries = (0 until countries.length()).map { i ->
                    val c = countries.getJSONObject(i)
                    val regions = c.optJSONArray("regions") ?: JSONArray()
                    HolidayCountry(c.textAt("code"), (0 until regions.length()).map { regions.getString(it) }, c.optBoolean("regionRequired"))
                },
            )
        }
    }
}

/**
 * notices are the notices as the app says them, in the terminal client's
 * sentences: the stopwatch first, where it has run longer than its owner wanted
 * to be told after.
 */
fun Language.notices(snapshot: Snapshot, notices: Notices, elapsedHours: Double): List<String> {
    val said = mutableListOf<String>()
    val clock = snapshot.clock
    val alert = snapshot.account.alertHours

    if (clock.running && clock.startedAt != null && alert > 0 && elapsedHours >= alert) {
        said += text(
            "track.notice.longRunning", "The stopwatch has been running since {0}, longer than the {1} you chose to be told after.",
            moment(clock.startedAt, snapshot.zone, snapshot.today), alert.toString() + " " + text("unit.hours", "h"),
        )
    }

    if (notices.unbookedDay.isNotEmpty()) {
        val day = runCatching { LocalDate.parse(notices.unbookedDay) }.getOrNull()
        said += text("track.notice.unbooked", "Nothing is booked on {0}, a working day by your plan.", day?.let { day(it) } ?: notices.unbookedDay)
    }

    val balance = notices.balance
    if (balance != null && balance.outside == "above" && balance.upper != null) {
        said += text(
            "track.notice.above", "Your running balance is {0}, above the upper bound of {1} you set.",
            signedHours(balance.hours), signedHours(balance.upper),
        )
    }

    if (balance != null && balance.outside == "below" && balance.lower != null) {
        said += text(
            "track.notice.below", "Your running balance is {0}, below the lower bound of {1} you set.",
            signedHours(balance.hours), signedHours(balance.lower),
        )
    }

    return said
}

/** moment writes when something happened as the terminal client does: the time alone today, the day before it otherwise. */
fun Language.moment(at: Instant, zone: ZoneId, today: LocalDate): String {
    val there = at.atZone(zone)
    if (there.toLocalDate() == today) return time(there)

    return day(there.toLocalDate()) + " " + time(there)
}

/** signedHours writes a balance as the browser does: a plus before a credit. */
fun Language.signedHours(h: Double): String = (if (h > 0) "+" else "") + hours(h)

/** figure reads a number of hours somebody typed, with a decimal point or comma; null for nothing, and a failure for anything else. */
fun figure(raw: String): Result<Double?> {
    val written = raw.trim()
    if (written.isEmpty()) return Result.success(null)

    val read = written.replaceFirst(',', '.').toDoubleOrNull()
    if (read == null || read.isNaN() || read.isInfinite()) return Result.failure(IllegalArgumentException(written))

    return Result.success(read)
}

/** figureField is a number as a field shows it to be changed: empty for zero, which is the installation's own. */
fun figureField(v: Double): String = if (v == 0.0) "" else boundField(v)

/** boundField is a bound as a field shows it, zero included: a bound of zero is a bound. */
fun boundField(v: Double): String = if (v == Math.floor(v)) v.toLong().toString() else v.toString()

private fun JSONObject.textAt(name: String): String = if (isNull(name)) "" else optString(name)

private fun JSONObject.numberAt(name: String): Double? = if (isNull(name) || !has(name)) null else optDouble(name)
