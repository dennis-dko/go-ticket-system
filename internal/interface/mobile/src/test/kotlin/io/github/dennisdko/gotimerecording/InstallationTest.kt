package io.github.dennisdko.gotimerecording

import com.sun.net.httpserver.HttpServer
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Test
import java.net.InetSocketAddress

class InstallationTest {

    private val english = Language("en", emptyMap())

    /** answering serves [status], with a refusal's body, to every request on a port of its own while [block] runs. */
    private fun <T> answering(status: Int, block: (String) -> T): T {
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)

        server.createContext("/") { exchange ->
            val body = """{"error":{"code":"internal","message":"something went wrong"}}""".toByteArray()

            exchange.responseHeaders.add("Content-Type", "application/json")
            exchange.sendResponseHeaders(status, body.size.toLong())
            exchange.responseBody.use { it.write(body) }
        }

        server.start()

        try {
            return block("http://127.0.0.1:${server.address.port}")
        } finally {
            server.stop(0)
        }
    }

    // A token the installation no longer knows - taken back, or run out - is told
    // apart from every other refusal, because the app answers it by disconnecting
    // rather than by waiting to try again: whether it opens the web interface's
    // session or asks about the reminders.
    @Test
    fun aTokenTakenBackIsToldApartFromOtherRefusals() {
        answering(401) { address ->
            assertThrows(TokenRefused::class.java) { runBlocking { Installation(address, english).handOver("gtr_gone") } }
            assertThrows(TokenRefused::class.java) { runBlocking { Installation(address, english).reminders("gtr_gone") } }
        }

        answering(500) { address ->
            val opening = assertThrows(Refused::class.java) { runBlocking { Installation(address, english).handOver("gtr_kept") } }
            val asking = assertThrows(Refused::class.java) { runBlocking { Installation(address, english).reminders("gtr_kept") } }

            assertFalse("a failing installation is taken for a token taken back", opening is TokenRefused || asking is TokenRefused)
        }
    }
}
