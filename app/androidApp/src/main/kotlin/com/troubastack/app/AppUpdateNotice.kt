// A96 — the app comes back after it updates itself. Android ends the app to replace it and blocks a relaunch from a
// receiver (measured: "Background activity launch blocked … callingUidProcState: RECEIVER" at API 34, "Background
// activity start … isBgStartWhitelisted: false" at 29), but a tap on a notification opens it
// (BAL_ALLOW_PENDING_INTENT). So the new version posts one, for the update the app itself started.
package com.troubastack.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build
import com.troubastack.shared.seams.Storage

/** A96 — Android's verdict on the install session, for the update status (which lives in the app's composition). */
object UpdateSessionEvents {
    /** Set by the update status while the app runs; called on the main thread. */
    var onNotInstalled: (() -> Unit)? = null

    /** The session failed, or the person cancelled in Android's dialog: nothing will be replaced. */
    fun notInstalled(context: Context) {
        ReopenMarker(Storage(context.applicationContext)).clear()
        onNotInstalled?.invoke()
    }
}

/** A96 — the new version's first moment: post "TroubaStage was updated" if this is the update the app started. */
class AppUpdatedReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        @Suppress("DEPRECATION")
        val info = context.packageManager.getPackageInfo(context.packageName, 0)
        @Suppress("DEPRECATION")
        val running = if (Build.VERSION.SDK_INT >= 28) info.longVersionCode else info.versionCode.toLong()
        if (ReopenMarker(Storage(context.applicationContext)).consume(running)) ReopenNotice.post(context)
    }
}

/** A96 — the one notification: "TroubaStage was updated" / "Tap to open.", which opens the app on Home. */
object ReopenNotice {
    private const val CHANNEL = "app-updates"
    private const val ID = 96

    /** Whether a posted notice would show (API 33+: the permission; any level: notifications not switched off). */
    fun canShow(context: Context): Boolean {
        val nm = context.getSystemService(NotificationManager::class.java)
        if (!nm.areNotificationsEnabled()) return false
        return nm.getNotificationChannel(CHANNEL)?.importance != NotificationManager.IMPORTANCE_NONE
    }

    fun post(context: Context) {
        val nm = context.getSystemService(NotificationManager::class.java)
        // HIGH: a heads-up over the launcher the app just vanished into — a status-bar icon alone is easy to miss.
        nm.createNotificationChannel(NotificationChannel(CHANNEL, "App updates", NotificationManager.IMPORTANCE_HIGH))
        val open = context.packageManager.getLaunchIntentForPackage(context.packageName) ?: return
        val tap = PendingIntent.getActivity(context, ID, open, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val n = Notification.Builder(context, CHANNEL)
            .setSmallIcon(R.drawable.ic_stat_updated)
            .setContentTitle("TroubaStage was updated")
            .setContentText("Tap to open.")
            .setContentIntent(tap)
            .setAutoCancel(true)
            .build()
        try { nm.notify(ID, n) } catch (e: SecurityException) { /* not allowed: the status said "Then open TroubaStage again." */ }
    }

    /** The app opened (from the notice or otherwise): the notice has done its job. */
    fun cancel(context: Context) = context.getSystemService(NotificationManager::class.java).cancel(ID)
}
