package io.github.dennisdko.timepulse

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
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
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
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

/** corners are the web interface's --radius: 10 pixels round on a card and a notice. */
private val corners = RoundedCornerShape(10.dp)

/** pill is the web interface's --pill: a field and a button are as round as they are high. */
private val pill = RoundedCornerShape(percent = 50)

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

/**
 * SignInBand is the web interface's sign-in band, measure for measure: app.css's
 * .login-band, .login-emblem, .login-mark and the band's heading, which a test in
 * internal/interface/mobile holds these to. The maintainer asked for the connect
 * card to draw the mark exactly as the sign-in card does.
 */
private object SignInBand {
    val top = 12.dp
    val end = 28.dp
    val bottom = 12.dp
    val start = 12.dp
    val rowGap = 14.dp
    val gap = 24.dp
    val emblemPadding = 6.dp
    val emblemCorner = 8.dp
    val mark = 96.dp
    val heading = 26.sp
}

/**
 * Band is the card's dark band: the mark on its white field and a heading, over
 * the accent's line.
 *
 * The heading stands beside the mark while ten ems of room are left for it and
 * under it once they are not, as the web's band wraps: on a telephone the mark
 * alone takes most of the width, and the words squeezed beside it would read one
 * to a line.
 */
@Composable
private fun Band(title: String) {
    val tones = LocalTones.current
    val density = LocalDensity.current

    Column {
        BoxWithConstraints(
            Modifier
                .fillMaxWidth()
                .background(Tones.band)
                .padding(start = SignInBand.start, top = SignInBand.top, end = SignInBand.end, bottom = SignInBand.bottom),
        ) {
            val field = SignInBand.mark + SignInBand.emblemPadding * 2
            val beside = maxWidth - field - SignInBand.gap >= with(density) { (SignInBand.heading * 10).toDp() }

            val emblem: @Composable () -> Unit = {
                Box(Modifier.clip(RoundedCornerShape(SignInBand.emblemCorner)).background(tones.surface).padding(SignInBand.emblemPadding)) {
                    Mark(SignInBand.mark)
                }
            }

            if (beside) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(SignInBand.gap)) {
                    emblem()
                    Heading(title, Modifier.weight(1f))
                }
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(SignInBand.rowGap)) {
                    emblem()
                    Heading(title, Modifier)
                }
            }
        }
        Spacer(Modifier.fillMaxWidth().height(5.dp).background(tones.accent))
    }
}

/** Heading is the band's words, white on its dark, as large as the sign-in card's welcome. */
@Composable
private fun Heading(title: String, modifier: Modifier) {
    Text(title, color = Color.White, fontSize = SignInBand.heading, lineHeight = SignInBand.heading * 1.25f, modifier = modifier)
}

/**
 * Mark is the application's mark: the drawable ic_mark, which is favicon.svg with
 * the SVG's own path data. It was drawn here point by point, from numbers worked
 * out of the SVG's, which is a second copy of the drawing that nothing compared.
 */
@Composable
fun Mark(size: Dp) {
    Image(painterResource(R.drawable.ic_mark), contentDescription = null, modifier = Modifier.size(size))
}

/**
 * Field is an input as the web interface draws one: its label above in the quiet
 * colour, the box a pill on a tint of the text over the card with no border until
 * it has the focus - app.css's --field-bg, mixed the same way - and then the card's
 * own colour inside the accent.
 */
@Composable
fun Field(label: String, value: String, onValueChange: (String) -> Unit, keyboard: KeyboardType, secret: Boolean = false) {
    val tones = LocalTones.current
    val box = mixed(tones.text, tones.surface, 0.06f)

    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(label, color = tones.muted, fontSize = 13.sp)
        OutlinedTextField(
            value = value,
            onValueChange = onValueChange,
            singleLine = true,
            shape = pill,
            visualTransformation = if (secret) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions = KeyboardOptions(keyboardType = keyboard),
            colors = OutlinedTextFieldDefaults.colors(
                focusedContainerColor = tones.surface,
                unfocusedContainerColor = box,
                focusedBorderColor = tones.accent,
                unfocusedBorderColor = Color.Transparent,
                focusedTextColor = tones.text,
                unfocusedTextColor = tones.text,
                cursorColor = tones.accent,
            ),
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

/**
 * Action is a button as the web interface draws one: a pill 48 high, [primary]
 * filled with the accent, otherwise a tint of the text on the card with no
 * border, app.css's --quiet - with [icon] before its words where it has one.
 */
@Composable
fun Action(text: String, primary: Boolean, onClick: () -> Unit, enabled: Boolean = true, icon: (@Composable () -> Unit)? = null) {
    val tones = LocalTones.current
    val ground = if (primary) tones.accent else mixed(tones.text, tones.surface, 0.07f)

    Button(
        onClick = onClick,
        enabled = enabled,
        shape = pill,
        colors = ButtonDefaults.buttonColors(
            containerColor = ground,
            contentColor = if (primary) tones.accentText else tones.text,
            disabledContainerColor = mixed(ground, tones.surface, 0.45f),
            disabledContentColor = mixed(if (primary) tones.accentText else tones.text, tones.surface, 0.6f),
        ),
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
