package io.github.dennisdko.timepulse

import com.sun.net.httpserver.HttpServer
import kotlinx.coroutines.runBlocking
import org.json.JSONObject
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.InetSocketAddress

class UpdateTest {

    private val english = Language("en", emptyMap())

    private fun answer(version: String?, download: String?) = JSONObject().apply {
        version?.let { put("version", it) }
        download?.let { put("download", it) }
    }

    private val apk = "https://github.com/dennis-dko/timepulse/releases/download/v0.4.24/timepulse-mobile_v0.4.24.apk"

    // A release is its three numbers and nothing else, compared as numbers: the
    // tenth patch comes after the ninth, which a comparison of the words would
    // put first.
    @Test
    fun aReleaseIsReadAsItsThreeNumbers() {
        assertEquals(Release(0, 4, 23), releaseOf("v0.4.23"))
        assertTrue(releaseOf("v0.4.10")!! > releaseOf("v0.4.9")!!)
        assertTrue(releaseOf("v0.5.0")!! > releaseOf("v0.4.99")!!)
        assertTrue(releaseOf("v1.0.0")!! > releaseOf("v0.99.99")!!)

        // What a build from a source tree says, a build that is no release's, and
        // numbers too long to be anybody's.
        for (none in listOf("v0.4.23-3-g30c92cb", "v0.4.23-dirty", "dev", "", "0.4.23", "v0.4", "v99999999999.0.0")) {
            assertNull("\"$none\" read as a release", releaseOf(none))
        }
    }

    // The app of the installation's release is offered where it is newer than
    // this one - and nowhere else: not the same release, not an older one, not
    // to an app or from an installation built from source, and not without a
    // file to download.
    @Test
    fun theInstallationsAppIsOfferedWhereItIsNewer() {
        assertEquals(Offer("v0.4.24", apk), offerFor("v0.4.20", answer("v0.4.24", apk)))

        assertNull(offerFor("v0.4.24", answer("v0.4.24", apk)))
        assertNull(offerFor("v0.4.25", answer("v0.4.24", apk)))
        assertNull(offerFor("dev", answer("v0.4.24", apk)))
        assertNull(offerFor("v0.4.20", answer(null, null)))
        assertNull(offerFor("v0.4.20", answer("v0.4.24", null)))
        assertNull(offerFor("v0.4.20", answer("v0.4.24", " ")))
        assertNull(offerFor("v0.4.20", null))
    }

    // The installation is asked with the app's own token, and its answer is read
    // out of the data it is wrapped in.
    @Test
    fun theInstallationIsAskedWithTheAppsToken() {
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        var asked = ""

        server.createContext("/api/v1/me/android") { exchange ->
            asked = exchange.requestHeaders.getFirst("Authorization").orEmpty()
            val body = """{"data":{"release":"page","download":"$apk","version":"v0.4.24"}}""".toByteArray()

            exchange.sendResponseHeaders(200, body.size.toLong())
            exchange.responseBody.use { it.write(body) }
        }

        server.start()

        try {
            val address = "http://127.0.0.1:${server.address.port}"
            val answered = runBlocking { Installation(address, english).android("timepulse_mine") }

            assertEquals("Bearer timepulse_mine", asked)
            assertEquals(Offer("v0.4.24", apk), offerFor("v0.4.20", answered))
        } finally {
            server.stop(0)
        }
    }

    // What is downloaded is written whole, and the share written is told each
    // hundredth along the way - up to the whole of it - where the size is known.
    @Test
    fun aDownloadIsWrittenWholeAndToldAsItGoes() {
        val bytes = ByteArray(300_000) { (it % 251).toByte() }
        val written = ByteArrayOutputStream()
        val told = mutableListOf<Float>()

        transfer(ByteArrayInputStream(bytes), written, bytes.size.toLong()) { told += it }

        assertArrayEquals(bytes, written.toByteArray())
        assertEquals(1f, told.last())
        assertTrue("told $told", told.zipWithNext().all { (a, b) -> b > a })

        val unknown = mutableListOf<Float>()
        transfer(ByteArrayInputStream(bytes), ByteArrayOutputStream(), -1) { unknown += it }

        assertTrue("told $unknown for a download of no known size", unknown.isEmpty())
    }
}
