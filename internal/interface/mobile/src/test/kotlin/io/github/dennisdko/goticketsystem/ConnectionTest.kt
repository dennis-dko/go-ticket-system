package io.github.dennisdko.goticketsystem

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ConnectionTest {

    // The address a token's code holds, written the way the server writes it:
    // url.Values encodes the two query-encoded and in the order of their names.
    @Test
    fun theAddressATokensCodeHoldsIsRead() {
        assertEquals(
            Connection("https://tickets.example.com", "gts_abc123"),
            connectionIn("go-ticket-system://connect?token=gts_abc123&url=https%3A%2F%2Ftickets.example.com"),
        )
    }

    @Test
    fun theOrderOfTheTwoIsNotReliedOn() {
        assertEquals(
            Connection("http://localhost:8000", "gts_abc123"),
            connectionIn("go-ticket-system://connect?url=http%3A%2F%2Flocalhost%3A8000%2F&token=gts_abc123"),
        )
    }

    // A code is a link any page can open, so anything but exactly this shape -
    // and an address the app would not ask with http or https - is nothing.
    @Test
    fun anythingElseIsNoConnection() {
        listOf(
            "https://tickets.example.com/?token=gts_abc123&url=https%3A%2F%2Ftickets.example.com",
            "go-ticket-system://elsewhere?token=gts_abc123&url=https%3A%2F%2Ftickets.example.com",
            "go-ticket-system://connect?url=https%3A%2F%2Ftickets.example.com",
            "go-ticket-system://connect?token=gts_abc123",
            "go-ticket-system://connect?token=&url=https%3A%2F%2Ftickets.example.com",
            "go-ticket-system://connect?token=gts_abc123&url=ftp%3A%2F%2Ftickets.example.com",
            "go-ticket-system://connect?token=gts_abc123&url=javascript%3Aalert(1)",
            "go-ticket-system://connect?token=gts_abc123&url=https%3A%2F%2F",
            "not a link at all",
        ).forEach { assertNull(it, connectionIn(it)) }
    }
}
