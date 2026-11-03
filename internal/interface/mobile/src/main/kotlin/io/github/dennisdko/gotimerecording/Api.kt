package io.github.dennisdko.gotimerecording

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONException
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.MalformedURLException
import java.net.URL

/**
 * Api asks one installation's API as the owner of one personal token and nothing
 * else, as the terminal client does: the app can do exactly what the token's
 * owner may, and is held to every rule the API keeps.
 */
class Api(address: String, private val token: String, private val words: Language) {

    private val base = address.trimEnd('/')

    /**
     * call sends a request and answers the data of the answer - a JSONObject, a
     * JSONArray or null - and throws [Refused] with a sentence when there is none.
     */
    suspend fun call(method: String, path: String, body: JSONObject? = null): Any? = withContext(Dispatchers.IO) {
        var url = try {
            URL("$base/api/v1$path")
        } catch (e: MalformedURLException) {
            throw Refused(words.text("track.badAddress", "\"{0}\" is not an address this can ask", base))
        }

        // A redirect is followed only to the same host, which is what an
        // installation that serves HTTPS does with a plain address: the token
        // goes with every request, and nowhere the address did not name.
        repeat(maxRedirects + 1) {
            val answer = send(method, url, body)
            val location = answer.location

            if (answer.status !in 300..399 || location == null) return@withContext dataOf(answer)

            val next = runCatching { URL(url, location) }.getOrNull()
            if (next == null || !next.host.equals(url.host, ignoreCase = true)) return@withContext dataOf(answer)

            url = next
        }

        throw Refused(words.text("track.unreachable", "could not reach {0}", base))
    }

    /** Answer is what came back for one request. */
    private class Answer(val status: Int, val reason: String, val location: String?, val body: String)

    private fun send(method: String, url: URL, body: JSONObject?): Answer {
        val connection = try {
            url.openConnection() as HttpURLConnection
        } catch (e: IOException) {
            throw Refused(words.text("track.unreachable", "could not reach {0}", base) + ": " + e.message)
        }

        try {
            connection.instanceFollowRedirects = false
            connection.requestMethod = method
            connection.connectTimeout = requestTimeout
            connection.readTimeout = requestTimeout
            connection.setRequestProperty("Authorization", "Bearer $token")
            connection.setRequestProperty("Accept", "application/json")

            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json")
                connection.outputStream.use { it.write(body.toString().toByteArray()) }
            }

            val status = connection.responseCode
            val stream = if (status >= 400) connection.errorStream else connection.inputStream

            return Answer(
                status = status,
                reason = connection.responseMessage.orEmpty(),
                location = connection.getHeaderField("Location"),
                body = stream?.use { read(it) }.orEmpty(),
            )
        } catch (e: IOException) {
            throw Refused(words.text("track.unreachable", "could not reach {0}", base) + ": " + e.message)
        } finally {
            connection.disconnect()
        }
    }

    private fun dataOf(answer: Answer): Any? {
        if (answer.status >= 300) throw Refused(words.refused(answer.status, answer.reason, answer.body))

        val notThis = Refused(words.text("track.notThisApplication", "{0} answered with something that is not this application's", base))

        val envelope = try {
            JSONObject(answer.body)
        } catch (e: JSONException) {
            throw notThis
        }

        if (!envelope.has("data")) throw notThis

        return envelope.get("data").takeUnless { it == JSONObject.NULL }
    }

    /** read reads an answer, no more of it than [maxAnswer]: anything near that is not one of this application's. */
    private fun read(stream: InputStream): String {
        val bytes = ByteArrayOutputStream()
        val buffer = ByteArray(16 shl 10)

        while (true) {
            val count = stream.read(buffer)
            if (count < 0) break

            bytes.write(buffer, 0, count)
            if (bytes.size() > maxAnswer) {
                throw Refused(words.text("track.notThisApplication", "{0} answered with something that is not this application's", base))
            }
        }

        return bytes.toString(Charsets.UTF_8.name())
    }

    private companion object {
        /** requestTimeout bounds one request: a phone waiting on an installation that has gone away should say so. */
        const val requestTimeout = 30_000

        /** maxAnswer bounds how much of an answer is read; the largest this asks for is a day's entries. */
        const val maxAnswer = 8 shl 20

        const val maxRedirects = 5
    }
}
