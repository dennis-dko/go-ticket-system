package io.github.dennisdko.gotimerecording

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels

/** MainActivity is the app's one screen, and where the link a token's code holds arrives. */
class MainActivity : ComponentActivity() {

    private val model: TrackModel by viewModels()

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

    // The day is read whenever the app comes back: the stopwatch may have been
    // stopped in the browser meanwhile, and stopping it anywhere stops it
    // everywhere.
    override fun onResume() {
        super.onResume()
        model.refresh()
    }

    private fun offer(intent: Intent?) {
        val link = intent?.dataString ?: return

        connectionIn(link)?.let(model::offer)
    }
}
