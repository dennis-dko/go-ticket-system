package io.github.dennisdko.gotimerecording

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URI
import java.net.URL

/**
 * Installation is what the app asks the installation itself, around the web
 * interface it shows: a session for the token it was connected with, and
 * whether the session the web interface holds is still one.
 *
 * Everything else is the web interface's own business. The app shows the
 * installation's pages rather than a copy of them, decided by the maintainer on
 * 2026-11-03, so that it can do whatever the browser can, in the same design, and
 * goes on doing so with every release without being built a second time.
 */
class Installation(private val address: String, private val words: Language) {

    /**
     * handOver opens a session for the owner of [token] and answers its cookies,
     * as Set-Cookie headers, for the web interface to be shown with. The token
     * grants nothing the session does not: it already carries its owner's role.
     */
    suspend fun handOver(token: String): List<String> = withContext(Dispatchers.IO) {
        val connection = open("/api/v1/auth/token")

        try {
            connection.requestMethod = "POST"
            connection.setRequestProperty("Authorization", "Bearer $token")
            connection.setRequestProperty("Accept", "application/json")

            val status = connection.responseCode
            if (status == HttpURLConnection.HTTP_UNAUTHORIZED) {
                throw Refused(
                    words.text(
                        "mobile.connect.tokenUnknown",
                        "The installation does not know this token. Make one under My account, API tokens, and scan its code.",
                    ),
                )
            }

            if (status !in 200..299) {
                val body = (connection.errorStream ?: connection.inputStream)?.use { it.readBytes().toString(Charsets.UTF_8) }.orEmpty()
                throw Refused(words.refused(status, connection.responseMessage.orEmpty(), body))
            }

            val cookies = connection.headerFields.entries
                .filter { (name, _) -> name.equals("Set-Cookie", ignoreCase = true) }
                .flatMap { it.value }

            if (cookies.none { it.startsWith("gtr_session=") }) throw notThis()

            cookies
        } catch (e: IOException) {
            throw unreachable(e)
        } finally {
            connection.disconnect()
        }
    }

    /** signedIn says whether [cookies] - the web interface's, as the WebView keeps them - still open a session. */
    suspend fun signedIn(cookies: String): Boolean = withContext(Dispatchers.IO) {
        if (cookies.isBlank()) return@withContext false

        val connection = open("/api/v1/me")

        try {
            connection.setRequestProperty("Cookie", cookies)
            connection.setRequestProperty("Accept", "application/json")

            connection.responseCode == HttpURLConnection.HTTP_OK
        } catch (e: IOException) {
            throw unreachable(e)
        } finally {
            connection.disconnect()
        }
    }

    /**
     * signOut ends the session [cookies] hold, as the web interface's own sign-out
     * does. The app does it itself rather than leave it to the page: when the page
     * signs out, the app forgets the cookies and the page with them while the
     * page's request may still be on its way, and either can lose it.
     */
    suspend fun signOut(cookies: String) {
        // The installation asks every change a session makes for this token in a
        // header beside the cookie, and for where the request comes from.
        val csrf = cookieValue(cookies, "gtr_csrf") ?: return

        withContext(Dispatchers.IO) {
            val connection = open("/api/v1/auth/logout")

            try {
                connection.requestMethod = "POST"
                connection.setRequestProperty("Cookie", cookies)
                connection.setRequestProperty("X-CSRF-Token", csrf)
                connection.setRequestProperty("Referer", address.trimEnd('/') + "/")
                connection.responseCode
            } catch (_: IOException) {
                // Out of reach, the session lasts until it runs out; the phone has
                // forgotten it either way.
            } finally {
                connection.disconnect()
            }
        }
    }

    private fun open(path: String): HttpURLConnection {
        val url = try {
            URL(address.trimEnd('/') + path)
        } catch (e: java.net.MalformedURLException) {
            throw Refused(words.text("mobile.notAnAddress", "\"{0}\" is not an address this can ask", address))
        }

        return (url.openConnection() as HttpURLConnection).apply {
            instanceFollowRedirects = false
            connectTimeout = requestTimeout
            readTimeout = requestTimeout
        }
    }

    private fun unreachable(e: IOException): Refused =
        Refused(words.text("mobile.unreachable", "could not reach {0}", address) + ": " + e.message)

    private fun notThis(): Refused =
        Refused(words.text("mobile.notThisApplication", "{0} answered with something that is not this application's", address))

    private companion object {
        /** requestTimeout bounds one request: a phone waiting on an installation that has gone away should say so. */
        const val requestTimeout = 30_000
    }
}

/**
 * belongsTo says whether a page is the installation's own, which the app shows
 * itself: the same scheme, host and port as its address. Anything else - a link
 * to the source code, a sign-in provider's page - opens in the phone's browser,
 * where its owner's rules for it apply rather than the app's.
 */
fun belongsTo(address: String, url: String): Boolean {
    val home = runCatching { URI(address) }.getOrNull() ?: return false
    val page = runCatching { URI(url) }.getOrNull() ?: return false

    return home.scheme.equals(page.scheme, ignoreCase = true) &&
        home.host.equals(page.host, ignoreCase = true) &&
        portOf(home) == portOf(page)
}

/** cookieValue is one cookie's value out of a Cookie header, as the WebView's cookie store hands them over. */
fun cookieValue(cookies: String, name: String): String? =
    cookies.split(';').map { it.trim() }.firstOrNull { it.startsWith("$name=") }?.substringAfter('=')

private fun portOf(uri: URI): Int = when {
    uri.port >= 0 -> uri.port
    uri.scheme.equals("https", ignoreCase = true) -> 443
    else -> 80
}
