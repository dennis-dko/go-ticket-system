package io.github.dennisdko.gotimerecording

import android.app.Application
import android.content.res.AssetManager
import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.time.LocalDate
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
 * languageFor decides what the app speaks: the language the account chose in the
 * browser where there are words for it, then the phone's, then English - the
 * terminal client's order, which has a flag and GTR_LANG above both.
 */
fun languageFor(account: String, phone: String, spoken: Set<String>): String =
    listOf(account, phone).firstOrNull { it in spoken || it == "en" } ?: "en"

/** State is everything the screen shows. */
data class State(
    val words: Language,
    val connection: Connection?,
    /** offered is a connection a scanned code asks for, waiting for whoever holds the phone to accept it. */
    val offered: Connection? = null,
    val snapshot: Snapshot? = null,
    /** readAt is when the snapshot was read, by the clock a running stopwatch is counted on from. */
    val readAt: Long = 0,
    /** busy says what the app is waiting for, while it is. */
    val busy: String? = null,
    /** said is what the last change did, or why it was refused. */
    val said: String? = null,
    val refused: Boolean = false,
)

/**
 * TrackModel holds the app's state across a turned screen and makes every
 * change through [Day], one at a time.
 */
class TrackModel(application: Application) : AndroidViewModel(application) {

    private val store = ConnectionStore(application)
    private val dictionaries = Dictionaries(application.assets)
    private val phone = Locale.getDefault().language

    var state by mutableStateOf(State(words = wordsFor(""), connection = store.load()))
        private set

    init {
        if (state.connection != null) refresh()
    }

    /** offer shows the connect form filled with what a scanned code holds; nothing is changed until somebody connects. */
    fun offer(connection: Connection) {
        state = state.copy(offered = connection, said = null, refused = false)
    }

    /** decline keeps the connection the app has and forgets the one offered. */
    fun decline() {
        state = state.copy(offered = null, said = null, refused = false)
    }

    /** connect asks the installation at address with token, and keeps the two only once it has answered as their owner. */
    fun connect(address: String, token: String) {
        val wanted = Connection(address.trim().trimEnd('/'), token.trim())

        if (!speaksHttp(wanted.address)) {
            refuse(state.words.text("mobile.connect.badAddress", "write the address with http:// or https://, as the browser shows it"))
            return
        }

        reading(wanted) { day ->
            val snapshot = day.snapshot()
            store.save(wanted)
            state = read(snapshot).copy(connection = wanted, offered = null, said = null, refused = false)
        }
    }

    /** disconnect takes the installation and its token off this phone. */
    fun disconnect() {
        store.forget()
        state = State(words = wordsFor(""), connection = null)
    }

    /** refresh reads the day again - when the app comes back, or somebody asks. */
    fun refresh() {
        val connection = state.connection ?: return
        if (state.busy != null) return

        reading(connection) { day -> state = read(day.snapshot()) }
    }

    fun start(project: Project?, description: String) = change { day, snapshot -> day.start(project, description, snapshot.zone) }

    fun stop() = change { day, snapshot -> day.stop(snapshot.projects) }

    fun switchTo(project: Project?, description: String) = change { day, snapshot -> day.switchTo(project, description, snapshot.projects) }

    /** languages are the languages the app speaks, English first, each by its own name. */
    val languages: List<Pair<String, String>>
        get() = (listOf("en") + dictionaries.spoken.sorted()).map { it to (dictionaries.words(it).lookup("language.name") ?: "English") }

    /**
     * save saves one of the account's settings to the route the browser saves it
     * to, and reads the day again, which brings the setting back as the account
     * now has it - a language from the moment it is chosen. [language] is the one
     * just chosen, where it is one, so the line saying so is in it.
     */
    fun save(route: String, body: JSONObject, language: String? = null) =
        change { day, _ -> day.save(route, body, language?.let { wordsFor(it) } ?: state.words) }

    fun book(hours: String, project: Project?, description: String, on: LocalDate) {
        val worked = try {
            state.words.parseHours(hours)
        } catch (refused: Refused) {
            refuse(refused.message.orEmpty())
            return
        }

        change { day, _ -> day.book(worked, project, description, on) }
    }

    /** change makes one change, says what it did, and reads the day it changed. */
    private fun change(make: suspend (Day, Snapshot) -> String) {
        val connection = state.connection ?: return
        val snapshot = state.snapshot ?: return
        if (state.busy != null) return

        val words = state.words
        state = state.copy(busy = words.text("track.sending", "Sending …"))

        viewModelScope.launch {
            val day = Day(Api(connection.address, connection.token, words), words)

            state = try {
                val said = make(day, snapshot)
                read(day.snapshot()).copy(said = said, refused = false)
            } catch (refused: Refused) {
                state.copy(said = refused.message, refused = true)
            } catch (e: Exception) {
                state.copy(said = notThisApplication(connection), refused = true)
            }

            state = state.copy(busy = null)
        }
    }

    private fun reading(connection: Connection, read: suspend (Day) -> Unit) {
        val words = state.words
        state = state.copy(busy = words.text("track.reading", "Reading {0} …", connection.address))

        viewModelScope.launch {
            try {
                read(Day(Api(connection.address, connection.token, words), words))
            } catch (refused: Refused) {
                state = state.copy(said = refused.message, refused = true)
            } catch (e: Exception) {
                state = state.copy(said = notThisApplication(connection), refused = true)
            }

            state = state.copy(busy = null)
        }
    }

    /** read is the state with a day read, in the language its account chose. */
    private fun read(snapshot: Snapshot): State =
        state.copy(snapshot = snapshot, readAt = SystemClock.elapsedRealtime(), words = wordsFor(snapshot.language))

    private fun refuse(said: String) {
        state = state.copy(said = said, refused = true)
    }

    private fun notThisApplication(connection: Connection): String =
        state.words.text("track.notThisApplication", "{0} answered with something that is not this application's", connection.address)

    private fun wordsFor(account: String): Language = dictionaries.words(languageFor(account, phone, dictionaries.spoken))
}
