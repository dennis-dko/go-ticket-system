package io.github.dennisdko.goticketsystem

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels

/** MainActivity is the app's one screen - the installation's web interface, once connected - and where the link a token's code holds arrives. */
class MainActivity : ComponentActivity() {

    private val model: AppModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        // A turned screen is created again with the intent it was opened with,
        // and offering that code a second time would undo a "Cancel".
        if (savedInstanceState == null) offer(intent)

        setContent { App(model) }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        offer(intent)
    }

    // Whenever the app comes back, the web interface's session is made sure of:
    // it ends after its idle time, and the page would otherwise open on the
    // sign-in form.
    override fun onResume() {
        super.onResume()
        model.resume()
    }

    // Leaving the app is when a stopwatch started in it needs its alarm: the
    // quarter-hourly check would find it too, but a phone that is put away dozes
    // and holds that check back.
    override fun onStop() {
        super.onStop()
        askNow(this)
    }

    private fun offer(intent: Intent?) {
        val link = intent?.dataString ?: return

        connectionIn(link)?.let(model::offer)
    }
}
