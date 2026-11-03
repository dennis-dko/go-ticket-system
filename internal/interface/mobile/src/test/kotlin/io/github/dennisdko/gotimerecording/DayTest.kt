package io.github.dennisdko.gotimerecording

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class DayTest {

    private val english = Words("en", emptyMap())

    @Test
    fun hoursAreReadTheWaysPeopleWriteThem() {
        assertEquals(1.5, english.parseHours("1.5"), 0.0)
        assertEquals(1.5, english.parseHours("1,5"), 0.0)
        assertEquals(1.5, english.parseHours("1:30"), 0.0)
        assertEquals(0.25, english.parseHours(" 0:15 "), 0.0)
    }

    // The terminal client refuses exactly these, saying how to write hours instead.
    @Test
    fun whatIsNotANumberOfHoursIsRefusedSayingHowToWriteOne() {
        listOf("0", "-1", "abc", "1:60", "0:00", "1:-5", "1:30:00", "1,000.5", "", "NaN", "Infinity").forEach {
            val refused = runCatching { english.parseHours(it) }.exceptionOrNull()

            assertTrue("\"$it\" was taken as hours", refused is Refused)
            assertTrue(refused?.message.orEmpty(), refused?.message.orEmpty().endsWith("write 1.5, 1,5 or 1:30"))
        }
    }

    @Test
    fun aRunningClockIsWrittenAsAClockWould() {
        assertEquals("1:23", hoursAndMinutes(1.3833))
        assertEquals("0:00", hoursAndMinutes(0.0))
        assertEquals("10:05", hoursAndMinutes(10.0833))
    }

    // The account's language first, as the terminal client takes it once nobody
    // asked for one on the command line, then the phone's, then English.
    @Test
    fun theAccountsLanguageComesBeforeThePhones() {
        val spoken = setOf("de")

        assertEquals("de", languageFor("de", "fr", spoken))
        assertEquals("de", languageFor("", "de", spoken))
        assertEquals("en", languageFor("en", "de", spoken))
        assertEquals("de", languageFor("fr", "de", spoken))
        assertEquals("en", languageFor("fr", "es", spoken))
    }

    @Test
    fun aProjectIsNamedAndNoProjectIsSaid() {
        val projects = listOf(Project(7, "Website", "active"))

        assertEquals("Website", english.nameOf(projects, 7))
        assertEquals("project 9", english.nameOf(projects, 9))
        assertEquals("no project", english.nameOf(projects, null))
    }
}
