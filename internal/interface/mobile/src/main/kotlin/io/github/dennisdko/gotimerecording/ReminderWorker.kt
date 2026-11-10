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
import java.time.Duration
import java.time.Instant
import java.time.ZoneId
import java.util.Locale
import java.util.concurrent.TimeUnit

/**
 * ReminderWorker shows the installation's reminders as the phone's own
 * notifications, whether the app is open or not: it asks the installation every
 * quarter of an hour, and again at the moment the running stopwatch passes the
 * hours chosen, which the quarter-hourly check could miss by as much.
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
        val context = applicationContext
        if (!mayNotify(context)) return Result.success()

        val connection = ConnectionStore(context).load() ?: return Result.success()
        val words = Dictionaries(context.assets).words(Locale.getDefault().language)

        // Unreachable, or answering with something this cannot read: there is
        // nothing to say until the next time it is asked.
        val reminders = try {
            Installation(connection.address, words).reminders(connection.token)
        } catch (_: Refused) {
            return Result.success()
        } catch (_: RuntimeException) {
            return Result.success()
        }

        val kept = context.getSharedPreferences(keptName, Context.MODE_PRIVATE)
        val told = stillTold(reminders, kept.getStringSet(toldKey, emptySet()).orEmpty())
        val now = Instant.now()
        val due = noticesDue(reminders, now, ZoneId.systemDefault(), told, words)

        // What is no longer true is taken back as well: a stopwatch stopped in
        // the browser, or a day booked since, leaves no notice behind on the phone.
        val manager = NotificationManagerCompat.from(context)
        if (stopwatchDue(reminders)?.let { !now.isBefore(it) } != true) manager.cancel(stopwatchNotification)
        if (reminders.unbookedDay == null) manager.cancel(unbookedNotification)

        due.forEach { show(context, words, it) }
        kept.edit { putStringSet(toldKey, told + due.map { it.key }) }

        stopwatchDue(reminders)?.takeIf { it.isAfter(now) }?.let { askAt(context, Duration.between(now, it)) }

        return Result.success()
    }
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
        cancelUniqueWork(atTheMoment)
    }

    context.getSharedPreferences(keptName, Context.MODE_PRIVATE).edit { remove(toldKey) }
    NotificationManagerCompat.from(context).cancelAll()
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

    if (mayNotify(context)) askAt(context, Duration.ZERO)
}

private fun askAt(context: Context, delay: Duration) {
    val request = OneTimeWorkRequestBuilder<ReminderWorker>()
        .setInitialDelay(delay.toMillis(), TimeUnit.MILLISECONDS)
        .setConstraints(online())
        .build()

    WorkManager.getInstance(context).enqueueUniqueWork(atTheMoment, ExistingWorkPolicy.REPLACE, request)
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

/** everyQuarter names the check every quarter of an hour, and atTheMoment the one when the stopwatch is due. */
private const val everyQuarter = "reminders"
private const val atTheMoment = "reminders-due"

/** keptName is where the app keeps what it has shown and whether it asked the phone. */
private const val keptName = "reminders"
private const val toldKey = "told"
private const val askedKey = "asked"

private const val channel = "reminders"
private const val stopwatchNotification = 1
private const val unbookedNotification = 2
