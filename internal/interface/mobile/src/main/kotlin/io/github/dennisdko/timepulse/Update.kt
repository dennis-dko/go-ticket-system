package io.github.dennisdko.timepulse

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.os.Build
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.net.HttpURLConnection
import java.net.URL

/**
 * Release is a version as a release is named - v0.4.23 - and nothing else. A
 * build from a source tree says how far past its last tag it is, and "dev" is a
 * build that is no release's; neither has a release to compare.
 */
data class Release(val major: Int, val minor: Int, val patch: Int) : Comparable<Release> {
    override fun compareTo(other: Release): Int = compareValuesBy(this, other, Release::major, Release::minor, Release::patch)
}

/** releaseOf reads [version] as the release it names, or null where it names none. */
fun releaseOf(version: String): Release? {
    val parts = Regex("""^v(\d+)\.(\d+)\.(\d+)$""").find(version)?.groupValues?.drop(1) ?: return null
    val (major, minor, patch) = parts.map { it.toIntOrNull() ?: return null }

    return Release(major, minor, patch)
}

/** Offer is the app of the installation's release, newer than this one: which release it is, and where it is downloaded. */
data class Offer(val version: String, val download: String)

/**
 * offerFor is the app the installation offers this one, installed as
 * [installed], read from its answer to /api/v1/me/android: the app of its own
 * release, where that is newer.
 *
 * The installation's rather than the newest on GitHub, because the app belongs to
 * the release it was built with, and one ahead of its installation could ask for
 * what that installation does not have yet. Nothing where either is no release:
 * an installation built from source has no app of its own, and an app built from
 * source is signed with a key a release's app cannot replace.
 */
fun offerFor(installed: String, answer: JSONObject?): Offer? {
    val mine = releaseOf(installed) ?: return null
    val version = answer?.optString("version").orEmpty()
    val download = answer?.optString("download").orEmpty()
    val theirs = releaseOf(version) ?: return null

    if (theirs <= mine || download.isBlank()) return null

    return Offer(version, download)
}

/**
 * Update is the newer app on offer and how far it has got: [loading] is the share
 * downloaded, while it is being, and [said] why the last attempt did not install it.
 */
data class Update(val offer: Offer, val loading: Float? = null, val said: String? = null)

/** Outcome is what Android said about an installation the app handed it, where that was not a question to whoever holds the phone. */
data class Outcome(val status: Int, val message: String?)

/** outcomes carries each [Outcome] from [UpdateReceiver], which hears it, to the offer on screen. */
val outcomes = MutableSharedFlow<Outcome>(extraBufferCapacity = 4)

/**
 * Updater downloads the app an [Offer] names and hands it to Android to install
 * over this one.
 *
 * Straight into an installation session rather than into a file first, so there
 * is no file left behind and none another app could be handed. Android installs
 * it only where it is signed with the key this app was - every release is signed
 * with one key - and only where it is newer, so the worst an answer naming some
 * other file can do is be refused.
 */
class Updater(private val context: Context, private val timeout: Int = Installation.requestTimeout) {

    /** install downloads [offer], telling [progress] the share it has, and commits it; Android answers [UpdateReceiver]. */
    suspend fun install(offer: Offer, progress: (Float) -> Unit) = withContext(Dispatchers.IO) {
        val installer = context.packageManager.packageInstaller
        val connection = URL(offer.download).openConnection() as HttpURLConnection
        connection.connectTimeout = timeout
        connection.readTimeout = timeout

        var session: Int? = null

        try {
            val status = connection.responseCode
            if (status != HttpURLConnection.HTTP_OK) throw IOException("$status ${connection.responseMessage.orEmpty()}".trim())

            val size = connection.contentLengthLong
            val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL)
            params.setAppPackageName(context.packageName)
            if (size > 0) params.setSize(size)

            val id = installer.createSession(params)
            session = id

            installer.openSession(id).use { open ->
                open.openWrite(apkName, 0, size).use { out ->
                    connection.inputStream.use { transfer(it, out, size, progress) }
                    open.fsync(out)
                }

                open.commit(answerTo(context, id).intentSender)
            }

            // Committed, it is Android's: abandoning it now would take it back.
            session = null
        } finally {
            session?.let { runCatching { installer.abandonSession(it) } }
            connection.disconnect()
        }
    }
}

/** transfer writes [input] to [output], telling [progress] the share of [size] written each hundredth, where the size is known. */
fun transfer(input: InputStream, output: OutputStream, size: Long, progress: (Float) -> Unit) {
    val buffer = ByteArray(64 * 1024)
    var written = 0L
    var told = -1

    while (true) {
        val read = input.read(buffer)
        if (read < 0) break

        output.write(buffer, 0, read)
        written += read

        if (size <= 0) continue

        val hundredths = (written * 100 / size).toInt().coerceAtMost(100)
        if (hundredths != told) {
            told = hundredths
            progress(hundredths / 100f)
        }
    }
}

/** apkName is what the app is called inside its installation session, where nothing else sees it. */
private const val apkName = "timepulse.apk"

/**
 * answerTo is where Android says how the installation of [session] went: the
 * app's own receiver, which no other app can reach. The answer can carry a
 * question for whoever holds the phone, which is a screen the app is to start,
 * and a component other apps could send to would start whatever screen they put
 * there.
 */
private fun answerTo(context: Context, session: Int): PendingIntent {
    // Android fills the answer in, so it has to be mutable; it names the receiver
    // itself, which is what lets it be.
    val mutable = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) PendingIntent.FLAG_MUTABLE else 0

    return PendingIntent.getBroadcast(
        context,
        session,
        Intent(context, UpdateReceiver::class.java),
        mutable or PendingIntent.FLAG_UPDATE_CURRENT,
    )
}

/**
 * UpdateReceiver hears how an installation the app handed Android went. Where
 * Android asks whoever holds the phone to confirm it - as it does for every app
 * not installed from a store - its question is shown; anything else goes to the
 * offer on screen.
 */
class UpdateReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)

        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            question(intent)?.let { context.startActivity(it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
            return
        }

        outcomes.tryEmit(Outcome(status, intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE)))
    }

    /** question is the screen Android asks on whether the installation may go ahead. */
    private fun question(intent: Intent): Intent? =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            intent.getParcelableExtra(Intent.EXTRA_INTENT, Intent::class.java)
        } else {
            @Suppress("DEPRECATION")
            intent.getParcelableExtra(Intent.EXTRA_INTENT)
        }
}
