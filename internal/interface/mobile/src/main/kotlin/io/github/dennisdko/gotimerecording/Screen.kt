package io.github.dennisdko.gotimerecording

import android.os.SystemClock
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import java.time.LocalDate

/** App is the whole of what the app shows: the connect form until it is connected, and the day after. */
@Composable
fun App(model: TrackModel) {
    // The mark's blue, and a lighter one where the background is dark, as the
    // web interface's accent does.
    val colors = if (isSystemInDarkTheme()) darkColorScheme(primary = Color(0xFF8AB4FF)) else lightColorScheme(primary = Color(0xFF2F6FEB))

    MaterialTheme(colorScheme = colors) {
        Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            val state = model.state

            if (state.connection == null || state.offered != null) {
                ConnectScreen(state, model::connect, if (state.connection != null) model::decline else null)
            } else {
                DayScreen(state, model)
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

@Composable
private fun DayScreen(state: State, model: TrackModel) {
    val words = state.words
    val connection = state.connection ?: return
    val snapshot = state.snapshot
    var form by rememberSaveable { mutableStateOf("") }

    Page {
        Text(snapshot?.name ?: connection.address, style = MaterialTheme.typography.titleLarge)
        Text(connection.address, style = MaterialTheme.typography.bodySmall)

        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = model::refresh, enabled = state.busy == null) { Text(words.text("action.reload", "Reload")) }
            OutlinedButton(onClick = { form = "disconnect" }) { Text(words.text("mobile.disconnect", "Disconnect")) }
        }

        // The app knows the API of the release it was built for, so another
        // release is a state that lasts as long as the two differ - shown for as
        // long as it is true rather than once.
        if (snapshot != null && BuildConfig.VERSION_NAME != "dev" && snapshot.version != BuildConfig.VERSION_NAME) {
            Card(Modifier.fillMaxWidth()) {
                Text(
                    words.text(
                        "mobile.otherVersion",
                        "This app belongs to {0}, the installation runs {1}. Install the app from the installation's own release, so that both know the same API.",
                        BuildConfig.VERSION_NAME, snapshot.version,
                    ),
                    modifier = Modifier.padding(16.dp),
                    color = MaterialTheme.colorScheme.error,
                )
            }
        }

        Waiting(state)

        if (snapshot != null) {
            Clock(state, snapshot, onStart = { form = "start" }, onStop = { form = "stop" }, onSwitch = { form = "switch" })
            Today(words, snapshot, onBook = { form = "book" })
        }
    }

    val close = { form = "" }

    when {
        form == "disconnect" -> Question(
            question = words.text("mobile.disconnectQuestion", "Forget {0} and its token on this phone?", connection.address),
            yes = words.text("mobile.disconnect", "Disconnect"),
            no = words.text("action.cancel", "Cancel"),
            onYes = { close(); model.disconnect() },
            onNo = close,
        )
        snapshot == null -> Unit
        form == "start" -> ProjectForm(
            words, snapshot.projects,
            title = words.text("track.form.start", "Start the stopwatch"),
            note = null,
            confirm = words.text("timer.start", "Start"),
            onDismiss = close,
            onConfirm = { project, description -> close(); model.start(project, description) },
        )
        form == "switch" -> ProjectForm(
            words, snapshot.projects,
            title = words.text("timer.switchTitle", "Carry on with something else"),
            note = words.text("track.form.switchNote", "What has run so far is booked on {0}.", words.nameOf(snapshot.projects, snapshot.clock.projectId)),
            confirm = words.text("timer.switchSave", "Book and carry on"),
            onDismiss = close,
            onConfirm = { project, description -> close(); model.switchTo(project, description) },
        )
        form == "stop" -> Question(
            question = words.text(
                "track.form.stopQuestion", "Book {0} on {1}?",
                hoursAndMinutes(running(state, snapshot)), words.nameOf(snapshot.projects, snapshot.clock.projectId),
            ),
            yes = words.text("track.form.stopYes", "Stop and book"),
            no = words.text("track.form.stopNo", "Keep it running"),
            onYes = { close(); model.stop() },
            onNo = close,
        )
        form == "book" -> BookForm(
            words, snapshot,
            onDismiss = close,
            onConfirm = { hours, project, description, day -> close(); model.book(hours, project, description, day) },
        )
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

/** Waiting says what the app is waiting for, and what the last change did or why it was refused. */
@Composable
private fun Waiting(state: State) {
    state.busy?.let {
        LinearProgressIndicator(Modifier.fillMaxWidth())
        Text(it, style = MaterialTheme.typography.bodySmall)
    }
    state.said?.let {
        Text(it, color = if (state.refused) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onBackground)
    }
}

/** running is how long the stopwatch has run by now: what the installation said, and what has passed since it was asked. */
private fun running(state: State, snapshot: Snapshot, now: Long = SystemClock.elapsedRealtime()): Double =
    snapshot.clock.elapsedHours + (now - state.readAt).coerceAtLeast(0) / 3_600_000.0

@Composable
private fun Clock(state: State, snapshot: Snapshot, onStart: () -> Unit, onStop: () -> Unit, onSwitch: () -> Unit) {
    val words = state.words
    val going = snapshot.clock.running
    var now by remember { mutableLongStateOf(SystemClock.elapsedRealtime()) }

    LaunchedEffect(going, state.readAt) {
        while (going) {
            now = SystemClock.elapsedRealtime()
            delay(1_000)
        }
    }

    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(words.text("timer.title", "Stopwatch"), style = MaterialTheme.typography.titleMedium)
            Text(words.clockLine(snapshot, running(state, snapshot, now)))

            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                if (going) {
                    Button(onClick = onStop, enabled = state.busy == null) { Text(words.text("timer.stop", "Stop and book")) }
                    OutlinedButton(onClick = onSwitch, enabled = state.busy == null) { Text(words.text("timer.switchOpen", "Switch to …")) }
                } else {
                    Button(onClick = onStart, enabled = state.busy == null) { Text(words.text("timer.start", "Start")) }
                }
            }
        }
    }
}

