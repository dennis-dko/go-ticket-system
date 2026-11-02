package io.github.dennisdko.gotimerecording

import org.junit.Assert.assertEquals
import org.junit.Test
import java.time.LocalDate

class WordsTest {

    private val english = Words("en", emptyMap())

    private val german = Words(
        "de",
        mapOf(
            "number.point" to ",",
            "number.group" to ".",
            "unit.hours" to "Std.",
            "layout.day" to "02.01.2006",
            "layout.time" to "15:04",
            "track.weekday" to "{0}, {1}. {2}",
            "track.weekdays" to "Sonntag Montag Dienstag Mittwoch Donnerstag Freitag Samstag",
            "track.months" to "Januar Februar März April Mai Juni Juli August September Oktober November Dezember",
        ),
    )

    // A figure is written to the last digit as the i18n package writes it, which
    // is how the page and the terminal client write it too. Every expectation is
    // what Go printed for the same value, not a reading of how Go rounds - the
    // ties and the near-ties are where a port goes wrong.
    @Test
    fun aFigureIsWrittenAsGoWritesIt() {
        val written = mapOf(
            1234.5 to ("1,234.50" to "1.234,50"),
            0.125 to ("0.12" to "0,12"),
            0.375 to ("0.38" to "0,38"),
            2.675 to ("2.67" to "2,67"),
            -2.0 to ("-2.00" to "-2,00"),
            -0.001 to ("0.00" to "0,00"),
            1e6 to ("1,000,000.00" to "1.000.000,00"),
            0.005 to ("0.01" to "0,01"),
            0.015 to ("0.01" to "0,01"),
        )

        written.forEach { (value, both) ->
            assertEquals("English $value", both.first, english.number(value))
            assertEquals("German $value", both.second, german.number(value))
        }

        assertEquals("1.50 h", english.hours(1.5))
        assertEquals("1,50 Std.", german.hours(1.5))
    }

    @Test
    fun valuesGoWhereTheSentenceSaysAndNowhereElse() {
        assertEquals("Booked 1 on A.", Words.fill("Booked {0} on {1}.", "1", "A"))
        assertEquals("b a", Words.fill("{1} {0}", "a", "b"))
        assertEquals("{x} and {", Words.fill("{x} and {", "v"))
        assertEquals("{2} stays", Words.fill("{2} stays", "x"))
        assertEquals("b", Words.fill("{+1}", "a", "b"))
        assertEquals("nothing to fill {0}", Words.fill("nothing to fill {0}"))
    }

    @Test
    fun aDayIsWrittenInTheLayoutTheDictionaryNames() {
        assertEquals("dd'.'MM'.'yyyy", Words.pattern("02.01.2006"))
        assertEquals("yyyy'-'MM'-'dd", Words.pattern("2006-01-02"))
        assertEquals("HH':'mm", Words.pattern("15:04"))

        val day = LocalDate.of(2026, 10, 29)

        assertEquals("2026-10-29", english.day(day))
        assertEquals("29.10.2026", german.day(day))
        assertEquals("Thursday, 29 October", english.weekday(day))
        assertEquals("Donnerstag, 29. Oktober", german.weekday(day))
    }

    @Test
    fun aSentenceADictionaryLacksIsSaidInEnglish() {
        assertEquals("Started at 09:12 on A.", german.text("track.started", "Started at {0} on {1}.", "09:12", "A"))
        assertEquals("Std.", german.text("unit.hours", "h"))
    }
}
