package io.github.dennisdko.gotimerecording

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * App is the whole of what the app shows: the connect form until it is
 * connected, the installation's web interface once it has a session for it, and
 * in between what it is waiting for or why it could not.
 */
@Composable
fun App(model: AppModel) {
    val dark = isSystemInDarkTheme()
    val tones = if (dark) Tones.dark else Tones.light

    // Material's own pieces - a field's cursor, a progress bar - in the web
    // interface's colours too.
    val colors = (if (dark) darkColorScheme() else lightColorScheme()).copy(
        primary = tones.accent,
        onPrimary = tones.accentText,
        background = tones.bg,
        onBackground = tones.text,
        surface = tones.surface,
        onSurface = tones.text,
        error = tones.danger,
        outline = tones.border,
    )

    // The page's text: 15 pixels on a line and a half, and no spacing between the
    // letters, which Material adds to its own.
    val type = Typography().let {
        it.copy(
            bodyLarge = it.bodyLarge.copy(fontSize = 15.sp, lineHeight = 22.sp, letterSpacing = 0.sp),
            labelLarge = it.labelLarge.copy(letterSpacing = 0.sp),
        )
    }

    CompositionLocalProvider(LocalTones provides tones) {
        MaterialTheme(colorScheme = colors, typography = type) {
            Surface(Modifier.fillMaxSize(), color = tones.bg) {
                val state = model.state
                val connection = state.connection

                when {
                    connection == null || state.offered != null ->
                        ConnectScreen(state, model::connect, if (connection != null) model::decline else null)
                    state.ready -> WebScreen(connection.address, state.words, state.shown, model::disconnect)
                    else -> WaitScreen(state, model::retry, model::disconnect)
                }
            }
        }
    }
}

@Composable
private fun ConnectScreen(state: State, onConnect: (String, String) -> Unit, onCancel: (() -> Unit)?) {
    val words = state.words
    val tones = LocalTones.current

    var address by rememberSaveable(state.offered) { mutableStateOf(state.offered?.address ?: state.connection?.address.orEmpty()) }
    var token by rememberSaveable(state.offered) { mutableStateOf(state.offered?.token.orEmpty()) }

    val filled = address.isNotBlank() && token.isNotBlank()

    Page {
        Card(words.text("mobile.connect.title", "Connect to your installation")) {
            Text(
                words.text(
                    "mobile.connect.text",
                    "Make a token under My account, API tokens, then scan its code with the phone's camera - or enter the address and the token here.",
                ),
                color = tones.muted,
                fontSize = 15.sp,
            )

            // A code is a link any page can open, so what one asks for is shown and
            // waits for whoever holds the phone, rather than being done for them.
            state.offered?.let {
                Warning(
                    words.text(
                        "mobile.connect.offered",
                        "A code asks to connect this app to {0}. Connect only to an installation you know: it is told everything you book here.",
                        it.address,
                    ),
                )
            }

            Field(words.text("mobile.connect.address", "Address"), address, { address = it }, KeyboardType.Uri)
            Field(words.text("mobile.connect.token", "Token"), token, { token = it }, KeyboardType.Password, secret = true)

            Waiting(state)

            Action(
                words.text("mobile.connect.submit", "Connect"),
                primary = true,
                enabled = state.busy == null && filled,
                onClick = { onConnect(address, token) },
            )

            if (onCancel != null) {
                Action(words.text("action.cancel", "Cancel"), primary = false, onClick = onCancel)
            }
        }
    }
}

/** WaitScreen is the time it takes to reach the installation, and why it could not be, with the two ways on from there. */
@Composable
private fun WaitScreen(state: State, onRetry: () -> Unit, onDisconnect: () -> Unit) {
    val words = state.words

    Page {
        Card(state.connection?.address.orEmpty()) {
            Waiting(state)

            if (state.busy == null) {
                Action(words.text("mobile.connect.retry", "Try again"), primary = true, onClick = onRetry)
                Action(words.text("mobile.disconnect", "Disconnect"), primary = false, onClick = onDisconnect)
            }
        }
    }
}

/**
 * Page is a screen's column, clear of the system's bars and the keyboard, with
 * the card in its middle as the web interface's sign-in has it - and scrolled
 * once the card is taller than the screen.
 */
@Composable
private fun Page(content: @Composable ColumnScope.() -> Unit) {
    BoxWithConstraints(Modifier.fillMaxSize().safeDrawingPadding()) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).heightIn(min = maxHeight).padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
            content = content,
        )
    }
}

/** Waiting says what the app is waiting for, and why the last attempt failed. */
@Composable
private fun Waiting(state: State) {
    val tones = LocalTones.current

    state.busy?.let {
        LinearProgressIndicator(Modifier.fillMaxWidth(), color = tones.accent, trackColor = tones.border)
        Text(it, color = tones.muted, fontSize = 13.sp)
    }
    state.said?.let {
        Text(it, color = tones.danger, fontSize = 14.sp)
    }
}