@Composable
private fun Today(words: Language, snapshot: Snapshot, onBook: () -> Unit) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(words.text("cal.today", "Today") + " - " + words.weekday(snapshot.today), style = MaterialTheme.typography.titleMedium)

            if (snapshot.entries.isEmpty()) {
                Text(words.text("track.nothingYet", "Nothing booked yet."))
            } else {
                snapshot.entries.forEach { entry ->
                    val description = entry.description?.let { " - $it" }.orEmpty()
                    Text(words.hours(entry.durationHours) + "  " + words.nameOf(snapshot.projects, entry.projectId) + description)
                }
                Text(words.hours(total(snapshot.entries)) + "  " + words.text("track.inTotal", "in total"), style = MaterialTheme.typography.titleSmall)
            }

            OutlinedButton(onClick = onBook) { Text(words.text("ts.book", "Book time")) }
        }
    }
}

@Composable
private fun Question(question: String, yes: String, no: String, onYes: () -> Unit, onNo: () -> Unit) {
    AlertDialog(
        onDismissRequest = onNo,
        text = { Text(question) },
        confirmButton = { Button(onClick = onYes) { Text(yes) } },
        dismissButton = { TextButton(onClick = onNo) { Text(no) } },
    )
}

@Composable
private fun ProjectForm(
    words: Language,
    projects: List<Project>,
    title: String,
    note: String?,
    confirm: String,
    onDismiss: () -> Unit,
    onConfirm: (Project?, String) -> Unit,
) {
    var chosen by rememberSaveable { mutableStateOf<Long?>(null) }
    var description by rememberSaveable { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                note?.let { Text(it) }
                ProjectChooser(words, projects, chosen) { chosen = it }
                Described(words, description) { description = it }
            }
        },
        confirmButton = { Button(onClick = { onConfirm(projects.firstOrNull { it.id == chosen }, description) }) { Text(confirm) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(words.text("action.cancel", "Cancel")) } },
    )
}

@Composable
private fun BookForm(
    words: Language,
    snapshot: Snapshot,
    onDismiss: () -> Unit,
    onConfirm: (String, Project?, String, LocalDate) -> Unit,
) {
    var hours by rememberSaveable { mutableStateOf("") }
    var chosen by rememberSaveable { mutableStateOf<Long?>(null) }
    var description by rememberSaveable { mutableStateOf("") }
    var day by rememberSaveable { mutableStateOf(snapshot.today.toString()) }
    val parsed = runCatching { LocalDate.parse(day.trim()) }.getOrNull()

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(words.text("track.form.book", "Book time by hand")) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = hours,
                    onValueChange = { hours = it },
                    label = { Text(words.text("field.hours", "Hours")) },
                    placeholder = { Text(words.text("track.form.hoursHint", "1.5, 1,5 or 1:30")) },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Text),
                )
                ProjectChooser(words, snapshot.projects, chosen) { chosen = it }
                Described(words, description) { description = it }
                OutlinedTextField(
                    value = day,
                    onValueChange = { day = it },
                    label = { Text(words.text("field.day", "Day")) },
                    placeholder = { Text(words.text("track.form.dayHint", "YYYY-MM-DD")) },
                    singleLine = true,
                    isError = parsed == null,
                    supportingText = {
                        Text(parsed?.let { words.day(it) } ?: words.text("track.notDay", "\"{0}\" is not a day: write it as YYYY-MM-DD", day.trim()))
                    },
                )
            }
        },
        confirmButton = {
            Button(onClick = { parsed?.let { onConfirm(hours, snapshot.projects.firstOrNull { p -> p.id == chosen }, description, it) } }, enabled = parsed != null && hours.isNotBlank()) {
                Text(words.text("ts.book", "Book time"))
            }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(words.text("action.cancel", "Cancel")) } },
    )
}

/** ProjectChooser offers the active projects, as the terminal client takes them, and no project at all. */
@Composable
private fun ProjectChooser(words: Language, projects: List<Project>, chosen: Long?, onChoose: (Long?) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val none = words.text("ts.noProject", "No project")
    val active = projects.filter { it.status == "active" }

    Box {
        OutlinedButton(onClick = { open = true }, modifier = Modifier.fillMaxWidth()) {
            Text(words.text("field.project", "Project") + ": " + (active.firstOrNull { it.id == chosen }?.name ?: none))
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            DropdownMenuItem(text = { Text(none) }, onClick = { onChoose(null); open = false })
            active.forEach { project ->
                DropdownMenuItem(text = { Text(project.name) }, onClick = { onChoose(project.id); open = false })
            }
        }
    }
}

@Composable
private fun Described(words: Language, description: String, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = description,
        onValueChange = onChange,
        label = { Text(words.text("field.description", "Description") + " (" + words.text("track.form.optional", "optional") + ")") },
    )
}
