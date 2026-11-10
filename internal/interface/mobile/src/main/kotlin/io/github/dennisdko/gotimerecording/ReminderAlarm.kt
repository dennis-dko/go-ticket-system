package io.github.dennisdko.gotimerecording

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import java.time.Instant

/**
 * ReminderAlarm wakes the app at the moment a reminder falls due and asks the
 * installation then, while the phone dozes as well - which a worker cannot do: a
 * dozing phone keeps the network from it and runs it once the phone wakes, so
 * the reminder arrived when somebody next picked the phone up.
 *
 * An exact alarm allowed while idle is let through, and the phone gives the app
 * ten seconds of network with it - measured on Android 15, whose network policy
 * put the app on its temporary allowlist for ALARM_MANAGER_WHILE_IDLE and opened
 * the firewall's dozable chain to it for exactly that long. [alarmTimeout] keeps
 * the check inside those ten seconds. An inexact alarm, which is all Android 14
 * and later lets an app set until Alarms & reminders is allowed for it, fires
 * while the phone dozes but brings no network, so the check it makes cannot reach
 * the installation; that check, like any other that could not, is left to a
 * worker that waits for the network.
 */
class ReminderAlarm : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        val app = context.applicationContext
        val pending = goAsync()

        CoroutineScope(Dispatchers.IO).launch {
            try {
                if (!checkReminders(app, alarmTimeout)) askNow(app)
            } finally {
                pending.finish()
            }
        }
    }
}

/**
 * armReminderAlarm sets the alarm for [at], in place of the one set before, or
 * takes it away where [at] is null.
 *
 * Exact where the phone allows it - without asking up to Android 13, and from
 * Android 14 on once Alarms & reminders is allowed for the app - and otherwise
 * inexact, within the hour after, which is as close as Android lets an app
 * without that leave come.
 */
fun armReminderAlarm(context: Context, at: Instant?) {
    val alarms = context.getSystemService(AlarmManager::class.java) ?: return
    val wake = PendingIntent.getBroadcast(
        context,
        0,
        Intent(context, ReminderAlarm::class.java),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    if (at == null) {
        alarms.cancel(wake)
        return
    }

    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || alarms.canScheduleExactAlarms()) {
        try {
            alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at.toEpochMilli(), wake)
            return
        } catch (_: SecurityException) {
            // The leave was taken back between asking and setting.
        }
    }

    alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at.toEpochMilli(), wake)
}

/** alarmTimeout bounds each of the check's three requests, so that all of them fit the ten seconds of network an alarm brings. */
private const val alarmTimeout = 3_000
