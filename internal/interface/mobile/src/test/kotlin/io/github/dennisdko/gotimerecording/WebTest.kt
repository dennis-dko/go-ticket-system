package io.github.dennisdko.gotimerecording

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WebTest {

    // The installation's own pages are shown in the app; anything else opens in
    // the phone's browser, where its owner's rules for it apply.
    @Test
    fun onlyTheInstallationsOwnPagesAreShownInTheApp() {
        val home = "https://time.example.com"

        assertTrue(belongsTo(home, "https://time.example.com/"))
        assertTrue(belongsTo(home, "https://time.example.com:443/api-docs"))
        assertTrue(belongsTo("http://192.168.1.20:8000", "http://192.168.1.20:8000/#week"))
        assertTrue(belongsTo(home, "https://TIME.example.com/"))

        assertFalse(belongsTo(home, "https://github.com/dennis-dko/go-time-recording"))
        assertFalse(belongsTo(home, "http://time.example.com/"))
        assertFalse(belongsTo(home, "https://time.example.com:8443/"))
        assertFalse(belongsTo(home, "https://time.example.com.evil.example/"))
        assertFalse(belongsTo(home, "not an address"))
    }

    // Signing out in the web interface is what disconnects the app - its own
    // sign-out, and nobody else's.
    @Test
    fun signingOutIsTheInstallationsOwnSignOut() {
        val home = "https://time.example.com"

        assertTrue(signsOut(home, "https://time.example.com/api/v1/auth/logout"))
        assertFalse(signsOut(home, "https://time.example.com/api/v1/auth/login"))
        assertFalse(signsOut(home, "https://elsewhere.example.com/api/v1/auth/logout"))
    }

    // The app signs out with the token the session's own cookie holds, which is
    // the one the installation compares the header against.
    @Test
    fun theSignOutCarriesTheTokenOfTheCookieBesideIt() {
        val held = "x_gtr_csrf=other; gtr_session=s3ss10n; gtr_csrf=c5rf"

        assertEquals("c5rf", cookieValue(held, "gtr_csrf"))
        assertEquals("s3ss10n", cookieValue(held, "gtr_session"))
        assertEquals(null, cookieValue("gtr_session=s3ss10n", "gtr_csrf"))
    }

    // A file is saved under the page's name for it, and never under one that
    // names a folder.
    @Test
    fun aFileIsSavedUnderTheNameThePageGaveIt() {
        assertEquals("time-entries-2026-11-03.xlsx", downloadName("time-entries-2026-11-03.xlsx"))
        assertEquals("report.pdf", downloadName("../../report.pdf"))
        assertEquals("report.pdf", downloadName("..\\report.pdf"))
        assertEquals("download", downloadName("  "))
    }
}
