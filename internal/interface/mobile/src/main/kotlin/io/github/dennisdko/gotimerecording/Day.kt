package io.github.dennisdko.gotimerecording

import org.json.JSONArray
import org.json.JSONObject
import java.net.URLEncoder
import java.time.Instant
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.ZoneOffset
import java.util.Locale

/** Project is one of the token owner's projects, as /projects lists it. */
data class Project(val id: Long, val name: String, val status: String)

/** Entry is a time entry, as the API answers with one. */
data class Entry(val projectId: Long?, val date: String, val durationHours: Double, val description: String?)

/** Stopwatch is the running clock, as /me/timer answers: the one the browser shows, kept on the server. */
data class Stopwatch(val running: Boolean, val projectId: Long?, val startedAt: Instant?, val elapsedHours: Double)

/**
 * Snapshot is the token owner's day as it stands: who they are, where their days
 * are kept, their projects, the stopwatch, what today has booked - and which
 * release the installation runs, which decides what this app can ask it. With it
 * comes what the owner set for themselves, what the installation calls itself and
 * says, the notices, and where public holidays are kept: read with the day, so
 * that a choice made in the browser reaches the phone at its next read.
 */
data class Snapshot(
    val name: String,
    val zone: ZoneId,
    val language: String,
    val projects: List<Project>,
    val clock: Stopwatch,
    val today: LocalDate,
    val entries: List<Entry>,
    val version: String,
    val account: Account = Account(),
    val branding: Branding = Branding(),
    val notices: Notices = Notices(),
    /** places is null where the installation names no countries, which one from before there were countries does not. */
    val places: HolidayPlaces? = null,
)

/**
 * Day reads the token owner's day and makes the changes the terminal client
 * makes - start, stop, switch and book - through the same routes. Each change
 * answers with what it did in the terminal client's sentence, so a stop booked
 * from the phone reads the same as one booked from the shell.
 */
class Day(private val api: Api, private val words: Language) {

    /** snapshot reads the token owner's day. */
    suspend fun snapshot(): Snapshot {
        val answer = api.call("GET", "/me") as JSONObject
        val me = answer.getJSONObject("user")
        val zone = runCatching { ZoneId.of(me.stringAt("effectiveTimezone")) }.getOrDefault(ZoneOffset.UTC)
        val today = LocalDate.now(zone)
        val branding = api.call("GET", "/branding") as JSONObject

        return Snapshot(
            name = me.stringAt("name"),
            zone = zone,
            language = me.stringAt("language"),
            projects = projects(),
            clock = stopwatchOf(api.call("GET", "/me/timer") as JSONObject),
            today = today,
            entries = entriesOn(today),
            version = branding.stringAt("version"),
            account = Account.of(answer),
            branding = Branding.of(branding),
            notices = optional { Notices.of(api.call("GET", "/me/notices") as JSONObject) } ?: Notices(),
            places = optional { HolidayPlaces.of(api.call("GET", "/me/holidays") as JSONObject) },
        )
    }

    /**
     * save sends one of the account's settings to the route the browser sends it
     * to - so the app can set nothing the browser cannot - and says where it went,
     * in [said]: the language just chosen, where the app is about to speak it.
     */
    suspend fun save(route: String, body: JSONObject, said: Language): String {
        api.call("PUT", route, body)

        return said.text("track.settings.saved", "Saved for your account - the browser and the app follow it too.")
    }

    /** optional is what read answers, or null where the installation refuses it: one from before it was asked has no such route. */
    private suspend fun <T> optional(read: suspend () -> T): T? = try {
        read()
    } catch (refused: Refused) {
        null
    }

    /** start starts the stopwatch on project - none for null - and says when, in the owner's zone. */
    suspend fun start(project: Project?, description: String, zone: ZoneId): String {
        val clock = stopwatchOf(api.call("POST", "/me/timer", aimedAt(project, description)) as JSONObject)
        val started = (clock.startedAt ?: Instant.now()).atZone(zone)

        return words.text("track.started", "Started at {0} on {1}.", words.time(started), nameOf(project))
    }

    /** stop stops the stopwatch and books what it measured. */
    suspend fun stop(projects: List<Project>): String {
        val booked = entryOf(api.call("POST", "/me/timer/stop") as JSONObject)

        return words.text("track.stopped", "Booked {0} on {1}.", words.hours(booked.durationHours), words.nameOf(projects, booked.projectId))
    }

    /** switchTo books what the stopwatch measured and carries on on project. */
    suspend fun switchTo(project: Project?, description: String, projects: List<Project>): String {
        val switched = api.call("POST", "/me/timer/switch", aimedAt(project, description)) as JSONObject
        val booked = entryOf(switched.getJSONObject("booked"))

        return words.text(
            "track.switched", "Booked {0} on {1}; running on {2} now.",
            words.hours(booked.durationHours), words.nameOf(projects, booked.projectId), nameOf(project),
        )
    }

