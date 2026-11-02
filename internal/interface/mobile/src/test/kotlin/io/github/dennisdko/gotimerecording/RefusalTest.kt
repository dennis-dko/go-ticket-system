package io.github.dennisdko.gotimerecording

import org.junit.Assert.assertEquals
import org.junit.Test

class RefusalTest {

    private val english = Words("en", emptyMap())

    private val german = Words(
        "de",
        mapOf(
            "number.point" to ",",
            "number.group" to ".",
            "layout.day" to "02.01.2006",
            "err.projectClosedForBooking" to "„{0}“ ist {1}.",
            "err.overDailyLimit" to "{0} über {1} am {2}",
            "err.invalidFields" to "Ungültige Felder",
            "err.maintenance" to "Wartung.",
            "status.completed" to "abgeschlossen",
            "field.durationHours" to "Stunden",
        ),
    )

    private val closed = """{"error":{"code":"projectClosedForBooking","message":"project Website is completed","values":["Website","completed"]}}"""

    @Test
    fun aTokenTheInstallationDoesNotKnowIsSaidInTheAppsWords() {
        assertEquals(
            "the installation does not know this token - make a new one under My account, API tokens, and scan its code",
            english.refused(401, "Unauthorized", """{"error":{"message":"not signed in"}}"""),
        )
    }

    // In the page's words, with a status in its words too - not as it travels.
    @Test
    fun aRefusalIsSaidAsThePageSaysIt() {
        assertEquals("„Website“ ist abgeschlossen.", german.refused(422, "Unprocessable Entity", closed))
    }

    // The page's English is the fallbacks in its own script, so English is the
    // installation's own sentence.
    @Test
    fun inEnglishTheInstallationsOwnSentenceIsSaid() {
        assertEquals("project Website is completed", english.refused(422, "Unprocessable Entity", closed))
    }

    @Test
    fun aDayAndTheFiguresAreWrittenAsTheScreenWritesThem() {
        val over = """{"error":{"code":"overDailyLimit","values":[9.5,8,"2026-10-29"]}}"""

        assertEquals("9,50 über 8 am 29.10.2026", german.refused(422, "Unprocessable Entity", over))
    }

    @Test
    fun theFieldsAreNamedBeforeAnySentence() {
        val invalid = """{"error":{"code":"badRequest","message":"invalid","param":["durationHours","comment"]}}"""

        assertEquals("Ungültige Felder: Stunden, comment", german.refused(400, "Bad Request", invalid))
    }

    @Test
    fun maintenanceIsSaidAsThePageSaysIt() {
        assertEquals("Wartung.", german.refused(503, "Service Unavailable", """{"error":{"code":"maintenance"}}"""))
    }

    @Test
    fun anAnswerWithNothingToSayIsNamedByItsStatus() {
        assertEquals("the installation answered 502 Bad Gateway", english.refused(502, "Bad Gateway", "<html>"))
    }
}
