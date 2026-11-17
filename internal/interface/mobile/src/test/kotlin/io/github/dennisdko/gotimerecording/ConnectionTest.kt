package io.github.dennisdko.gotimerecording

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ConnectionTest {

    // The address a token's code holds, written the way the server writes it:
    // url.Values encodes the two query-encoded and in the order of their names.
    @Test
    fun theAddressATokensCodeHoldsIsRead() {
        assertEquals(
            Connection("https://time.example.com", "timepulse_abc123"),
            connectionIn("timepulse://connect?token=timepulse_abc123&url=https%3A%2F%2Ftime.example.com"),
        )
    }

    // An installation from before the rename to TimePulse writes its old scheme
    // into a token's code, and the app is updated apart from it.
    @Test
    fun theAddressAnOlderInstallationWritesIsReadToo() {
        assertEquals(
            Connection("https://time.example.com", "gtr_abc123"),
            connectionIn("go-time-recording://connect?token=gtr_abc123&url=https%3A%2F%2Ftime.example.com"),
        )
    }

    @Test
    fun theOrderOfTheTwoIsNotReliedOn() {
        assertEquals(
            Connection("http://localhost:8000", "timepulse_abc123"),
            connectionIn("timepulse://connect?url=http%3A%2F%2Flocalhost%3A8000%2F&token=timepulse_abc123"),
        )
    }

    // A code is a link any page can open, so anything but exactly this shape -
    // and an address the app would not ask with http or https - is nothing.
    @Test
    fun anythingElseIsNoConnection() {
        listOf(
            "https://time.example.com/?token=timepulse_abc123&url=https%3A%2F%2Ftime.example.com",
            "timepulse://elsewhere?token=timepulse_abc123&url=https%3A%2F%2Ftime.example.com",
            "timepulse://connect?url=https%3A%2F%2Ftime.example.com",
            "timepulse://connect?token=timepulse_abc123",
            "timepulse://connect?token=&url=https%3A%2F%2Ftime.example.com",
            "timepulse://connect?token=timepulse_abc123&url=ftp%3A%2F%2Ftime.example.com",
            "timepulse://connect?token=timepulse_abc123&url=javascript%3Aalert(1)",
            "timepulse://connect?token=timepulse_abc123&url=https%3A%2F%2F",
            "not a link at all",
        ).forEach { assertNull(it, connectionIn(it)) }
    }
}
