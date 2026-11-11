package io.github.dennisdko.gotimerecording

import android.app.Application
import android.content.res.AssetManager
import android.os.SystemClock
import android.webkit.CookieManager
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.util.Locale

/**
 * Dictionaries are the application's dictionaries as the build copied them into
 * the app, one file per language, named by its code.
 */
class Dictionaries(private val assets: AssetManager) {

    /** spoken is the languages there are words for, English aside: English has no file. */
    val spoken: Set<String> = assets.list("").orEmpty()
        .filter { it.endsWith(".json") }
        .map { it.removeSuffix(".json") }
        .toSet()

    /** words is a language's words, or English for one there are none for. */
    fun words(language: String): Language {
        if (language !in spoken) return Language("en", emptyMap())

        val read = JSONObject(assets.open("$language.json").bufferedReader(Charsets.UTF_8).use { it.readText() })

        return Language(language, read.keys().asSequence().associateWith { read.getString(it) })
    }
}

/**
 * State is what the app shows: the connect form until it is connected, and the
 * installation's web interface once it has a session to show it with.
 */
data class State(
    val words: Language,
    val connection: Connection?,
    /** offered is a connection a scanned code asks for, waiting for whoever holds the phone to accept it. */
    val offered: Connection? = null,
    /** ready says the web interface has a session to be shown with. */
    val ready: Boolean = false,
    /** shown counts the sessions the web interface was given; the page is loaded afresh each time it changes. */
    val shown: Int = 0,
    /** busy says what the app is waiting for, while it is. */
    val busy: String? = null,
    /** said is why the last attempt failed. */
    val said: String? = null,
)

/**
 * AppModel holds the connection across a turned screen and keeps the web
 * interface in a session: the token's, opened when the app is connected and
 * again whenever the one the page holds has ended - so that the phone stays
 * signed in for as long as the token is good, without a password typed on it.
 */
class AppModel(application: Application) : AndroidViewModel(application) {

    private val store = ConnectionStore(application)
    private val dictionaries = Dictionaries(application.assets)

    /** words are the phone's language, which is all there is to speak in before an installation is reached. */
    private val words = dictionaries.words(Locale.getDefault().language)

    /** checkedAt is when the web interface's session was last found or opened. */
    private var checkedAt = 0L

    var state by mutableStateOf(State(words = words, connection = store.load()))
        private set

    init {
        state.connection?.let {
            open(it, keep = false)
            startReminders(application)
        }
    }

    /** offer shows the connect form filled with what a scanned code holds; nothing is changed until somebody connects. */
    fun offer(connection: Connection) {
        state = state.copy(offered = connection, said = null)
    }

    /** decline keeps the connection the app has and forgets the one offered. */
    fun decline() {
        state = state.copy(offered = null, said = null)
    }

    /** connect asks the installation at address for a session with token, and keeps the two once it has one. */
    fun connect(address: String, token: String) {
        val wanted = Connection(address.trim().trimEnd('/'), token.trim())

        if (!speaksHttp(wanted.address)) {
            state = state.copy(said = words.text("mobile.connect.badAddress", "write the address with http:// or https://, as the browser shows it"))
            return
        }

        open(wanted, keep = true)
    }

    /** retry tries the connection the app has again, after the installation could not be reached. */
    fun retry() {
        state.connection?.let { open(it, keep = false) }
    }

    /**
     * resume makes sure the web interface still has a session when the app comes
     * back, at most once in a while: a session ends after its idle time, and the
     * page would otherwise greet its owner with the sign-in form.
     */
    fun resume() {
        val connection = state.connection ?: return
        if (state.busy != null || SystemClock.elapsedRealtime() - checkedAt < checkEvery) return

        open(connection, keep = false)
    }

    /**
     * disconnect forgets the installation, its token and the web interface's
     * session on this phone, and ends the session - when somebody signs out in the
     * web interface, or asks to connect anew.
     */
    fun disconnect() {
        state = forgotten()
    }

    /**
     * sessionEnded is the page saying its session ended without its being signed
     * out of: the session's idle time ran out, or the token it was opened with was
     * taken back. The token says which - a new session for one still good, which
     * the page is loaded afresh with, and the connect form, saying why, for one
     * the installation no longer knows.
     */
    fun sessionEnded() {
        val connection = state.connection ?: return
        if (state.busy != null) return

        open(connection, keep = false)
    }

    /**
     * forgotten forgets the installation, its token and the web interface's
     * session on this phone, ends the session, and is the state with nothing
     * connected.
     */
    private fun forgotten(): State {
        val connection = state.connection
        val cookies = CookieManager.getInstance()
        val held = connection?.let { cookies.getCookie(it.address) }.orEmpty()

        store.forget()
        stopReminders(getApplication())
        cookies.removeAllCookies(null)

        connection?.let { signOut(it, held) }

        return State(words = words, connection = null)
    }

    /**
     * signOut ends a session the app holds no longer, with its cookies as they
     * were taken before they were forgotten. Nothing waits for it: the phone has
     * forgotten the session either way.
     */
    private fun signOut(connection: Connection, cookies: String) {
        if (cookies.isBlank()) return

        viewModelScope.launch { runCatching { Installation(connection.address, words).signOut(cookies) } }
    }

    /**
     * open gives the web interface a session: the one the page holds where it
     * still is one, and a new one for the token otherwise. A connection is kept
     * only once the installation has answered as its owner's.
     */
    private fun open(connection: Connection, keep: Boolean) {
        state = state.copy(busy = words.text("mobile.reading", "Reading {0} …", connection.address), said = null)

        viewModelScope.launch {
            val installation = Installation(connection.address, words)
            val cookies = CookieManager.getInstance()

            // A connection made anew replaces the one the app held, whose session
            // nobody will use again; its cookies are taken before the new ones can
            // overwrite them.
            val replaced = state.connection.takeIf { keep }
            val held = replaced?.let { cookies.getCookie(it.address) }.orEmpty()

            state = try {
                if (!keep && installation.signedIn(cookies.getCookie(connection.address).orEmpty())) {
                    state.copy(ready = true)
                } else {
                    installation.handOver(connection.token).forEach { cookies.setCookie(connection.address, it) }
                    cookies.flush()

                    if (keep) {
                        store.save(connection)
                        startReminders(getApplication())
                    }
                    replaced?.let { signOut(it, held) }

                    state.copy(connection = connection, offered = null, ready = true, shown = state.shown + 1)
                }
            } catch (refused: TokenRefused) {
                // The app's own token taken back - under API tokens, in the app or
                // anywhere else - or run out: the phone is disconnected, as a lost
                // one is meant to be, and says why. A token being connected with
                // anew that the installation does not know leaves the connection
                // there was as it was.
                if (keep) state.copy(said = refused.message) else forgotten().copy(said = refused.message)
            } catch (refused: Refused) {
                state.copy(said = refused.message)
            }

            checkedAt = SystemClock.elapsedRealtime()
            state = state.copy(busy = null)
        }
    }

    private companion object {
        /** checkEvery is how long the app trusts the session it found before asking again, in milliseconds. */
        const val checkEvery = 5 * 60_000L
    }
}
