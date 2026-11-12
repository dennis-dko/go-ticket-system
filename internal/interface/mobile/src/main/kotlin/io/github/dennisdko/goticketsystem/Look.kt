package io.github.dennisdko.goticketsystem

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Tones are the web interface's colours, light and dark - app.css holds the same
 * ones, and a test in internal/interface/mobile holds the two together - so that
 * the screens before the web interface takes over look like its own sign-in.
 */
data class Tones(
    val bg: Color,
    val surface: Color,
    val border: Color,
    val text: Color,
    val muted: Color,
    val accent: Color,
    val accentText: Color,
    val danger: Color,
) {
    companion object {
        val light = Tones(bg = Color(0xFFF5F6F8), surface = Color(0xFFFFFFFF), border = Color(0xFFDFE3E8), text = Color(0xFF1C2126), muted = Color(0xFF626D78), accent = Color(0xFF2F6FEB), accentText = Color(0xFFFFFFFF), danger = Color(0xFFC0392B))
        val dark = Tones(bg = Color(0xFF14171A), surface = Color(0xFF1C2126), border = Color(0xFF2C333B), text = Color(0xFFE8EAED), muted = Color(0xFF98A2AD), accent = Color(0xFF5B8DFA), accentText = Color(0xFF0D1117), danger = Color(0xFFE06C5C))

        /** band is the sign-in card's band, the same in light and dark. */
        val band = Color(0xFF2B3440)
    }
}

/** LocalTones is the tones the screen is drawn in. */
val LocalTones = staticCompositionLocalOf { Tones.light }

/** mixed is share of a with the rest of b, channel by channel, as app.css's color-mix in srgb mixes them. */
fun mixed(a: Color, b: Color, share: Float): Color = Color(
    red = a.red * share + b.red * (1 - share),
    green = a.green * share + b.green * (1 - share),
    blue = a.blue * share + b.blue * (1 - share),
)

/** corners are the web interface's: 8 pixels round on a card, a field and a button. */
private val corners = RoundedCornerShape(8.dp)

/**
 * Card is the web interface's sign-in card: a surface with its border, rounded
 * and lifted off the page, headed by the band with the app's mark and [title].
 */
@Composable
fun Card(title: String, content: @Composable ColumnScope.() -> Unit) {
    val tones = LocalTones.current

    Column(
        Modifier
            .widthIn(max = 560.dp)
            .fillMaxWidth()
            .shadow(16.dp, corners, ambientColor = Color(0x24101828), spotColor = Color(0x24101828))
            .clip(corners)
            .background(tones.surface)
            .border(1.dp, tones.border, corners),
    ) {
        Band(title)
        Column(Modifier.padding(horizontal = 24.dp, vertical = 24.dp), verticalArrangement = Arrangement.spacedBy(14.dp), content = content)
    }
}

