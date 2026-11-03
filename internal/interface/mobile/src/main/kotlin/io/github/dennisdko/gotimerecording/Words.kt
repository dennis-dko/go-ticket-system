package io.github.dennisdko.gotimerecording

import java.math.BigDecimal
import java.math.RoundingMode
import java.time.LocalDate
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

/**
 * Words is what the app says things in: one of the application's dictionaries,
 * the very files internal/support/i18n/locale holds, copied into the app when it
 * is built - so the app speaks every language the installation does, and a new
 * language is still only its file.
 *
 * Its own sentences are the dictionaries' mobile.* entries. What the terminal
 * client says as well it says in its track.* words, and what the web interface
 * says - a refusal, a field, the unit of an hour - in the page's, so the three
 * never put one thing two ways. English has no file: the empty dictionary is
 * English, and a sentence a dictionary lacks is said in the English written
 * where it is said.
 */
class Words(val language: String, private val dictionary: Map<String, String>) {

    /** text is what key says, or [english] where there are no words for it, with [values] put in where it says {0}, {1} ... */
    fun text(key: String, english: String, vararg values: String): String =
        fill(dictionary[key] ?: english, *values)

    /** lookup is what key says, or null where the dictionary has nothing for it - for a caller with English of its own to fall back on. */
    fun lookup(key: String): String? = dictionary[key]

    /**
     * number writes a figure to two places the way the web interface does: 1,234.50
     * in English and 1.234,50 in German. Rounded as Go rounds it - half to even on
     * an exact tie, which 0.125 hours is - so the app, the terminal and the page
     * never disagree about the last digit.
     */
    fun number(v: Double): String {
        val point = text("number.point", ".")
        val group = text("number.group", ",")

        val fixed = BigDecimal(Math.abs(v)).setScale(2, RoundingMode.HALF_EVEN).toPlainString()
        val whole = fixed.substringBefore('.')
        val fraction = fixed.substringAfter('.')

        val grouped = StringBuilder()
        whole.forEachIndexed { i, digit ->
            if (i > 0 && (whole.length - i) % 3 == 0) grouped.append(group)
            grouped.append(digit)
        }

        val sign = if (v < 0 && (whole != "0" || fraction != "00")) "-" else ""

        return sign + grouped + point + fraction
    }

    /** hours writes a number of hours: 2.50 h, or 2,50 Std. */
    fun hours(h: Double): String = number(h) + " " + text("unit.hours", "h")

    /** day writes a calendar day the way the reader writes one, in the Go layout a dictionary names as layout.day. */
    fun day(day: LocalDate): String = DateTimeFormatter.ofPattern(pattern(text("layout.day", "2006-01-02"))).format(day)

    /** time writes the time of day, in the Go layout a dictionary names as layout.time. */
    fun time(at: ZonedDateTime): String = DateTimeFormatter.ofPattern(pattern(text("layout.time", "15:04"))).format(at)

    /** weekday writes a day of the week and its date as the terminal client heads today: Tuesday, 27 October. */
    fun weekday(day: LocalDate): String {
        val days = text("track.weekdays", "").split(' ').filter { it.isNotEmpty() }
        val months = text("track.months", "").split(' ').filter { it.isNotEmpty() }

        val name = if (days.size == 7) days[day.dayOfWeek.value % 7] else english(day.dayOfWeek.name)
        val month = if (months.size == 12) months[day.monthValue - 1] else english(day.month.name)

        return text("track.weekday", "{0}, {1} {2}", name, day.dayOfMonth.toString(), month)
    }

    companion object {

        /** english is an enum's name as English writes it: MONDAY is Monday. */
        private fun english(name: String): String = name.lowercase().replaceFirstChar { it.uppercase() }

        /**
         * fill puts values into a sentence where it says {0}, {1} ..., as the i18n
         * package and the web interface's fillIn do; a place there is no value for
         * is left as it stands.
         */
        fun fill(sentence: String, vararg values: String): String {
            if (values.isEmpty()) return sentence

            val out = StringBuilder()
            var rest = sentence

            while (rest.isNotEmpty()) {
                val open = rest.indexOf('{')
                if (open < 0) {
                    out.append(rest)
                    break
                }

                out.append(rest, 0, open)

                val after = rest.substring(open + 1)
                val close = after.indexOf('}')
                val at = if (close < 0) null else after.substring(0, close).toIntOrNull()

                if (at == null || at < 0 || at >= values.size) {
                    out.append('{')
                    rest = after
                    continue
                }

                out.append(values[at])
                rest = after.substring(close + 1)
            }

            return out.toString()
        }

        /**
         * pattern is a Go layout as a DateTimeFormatter pattern: 02.01.2006 is
         * dd'.'MM'.'yyyy. Only the parts a dictionary writes a day and a time with
         * are known - 2006, 01, 02, 15, 04, 05 - and a test in
         * internal/interface/mobile fails on a dictionary using any other, so
         * everything else is quoted as it stands.
         */
        fun pattern(layout: String): String {
            val parts = listOf("2006" to "yyyy", "01" to "MM", "02" to "dd", "15" to "HH", "04" to "mm", "05" to "ss")
            val out = StringBuilder()
            var at = 0

            while (at < layout.length) {
                val part = parts.firstOrNull { layout.startsWith(it.first, at) }
                if (part != null) {
                    out.append(part.second)
                    at += part.first.length
                    continue
                }

                val literal = layout[at]
                out.append(if (literal == '\'') "''" else "'$literal'")
                at++
            }

            return out.toString()
        }
    }
}
