package io.github.dennisdko.goticketsystem

import org.junit.Assert.assertEquals
import org.junit.Test
import java.time.LocalDate

class LanguageTest {

    private val english = Language("en", emptyMap())

    private val german = Language(
        "de",
        mapOf(
            "number.point" to ",",
            "number.group" to ".",
            "unit.hours" to "Std.",
            "layout.day" to "02.01.2006",
            "layout.time" to "15:04",
        ),
    )

    // A figure is written to the last digit as the i18n package writes it, which
    // is how the page writes it too. Every expectation is
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
        assertEquals("Booked 1 on A.", Language.fill("Booked {0} on {1}.", "1", "A"))
        assertEquals("b a", Language.fill("{1} {0}", "a", "b"))
        assertEquals("{x} and {", Language.fill("{x} and {", "v"))
        assertEquals("{2} stays", Language.fill("{2} stays", "x"))
        assertEquals("b", Language.fill("{+1}", "a", "b"))
        assertEquals("nothing to fill {0}", Language.fill("nothing to fill {0}"))
    }

    @Test
    fun aDayIsWrittenInTheLayoutTheDictionaryNames() {
        assertEquals("dd'.'MM'.'yyyy", Language.pattern("02.01.2006"))
        assertEquals("yyyy'-'MM'-'dd", Language.pattern("2006-01-02"))
        assertEquals("HH':'mm", Language.pattern("15:04"))

        val day = LocalDate.of(2026, 10, 29)

        assertEquals("2026-10-29", english.day(day))
        assertEquals("29.10.2026", german.day(day))
    }

    @Test
    fun aSentenceADictionaryLacksIsSaidInEnglish() {
        assertEquals("Reading tickets.example.com …", german.text("mobile.reading", "Reading {0} …", "tickets.example.com"))
        assertEquals("Std.", german.text("unit.hours", "h"))
    }
}
