package io.github.dennisdko.gotimerecording

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp

/**
 * App is the whole of what the app shows: the connect form until it is
 * connected, the installation's web interface once it has a session for it, and
 * in between what it is waiting for or why it could not.
 */
@Composable
fun App(model: AppModel) {
    // The mark's blue, and a lighter one where the background is dark, as the
    // web interface's accent does - for the few screens before it takes over.
    val colors = if (isSystemInDarkTheme()) darkColorScheme(primary = Color(0xFF8AB4FF)) else lightColorScheme(primary = Color(0xFF2F6FEB))

    MaterialTheme(colorScheme = colors) {
        Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
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

@Composable
private fun ConnectScreen(state: State, onConnect: (String, String) -> Unit, onCancel: (() -> Unit)?) {
    val words = state.words
    var address by rememberSaveable(state.offered) { mutableStateOf(state.offered?.address ?: state.connection?.address.orEmpty()) }
    var token by rememberSaveable(state.offered) { mutableStateOf(state.offered?.token.orEmpty()) }

    Page {
        Text(words.text("mobile.connect.title", "Connect to your installation"), style = MaterialTheme.typography.headlineSmall)
        Text(
            words.text(
                "mobile.connect.text",
                "Make a token under My account, API tokens, then scan its code with the phone's camera - or enter the address and the token here.",
            ),
        )

        // A code is a link any page can open, so what one asks for is shown and
        // waits for whoever holds the phone, rather than being done for them.
        state.offered?.let {
            Text(
                words.text(
                    "mobile.connect.offered",
                    "A code asks to connect this app to {0}. Connect only to an installation you know: it is told everything you book here.",
                    it.address,
                ),
                color = MaterialTheme.colorScheme.error,
            )
        }

        OutlinedTextField(
            value = address,
            onValueChange = { address = it },
            label = { Text(words.text("mobile.connect.address", "Address")) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = token,
            onValueChange = { token = it },
            label = { Text(words.text("mobile.connect.token", "Token")) },
            singleLine = true,
            visualTransformation = PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
            modifier = Modifier.fillMaxWidth(),
        )

        Waiting(state)

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = { onConnect(address, token) }, enabled = state.busy == null && address.isNotBlank() && token.isNotBlank()) {
                Text(words.text("mobile.connect.submit", "Connect"))
            }
            if (onCancel != null) {
                OutlinedButton(onClick = onCancel) { Text(words.text("action.cancel", "Cancel")) }
            }
        }
    }
}

/** WaitScreen is the time it takes to reach the installation, and why it could not be, with the two ways on from there. */
@Composable
private fun WaitScreen(state: State, onRetry: () -> Unit, onDisconnect: () -> Unit) {
    val words = state.words

    Page {
        Text(state.connection?.address.orEmpty(), style = MaterialTheme.typography.titleLarge)

        Waiting(state)

        if (state.busy == null) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onRetry) { Text(words.text("mobile.connect.retry", "Try again")) }
                OutlinedButton(onClick = onDisconnect) { Text(words.text("mobile.disconnect", "Disconnect")) }
            }
        }
    }
}

/** Page is a screen's column: scrolled, and clear of the system's bars and the keyboard. */
@Composable
private fun Page(content: @Composable () -> Unit) {
    Column(
        Modifier.fillMaxSize().safeDrawingPadding().verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) { content() }
}

/** Waiting says what the app is waiting for, and why the last attempt failed. */
@Composable
private fun Waiting(state: State) {
    state.busy?.let {
        LinearProgressIndicator(Modifier.fillMaxWidth())
        Text(it, style = MaterialTheme.typography.bodySmall)
    }
    state.said?.let {
        Text(it, color = MaterialTheme.colorScheme.error)
    }
}