/** Band is the card's dark band: the mark on its emblem and a heading, over the accent's line. */
@Composable
private fun Band(title: String) {
    val tones = LocalTones.current

    Column {
        Row(
            Modifier.fillMaxWidth().background(Tones.band).padding(start = 12.dp, top = 12.dp, end = 20.dp, bottom = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(Modifier.clip(corners).background(tones.surface).padding(6.dp)) { Mark(36.dp) }
            Spacer(Modifier.size(16.dp))
            Text(title, color = Color.White, fontSize = 22.sp, lineHeight = 28.sp)
        }
        Spacer(Modifier.fillMaxWidth().height(5.dp).background(tones.accent))
    }
}

/**
 * Mark is the hourglass on its tile, from favicon.svg's own numbers: a tile 26
 * units across with corners of 7.6 in a box of 32, and the two halves meeting in
 * the middle, the fallen sand dimmer.
 */
@Composable
fun Mark(size: Dp) {
    Canvas(Modifier.size(size)) {
        val unit = this.size.width / 32f
        fun at(x: Float, y: Float) = Offset(x * unit, y * unit)

        drawRoundRect(Color(0xFF2F6FEB), topLeft = at(3f, 3f), size = Size(26 * unit, 26 * unit), cornerRadius = CornerRadius(7.6f * unit))
        drawPath(Path().apply { moveTo(at(9.6f, 8.6f)); lineTo(at(22.4f, 8.6f)); lineTo(at(16f, 16f)); close() }, Color.White)
        drawPath(Path().apply { moveTo(at(9.6f, 23.4f)); lineTo(at(22.4f, 23.4f)); lineTo(at(16f, 16f)); close() }, Color.White.copy(alpha = 0.55f))
    }
}

private fun Path.moveTo(point: Offset) = moveTo(point.x, point.y)
private fun Path.lineTo(point: Offset) = lineTo(point.x, point.y)

/** Field is an input as the web interface draws one: its label above in the quiet colour, the box on the page's background. */
@Composable
fun Field(label: String, value: String, onValueChange: (String) -> Unit, keyboard: KeyboardType, secret: Boolean = false) {
    val tones = LocalTones.current

    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(label, color = tones.muted, fontSize = 13.sp)
        OutlinedTextField(
            value = value,
            onValueChange = onValueChange,
            singleLine = true,
            shape = corners,
            visualTransformation = if (secret) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions = KeyboardOptions(keyboardType = keyboard),
            colors = OutlinedTextFieldDefaults.colors(
                focusedContainerColor = tones.bg,
                unfocusedContainerColor = tones.bg,
                focusedBorderColor = tones.accent,
                unfocusedBorderColor = tones.border,
                focusedTextColor = tones.text,
                unfocusedTextColor = tones.text,
                cursorColor = tones.accent,
            ),
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

/**
 * Action is a button as the web interface draws one: [primary] filled with the
 * accent, otherwise on the page's background inside a border - with [icon]
 * before its words where it has one.
 */
@Composable
fun Action(text: String, primary: Boolean, onClick: () -> Unit, enabled: Boolean = true, icon: (@Composable () -> Unit)? = null) {
    val tones = LocalTones.current

    Button(
        onClick = onClick,
        enabled = enabled,
        shape = corners,
        colors = ButtonDefaults.buttonColors(
            containerColor = if (primary) tones.accent else tones.bg,
            contentColor = if (primary) tones.accentText else tones.text,
            disabledContainerColor = mixed(if (primary) tones.accent else tones.bg, tones.surface, 0.45f),
            disabledContentColor = mixed(if (primary) tones.accentText else tones.text, tones.surface, 0.6f),
        ),
        border = if (primary) null else BorderStroke(1.dp, tones.border),
        modifier = Modifier.fillMaxWidth().height(48.dp),
    ) {
        if (icon != null) {
            icon()
            Spacer(Modifier.size(10.dp))
        }
        Text(text, fontWeight = FontWeight.Medium, fontSize = 15.sp)
    }
}

/** Warning is the web interface's warning banner: the danger colour mixed into the surface, inside a border of it. */
@Composable
fun Warning(text: String) {
    val tones = LocalTones.current

    Text(
        text,
        color = tones.text,
        fontSize = 14.sp,
        fontWeight = FontWeight.Medium,
        modifier = Modifier
            .fillMaxWidth()
            .clip(corners)
            .background(mixed(tones.danger, tones.surface, 0.16f))
            .border(1.dp, mixed(tones.danger, tones.border, 0.4f), corners)
            .padding(horizontal = 14.dp, vertical = 10.dp),
    )
}

/** Divided is a quiet line across the card with [words] in its middle, between two ways of doing one thing. */
@Composable
fun Divided(words: String) {
    val tones = LocalTones.current

    Row(verticalAlignment = Alignment.CenterVertically) {
        Spacer(Modifier.weight(1f).height(1.dp).background(tones.border))
        Text(words, color = tones.muted, fontSize = 13.sp, modifier = Modifier.padding(horizontal = 12.dp))
        Spacer(Modifier.weight(1f).height(1.dp).background(tones.border))
    }
}

/** ScanIcon is a code in a viewfinder's four corners, in the colour of the words beside it. */
@Composable
fun ScanIcon(color: Color) {
    Canvas(Modifier.size(20.dp)) {
        val stroke = Stroke(width = 2.dp.toPx(), cap = StrokeCap.Round)
        val w = size.width
        val arm = w * 0.3f

        for ((x, y) in listOf(0f to 0f, w to 0f, 0f to w, w to w)) {
            val dx = if (x == 0f) arm else -arm
            val dy = if (y == 0f) arm else -arm
            drawPath(Path().apply { moveTo(x + dx, y); lineTo(x, y); lineTo(x, y + dy) }, color, style = stroke)
        }

        val inner = w * 0.32f
        drawRect(color, topLeft = Offset(inner, inner), size = Size(w - 2 * inner, w - 2 * inner))
    }
}
