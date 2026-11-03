package io.github.dennisdko.gotimerecording

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import org.json.JSONObject
import java.time.LocalDate
import java.time.ZoneId

/**
 * SettingsScreen is the account's own settings, each with what it is set to now
 * and saved to the account on its own - as My account in the browser offers them,
 * on the browser's conditions, and to the routes the browser saves them to, so
 * that the app can set nothing the browser cannot. Password, second factor and
 * tokens stay in the browser: they manage the account rather than preset it.
 */
@Composable
fun SettingsScreen(state: State, model: TrackModel, onClose: () -> Unit) {
    val words = state.words
    val snapshot = state.snapshot ?: return
    val account = snapshot.account
    val busy = state.busy != null

    Page {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = onClose) { Text(words.text("mobile.settings.back", "Back")) }
            Text(words.text("track.menu.settings", "Settings"), style = MaterialTheme.typography.headlineSmall)
        }

        Waiting(state)

        LanguageSetting(words, model.languages, snapshot.language.ifEmpty { words.code }, busy) { chosen ->
            model.save("/me/language", JSONObject().put("language", chosen), language = chosen)
        }
        ThemeSetting(words, account.theme, busy) { chosen -> model.save("/me/theme", JSONObject().put("theme", chosen)) }
        ZoneSetting(words, account, busy) { chosen -> model.save("/me/timezone", JSONObject().put("timezone", chosen)) }

        snapshot.places?.takeIf { it.countries.isNotEmpty() }?.let { places ->
            HolidaySetting(words, places, busy) { chosen -> model.save("/me/holidays", JSONObject().put("place", chosen)) }
        }

        ReminderSetting(words, account, busy) { unbooked, hours ->
            model.save("/me/reminders", JSONObject().put("unbookedDay", unbooked).put("timerAlertHours", hours))
        }

        // The bounds only where the installation answers the balance they bound.
        snapshot.notices.balance?.let { balance ->
            BoundSetting(words, balance, busy) { upper, lower ->
                model.save("/me/balance-limits", JSONObject().put("upperBound", upper ?: JSONObject.NULL).put("lowerBound", lower ?: JSONObject.NULL))
            }
        }

        if (account.mayKeepWorkingHours) {
            WorkingHourSetting(words, account, busy) { target, most, countsFrom ->
                // Empty is the installation's own, which the route reads as zero, and an
                // empty day takes it off again - as the browser sends both.
                model.save(
                    "/users/" + account.id + "/working-times",
                    JSONObject().put("dailyTargetHours", target ?: 0.0).put("maxDailyHours", most ?: 0.0).put("everyDayCountsFrom", countsFrom),
                )
            }
        }
    }
}

/** Setting is one setting's card: its name, its controls, and a button that saves it. */
@Composable
private fun Setting(title: String, words: Language, canSave: Boolean, onSave: () -> Unit, content: @Composable () -> Unit) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            content()
            Button(onClick = onSave, enabled = canSave) { Text(words.text("action.save", "Save")) }
        }
    }
}

/** Chooser offers a choice of named values, as the app's project chooser does. */
@Composable
private fun Chooser(label: String, options: List<Pair<String, String>>, chosen: String, onChoose: (String) -> Unit) {
    var open by remember { mutableStateOf(false) }

    Box {
        OutlinedButton(onClick = { open = true }, modifier = Modifier.fillMaxWidth()) {
            Text(label + ": " + (options.firstOrNull { it.first == chosen }?.second ?: chosen))
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            options.forEach { (value, name) ->
                DropdownMenuItem(text = { Text(name) }, onClick = { onChoose(value); open = false })
            }
        }
    }
}

@Composable
private fun LanguageSetting(words: Language, languages: List<Pair<String, String>>, now: String, busy: Boolean, onSave: (String) -> Unit) {
    var chosen by rememberSaveable(now) { mutableStateOf(now) }

    Setting(words.text("app.language", "Language"), words, !busy && chosen != now, { onSave(chosen) }) {
        Chooser(words.text("app.language", "Language"), languages, chosen) { chosen = it }
    }
}

