package io.github.dennisdko.goticketsystem

import com.google.zxing.BarcodeFormat
import com.google.zxing.qrcode.QRCodeWriter
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ScannerTest {

    private val link = "go-ticket-system://connect?url=https%3A%2F%2Ftickets.example.com&token=gts_abc123"

    /**
     * frame is [text] as a QR code in a camera's first plane: 300 pixels square,
     * each row followed by [padding] bytes that are not the picture's, dark on
     * light unless [inverted].
     */
    private fun frame(text: String, padding: Int, inverted: Boolean = false): Pair<ByteArray, Int> {
        val side = 300
        val stride = side + padding
        val code = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, side, side)
        val bytes = ByteArray(stride * side) { 0x55 }

        for (y in 0 until side) {
            for (x in 0 until side) {
                val dark = code[x, y] != inverted
                bytes[y * stride + x] = if (dark) 0x10 else 0xF0.toByte()
            }
        }

        return bytes to stride
    }

    // A camera's plane is wider than its picture more often than not, and
    // reading it as though it were not shears the code past reading.
    @Test
    fun aTokensCodeIsReadFromAFrameWhoseRowsArePadded() {
        val (bytes, stride) = frame(link, padding = 64)

        val read = codeIn(bytes, 300, 300, stride)

        assertEquals(link, read)
        assertEquals(Connection("https://tickets.example.com", "gts_abc123"), connectionIn(read!!))
    }

    @Test
    fun aCodeDrawnLightOnDarkIsReadToo() {
        val (bytes, stride) = frame(link, padding = 0, inverted = true)

        assertEquals(link, codeIn(bytes, 300, 300, stride))
    }

    @Test
    fun aFrameWithoutACodeHoldsNone() {
        assertNull(codeIn(ByteArray(300 * 300) { 0x7F }, 300, 300, 300))
    }

    // The scanner reads every code it sees, and only a token's becomes a connection.
    @Test
    fun aCodeThatIsNotATokensIsNoConnection() {
        val (bytes, stride) = frame("https://example.com/menu", padding = 16)

        val read = codeIn(bytes, 300, 300, stride)

        assertEquals("https://example.com/menu", read)
        assertNull(connectionIn(read!!))
    }
}