    /** book books hours by hand on project, on day. */
    suspend fun book(hours: Double, project: Project?, description: String, day: LocalDate): String {
        val body = aimedAt(project, description)
            .put("date", day.toString())
            .put("durationHours", hours)

        val booked = entryOf(api.call("POST", "/timesheets", body) as JSONObject)
        val on = runCatching { LocalDate.parse(booked.date) }.getOrDefault(day)

        return words.text("track.booked", "Booked {0} on {1} for {2}.", words.hours(booked.durationHours), nameOf(project), words.day(on))
    }

    private suspend fun projects(): List<Project> {
        val items = (api.call("GET", "/projects") as JSONObject).optJSONArray("items") ?: JSONArray()

        return (0 until items.length()).map { i ->
            val p = items.getJSONObject(i)
            Project(p.getLong("id"), p.stringAt("name"), p.stringAt("status"))
        }
    }

    /** entriesOn reads the entries of one day - a page holds far more than a day's. */
    private suspend fun entriesOn(day: LocalDate): List<Entry> {
        val query = listOf("from" to day.toString(), "to" to day.toString(), "limit" to "500")
            .joinToString("&") { (name, value) -> name + "=" + URLEncoder.encode(value, "UTF-8") }
        val items = (api.call("GET", "/timesheets?$query") as JSONObject).optJSONArray("items") ?: JSONArray()

        return (0 until items.length()).map { entryOf(items.getJSONObject(it)) }
    }

    private fun aimedAt(project: Project?, description: String): JSONObject {
        val body = JSONObject()
        if (project != null) body.put("projectId", project.id)
        if (description.isNotBlank()) body.put("description", description.trim())

        return body
    }

    private fun nameOf(project: Project?): String = project?.name ?: words.text("track.noProject", "no project")
}

/** nameOf is what a project is called, by its id; null is no project. */
fun Language.nameOf(projects: List<Project>, id: Long?): String {
    if (id == null) return text("track.noProject", "no project")

    return projects.firstOrNull { it.id == id }?.name ?: text("track.projectNumber", "project {0}", id.toString())
}

/** clockLine says what the stopwatch is doing, given how long it has run. */
fun Language.clockLine(snapshot: Snapshot, elapsedHours: Double): String {
    val clock = snapshot.clock
    val started = clock.startedAt
    if (!clock.running || started == null) return text("track.notRunning", "The stopwatch is not running.")

    return text(
        "track.running", "Running since {0} on {1} - {2} so far.",
        time(started.atZone(snapshot.zone)), nameOf(snapshot.projects, clock.projectId), hoursAndMinutes(elapsedHours),
    )
}

/** parseHours reads hours the ways people write them: 1.5, 1,5 or 1:30. */
fun Language.parseHours(raw: String): Double {
    val written = raw.trim()
    val refused = Refused(text("track.notHours", "\"{0}\" is not a number of hours: write 1.5, 1,5 or 1:30", written))

    if (':' in written) {
        val h = written.substringBefore(':').toIntOrNull()
        val m = written.substringAfter(':').toIntOrNull()

        if (h == null || m == null || h < 0 || m < 0 || m >= 60 || h + m == 0) throw refused

        return h + m / 60.0
    }

    val worked = written.replaceFirst(',', '.').toDoubleOrNull()
    if (worked == null || worked.isNaN() || worked.isInfinite() || worked <= 0) throw refused

    return worked
}

/** hoursAndMinutes writes a running clock's time as a clock would: 1:23. */
fun hoursAndMinutes(h: Double): String {
    val minutes = Math.round(h * 60)

    return String.format(Locale.ROOT, "%d:%02d", minutes / 60, minutes % 60)
}

/** total adds up what entries booked. */
fun total(entries: List<Entry>): Double = entries.sumOf { it.durationHours }

private fun stopwatchOf(clock: JSONObject): Stopwatch = Stopwatch(
    running = clock.optBoolean("running"),
    projectId = longOrNull(clock, "projectId"),
    startedAt = clock.stringAt("startedAt").takeIf { it.isNotEmpty() }?.let { runCatching { OffsetDateTime.parse(it).toInstant() }.getOrNull() },
    elapsedHours = clock.optDouble("elapsedHours", 0.0),
)

private fun entryOf(entry: JSONObject): Entry = Entry(
    projectId = longOrNull(entry, "projectId"),
    date = entry.stringAt("date"),
    durationHours = entry.optDouble("durationHours", 0.0),
    description = entry.stringAt("description").takeIf { it.isNotEmpty() },
)

private fun longOrNull(json: JSONObject, name: String): Long? = if (json.isNull(name)) null else json.optLong(name)

/** stringAt is a field's text, empty where it is absent or null - which optString writes as the word null. */
private fun JSONObject.stringAt(name: String): String = if (isNull(name)) "" else optString(name)