@Composable
private fun ThemeSetting(words: Language, now: String, busy: Boolean, onSave: (String) -> Unit) {
    var chosen by rememberSaveable(now) { mutableStateOf(now) }
    val options = listOf(
        "" to words.text("mobile.theme.auto", "Like the phone"),
        "light" to words.text("theme.light", "Light"),
        "dark" to words.text("theme.dark", "Dark"),
    )

    Setting(words.text("app.theme", "Appearance"), words, !busy && chosen != now, { onSave(chosen) }) {
        Chooser(words.text("app.theme", "Appearance"), options, chosen) { chosen = it }
    }
}

@Composable
private fun ZoneSetting(words: Language, account: Account, busy: Boolean, onSave: (String) -> Unit) {
    var zone by rememberSaveable(account.timezone) { mutableStateOf(account.timezone) }
    val written = zone.trim()
    val known = written.isEmpty() || runCatching { ZoneId.of(written) }.isSuccess

    Setting(words.text("tz.myTitle", "My timezone"), words, !busy && known && written != account.timezone, { onSave(written) }) {
        OutlinedTextField(
            value = zone,
            onValueChange = { zone = it },
            placeholder = { Text(account.installationZone) },
            singleLine = true,
            isError = !known,
            supportingText = {
                Text(
                    if (known) {
                        words.text(
                            "track.settings.zoneNote",
                            "Written as the tz database writes it, such as Europe/Berlin. Empty follows the installation ({0}).",
                            account.installationZone,
                        )
                    } else {
                        words.text("track.settings.unknownZone", "{0} is not a time zone", written)
                    },
                )
            },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun HolidaySetting(words: Language, places: HolidayPlaces, busy: Boolean, onSave: (String) -> Unit) {
    var country by rememberSaveable(places.place) { mutableStateOf(places.place.substringBefore('-')) }
    var region by rememberSaveable(places.place) { mutableStateOf(places.place.substringAfter('-', "")) }

    val offered = places.country(country)
    val chosen = places.chosen(country, region)
    val answered = offered?.regionRequired != true || region in offered.regions

    val countries = listOf(
        "" to words.text("holiday.inherit", "Follow the installation ({0})", words.placeName(places.installation)),
        "none" to words.text("holiday.noRegion", "No public holidays"),
    ) + places.countries.map { it.code to words.countryName(it.code) }.sortedBy { it.second }

    Setting(words.text("holiday.myTitle", "My public holidays"), words, !busy && answered && chosen != places.place, { onSave(chosen) }) {
        Chooser(words.text("holiday.country", "Country"), countries, country) { country = it; region = "" }

        if (offered != null && offered.regions.isNotEmpty()) {
            val first = if (offered.regionRequired) {
                words.text("holiday.chooseRegion", "Choose one")
            } else {
                words.text("holiday.elsewhere", "Everywhere else")
            }
            val regions = listOf("" to first) + offered.regions.map { it to words.regionName(offered.code + "-" + it) }

            Chooser(words.regionsCalled(offered.code), regions, region) { region = it }
        }
    }
}

@Composable
private fun ReminderSetting(words: Language, account: Account, busy: Boolean, onSave: (Boolean, Int) -> Unit) {
    var unbooked by rememberSaveable(account.unbookedDay) { mutableStateOf(account.unbookedDay) }
    var hours by rememberSaveable(account.alertHours) { mutableStateOf(account.alertHours.toString()) }
    val read = hours.trim().toIntOrNull()?.takeIf { it in 0..24 }

    Setting(words.text("remind.title", "Reminders"), words, !busy && read != null, { onSave(unbooked, read ?: 0) }) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Switch(checked = unbooked, onCheckedChange = { unbooked = it })
            Text(words.text("remind.unbookedDay", "Mention the last working day when nothing is booked on it"))
        }
        OutlinedTextField(
            value = hours,
            onValueChange = { hours = it },
            label = { Text(words.text("field.timerAlertHours", "Mention the stopwatch after (hours)")) },
            singleLine = true,
            isError = read == null,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
            supportingText = {
                Text(
                    if (read != null) {
                        words.text(
                            "remind.hint",
                            "Zero means the stopwatch is never mentioned. Past a day, it says by itself that it holds more than one entry can.",
                        )
                    } else {
                        words.text("track.settings.alertRange", "write a whole number from 0 to {0}", "24")
                    },
                )
            },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun BoundSetting(words: Language, balance: Balance, busy: Boolean, onSave: (Double?, Double?) -> Unit) {
    var upper by rememberSaveable(balance.upper) { mutableStateOf(balance.upper?.let { boundField(it) }.orEmpty()) }
    var lower by rememberSaveable(balance.lower) { mutableStateOf(balance.lower?.let { boundField(it) }.orEmpty()) }
    val readUpper = figure(upper)
    val readLower = figure(lower)

    Setting(
        words.text("bounds.title", "Bounds of my balance"), words, !busy && readUpper.isSuccess && readLower.isSuccess,
        { onSave(readUpper.getOrNull(), readLower.getOrNull()) },
    ) {
        Figure(words, words.text("field.upperBound", "Upper bound of the balance"), upper, readUpper.isSuccess) { upper = it }
        Figure(words, words.text("field.lowerBound", "Lower bound of the balance"), lower, readLower.isSuccess) { lower = it }
    }
}

@Composable
private fun WorkingHourSetting(words: Language, account: Account, busy: Boolean, onSave: (Double?, Double?, String) -> Unit) {
    var target by rememberSaveable(account.targetHours) { mutableStateOf(figureField(account.targetHours)) }
    var most by rememberSaveable(account.maxHours) { mutableStateOf(figureField(account.maxHours)) }
    var countsFrom by rememberSaveable(account.countsFrom) { mutableStateOf(account.countsFrom) }
    val readTarget = figure(target)
    val readMost = figure(most)
    val day = countsFrom.trim()
    val readDay = day.isEmpty() || runCatching { LocalDate.parse(day) }.isSuccess

    Setting(
        words.text("settings.workingTimes", "My working hours"), words, !busy && readTarget.isSuccess && readMost.isSuccess && readDay,
        { onSave(readTarget.getOrNull(), readMost.getOrNull(), day) },
    ) {
        Figure(words, words.text("settings.targetHours", "Target hours per day"), target, readTarget.isSuccess) { target = it }
        Figure(words, words.text("settings.maxHours", "Maximum hours per day"), most, readMost.isSuccess) { most = it }
        OutlinedTextField(
            value = countsFrom,
            onValueChange = { countsFrom = it },
            label = { Text(words.text("settings.everyDayCountsFrom", "Every working day counts from")) },
            placeholder = { Text(words.text("track.form.dayHint", "YYYY-MM-DD")) },
            singleLine = true,
            isError = !readDay,
            supportingText = {
                Text(
                    if (readDay) {
                        words.text(
                            "settings.everyDayCountsFromHint",
                            "Until you choose a day, only the days you booked on count towards your balance. From the day you choose, every working day up to today counts, booked or not, less what a public holiday or an absence takes off it. Empty it to go back.",
                        )
                    } else {
                        words.text("track.notDay", "\"{0}\" is not a day: write it as YYYY-MM-DD", day)
                    },
                )
            },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

/** Figure is a field for a number of hours, which may be left empty. */
@Composable
private fun Figure(words: Language, label: String, value: String, readable: Boolean, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        placeholder = { Text(words.text("track.form.optional", "optional")) },
        singleLine = true,
        isError = !readable,
        supportingText = if (readable) null else {
            { Text(words.text("track.settings.notHours", "{0} is not a number of hours", value.trim())) }
        },
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
        modifier = Modifier.fillMaxWidth(),
    )
}
