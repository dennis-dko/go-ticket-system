package io.github.dennisdko.goticketsystem

import org.junit.Assert.assertEquals
import org.junit.Test

class RefusalTest {

    private val english = Language("en", emptyMap())

    private val german = Language(
        "de",
        mapOf(
            "number.point" to ",",
            "number.group" to ".",
            "err.notFound" to "{0} mit der Kennung {1} wurde nicht gefunden.",
            "err.overTheLimit" to "{0} über {1}",
            "err.invalidFields" to "Ungültige Felder",
            "err.maintenance" to "Wartung.",
            "entity.role" to "Rolle",
            "field.name" to "Name",
        ),
    )

    private val missing = """{"error":{"code":"notFound","message":"role with id 42 not found","values":["role",42]}}"""

    @Test
    fun aTokenTheInstallationDoesNotKnowIsSaidInTheAppsWords() {
        assertEquals(
            "the installation does not know this token - make a new one under My account, API tokens, and scan its code",
            english.refused(401, "Unauthorized", """{"error":{"message":"not signed in"}}"""),
        )
    }

    // In the page's words, with the kind of record in its words too - not as it travels.
    @Test
    fun aRefusalIsSaidAsThePageSaysIt() {
        assertEquals("Rolle mit der Kennung 42 wurde nicht gefunden.", german.refused(404, "Not Found", missing))
    }

    // The page's English is the fallbacks in its own script, so English is the
    // installation's own sentence.
    @Test
    fun inEnglishTheInstallationsOwnSentenceIsSaid() {
        assertEquals("role with id 42 not found", english.refused(404, "Not Found", missing))
    }

    @Test
    fun theFiguresAreWrittenAsTheScreenWritesThem() {
        val over = """{"error":{"code":"overTheLimit","values":[9.5,8]}}"""

        assertEquals("9,50 über 8", german.refused(422, "Unprocessable Entity", over))
    }

    @Test
    fun theFieldsAreNamedBeforeAnySentence() {
        val invalid = """{"error":{"code":"badRequest","message":"invalid","param":["name","comment"]}}"""

        assertEquals("Ungültige Felder: Name, comment", german.refused(400, "Bad Request", invalid))
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
