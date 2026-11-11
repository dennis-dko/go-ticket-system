package io.github.dennisdko.gotimerecording

import android.Manifest
import android.annotation.SuppressLint
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationChannelCompat
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import java.time.Instant
import java.time.ZoneId
import java.util.Locale
import java.util.concurrent.TimeUnit

/**
 * ReminderWorker shows the installation's reminders as the phone's own
 * notifications, whether the app is open or not: it asks the installation every
 * quarter of an hour, which is how the app learns of a stopwatch started in a
 * browser.
 *
 * That is not enough on its own, because a dozing phone holds a worker back until
 * it wakes - measured on an emulator in deep idle, where the check set for the
 * stopwatch's moment waited on the network and ran the moment the phone woke. So
 * every check also sets a [ReminderAlarm] for the moment the next reminder falls
 * due, which the phone lets through while it dozes, and the app is asked about as
 * soon as somebody leaves it, so a stopwatch started in it has its alarm at once.
 *
 * Nothing is shown that the installation has not just confirmed. A stopwatch
 * stopped in the browser in the meantime is asked about again rather than
 * remembered, so the phone does not say it is still running. Without a network,
 * or without the phone's leave to notify, it does nothing and asks again next
 * time. No push service is involved - the app asks, with its token - which keeps
 * it free of Google Play services, as the scanner is.
 */
class ReminderWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        checkReminders(applicationContext)

        return Result.success()
    }
}

/**
 * checkReminders asks the installation what is due and shows it, takes back what
 * is no longer true, and sets the alarm for the next moment a reminder falls due.
 * It says whether the installation answered, so that a check the alarm could not
 * make is left to a worker; one with nothing to ask - no connection, or no leave
 * to notify - counts as answered. [timeout] bounds each request.
 */
suspend fun checkReminders(context: Context, timeout: Int = Installation.requestTimeout): Boolean {
    if (!mayNotify(context)) {
        armReminderAlarm(context, null)
        return true
    }

    val connection = ConnectionStore(context).load() ?: return true
    val words = Dictionaries(context.assets).words(Locale.getDefault().language)

    // Unreachable, or answering with something this cannot read: there is
    // nothing to say until the next time it is asked.
    val reminders = try {
        Installation(connection.address, words, timeout).reminders(connection.token)
    } catch (_: Refused) {
        return false
    } catch (_: RuntimeException) {
        return false
    }

    val kept = context.getSharedPreferences(keptName, Context.MODE_PRIVATE)
    val told = stillTold(reminders, kept.getStringSet(toldKey, emptySet()).orEmpty())
    val now = Instant.now()
    val zone = ZoneId.systemDefault()
    val due = noticesDue(reminders, now, zone, told, words)

    // What is no longer true is taken back as well: a stopwatch stopped in
    // the browser, or a day booked since, leaves no notice behind on the phone.
    val manager = NotificationManagerCompat.from(context)
    if (stopwatchDue(reminders)?.let { !now.isBefore(it) } != true) manager.cancel(stopwatchNotification)
    if (reminders.unbookedDay == null) manager.cancel(unbookedNotification)

    due.forEach { show(context, words, it) }
    kept.edit { putStringSet(toldKey, told + due.map { it.key }) }

    armReminderAlarm(context, nextCheck(reminders, now, zone))

    return true
}

/** startReminders starts asking for a connected app, keeping the schedule where there is one. */
fun startReminders(context: Context) {
    val request = PeriodicWorkRequestBuilder<ReminderWorker>(15, TimeUnit.MINUTES).setConstraints(online()).build()

    WorkManager.getInstance(context).enqueueUniquePeriodicWork(everyQuarter, ExistingPeriodicWorkPolicy.KEEP, request)
}

/** stopReminders stops asking and forgets what was shown, when the app is disconnected. */
fun stopReminders(context: Context) {
    WorkManager.getInstance(context).apply {
        cancelUniqueWork(everyQuarter)
        cancelUniqueWork(rightAway)
    }

    armReminderAlarm(context, null)
    context.getSharedPreferences(keptName, Context.MODE_PRIVATE).edit { remove(toldKey) }
    NotificationManagerCompat.from(context).cancelAll()
}

/**
 * askNow asks the installation as soon as there is a network: when somebody
 * leaves the app, when the phone has just allowed notifications, and when an
 * alarm could not reach the installation in its moment.
 */
fun askNow(context: Context) {
    val request = OneTimeWorkRequestBuilder<ReminderWorker>().setConstraints(online()).build()

    WorkManager.getInstance(context).enqueueUniqueWork(rightAway, ExistingWorkPolicy.REPLACE, request)
}

/** mayNotify says whether the phone lets the app show notifications at all: switched on, and from Android 13 on granted. */
fun mayNotify(context: Context): Boolean =
    NotificationManagerCompat.from(context).areNotificationsEnabled() &&
        (Build.VERSION.SDK_INT < 33 || ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED)

/**
 * notificationState is what the page's Reminders card is told, in the words the
 * browser's Notification.permission uses: granted, default where the phone has
 * not been asked yet, and denied otherwise - a phone that was asked and said no
 * asks its owner no more than once again, so the card says where to change it.
 */
fun notificationState(context: Context): String = when {
    mayNotify(context) -> "granted"
    Build.VERSION.SDK_INT >= 33 && !context.getSharedPreferences(keptName, Context.MODE_PRIVATE).getBoolean(askedKey, false) -> "default"
    else -> "denied"
}

/**
 * askedForNotifications records that the phone was asked, whatever it answered,
 * and asks the installation at once where it allowed them: a stopwatch already
 * past its hours is said now rather than at the next quarter of an hour.
 */
fun askedForNotifications(context: Context) {
    context.getSharedPreferences(keptName, Context.MODE_PRIVATE).edit { putBoolean(askedKey, true) }

    if (mayNotify(context)) askNow(context)
}

// The leave to notify is asked for once more just before, because it can be
// taken back while the installation is asked, and that is the check lint cannot
// follow into mayNotify.
@SuppressLint("MissingPermission")
private fun show(context: Context, words: Language, notice: Notice) {
    val manager = NotificationManagerCompat.from(context)

    manager.createNotificationChannel(
        NotificationChannelCompat.Builder(channel, NotificationManagerCompat.IMPORTANCE_DEFAULT)
            .setName(words.text("remind.title", "Reminders"))
            .build(),
    )

    val open = PendingIntent.getActivity(
        context,
        0,
        Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    val notification = NotificationCompat.Builder(context, channel)
        .setSmallIcon(R.drawable.ic_notification)
        .setContentTitle(notice.title)
        .setContentText(notice.body)
        .setStyle(NotificationCompat.BigTextStyle().bigText(notice.body))
        .setContentIntent(open)
        .setAutoCancel(true)
        .build()

    if (!mayNotify(context)) return

    // One notification of each kind, so a newer one takes the older's place.
    manager.notify(if (notice.key.startsWith("stopwatch:")) stopwatchNotification else unbookedNotification, notification)
}

private fun online(): Constraints = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

/** everyQuarter names the check every quarter of an hour, and rightAway the one asked for at once. */
private const val everyQuarter = "reminders"
private const val rightAway = "reminders-now"

/** keptName is where the app keeps what it has shown and whether it asked the phone. */
private const val keptName = "reminders"
private const val toldKey = "told"
private const val askedKey = "asked"

private const val channel = "reminders"
private const val stopwatchNotification = 1
private const val unbookedNotification = 2
