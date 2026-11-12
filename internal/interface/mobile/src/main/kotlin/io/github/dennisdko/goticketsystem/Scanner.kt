package io.github.dennisdko.goticketsystem

import android.os.SystemClock
import android.view.ViewGroup
import androidx.activity.compose.BackHandler
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.RoundRect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.LuminanceSource
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.ReaderException
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/**
 * ScanScreen is the phone's camera looking for a token's code - the one the web
 * interface shows under My account, API tokens, once a token is made - until it
 * sees one, which it hands to [onFound]. A code that is something else is said
 * to be, and the camera goes on looking.
 */
@Composable
fun ScanScreen(words: Language, onFound: (Connection) -> Unit, onCancel: () -> Unit) {
    val context = LocalContext.current
    val lifecycle = LocalLifecycleOwner.current
    val tones = LocalTones.current

    var said by remember { mutableStateOf<String?>(null) }

    // A TextureView rather than a SurfaceView, so that the window drawn over the
    // picture composes with it like any other view. MATCH_PARENT for the reason
    // Web.kt gives its WebView: AndroidView would make it WRAP_CONTENT.
    val preview = remember {
        PreviewView(context).apply {
            layoutParams = ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
            implementationMode = PreviewView.ImplementationMode.COMPATIBLE
            scaleType = PreviewView.ScaleType.FILL_CENTER
        }
    }

    DisposableEffect(lifecycle) {
        val main = ContextCompat.getMainExecutor(context)
        val reading = Executors.newSingleThreadExecutor()
        val future = ProcessCameraProvider.getInstance(context)
        val found = AtomicBoolean(false)
        var disposed = false

        future.addListener({
            if (disposed) return@addListener

            val provider = future.get()

            val shown = Preview.Builder().build().also { it.setSurfaceProvider(preview.surfaceProvider) }
            val read = ImageAnalysis.Builder().setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST).build()

            // A few frames a second rather than every one: a code held in front of
            // the camera is found as soon, and reading every frame kept the phone's
            // garbage collector running twice a second for nothing.
            var readAt = 0L

            read.setAnalyzer(reading) { frame ->
                val text = frame.use {
                    val now = SystemClock.elapsedRealtime()

                    if (now - readAt < readEvery) {
                        null
                    } else {
                        readAt = now
                        codeIn(it)
                    }
                } ?: return@setAnalyzer

                main.execute {
                    val connection = connectionIn(text)

                    when {
                        connection == null -> said = words.text("mobile.scan.notThis", "That is not a token's code.")
                        found.compareAndSet(false, true) -> onFound(connection)
                    }
                }
            }

            // The camera on the back where there is one, and the only one there
            // is where it is not - a tablet may have only the front one.
            val back = runCatching { provider.hasCamera(CameraSelector.DEFAULT_BACK_CAMERA) }.getOrDefault(true)
            val camera = if (back) CameraSelector.DEFAULT_BACK_CAMERA else CameraSelector.DEFAULT_FRONT_CAMERA

            try {
                provider.unbindAll()
                provider.bindToLifecycle(lifecycle, camera, shown, read)
            } catch (_: IllegalArgumentException) {
                said = words.text("mobile.scan.noCamera", "The camera could not be opened.")
            } catch (_: IllegalStateException) {
                said = words.text("mobile.scan.noCamera", "The camera could not be opened.")
            }
        }, main)

        onDispose {
            disposed = true
            if (future.isDone) runCatching { future.get().unbindAll() }
            reading.shutdown()
        }
    }

    BackHandler(onBack = onCancel)

    Box(Modifier.fillMaxSize().background(Color.Black)) {
        AndroidView(factory = { preview }, modifier = Modifier.fillMaxSize())

        // The picture under a dark veil, with a window over its middle where the
        // code is to go, edged in the accent.
        Canvas(Modifier.fillMaxSize()) {
            val side = minOf(size.width, size.height) * 0.68f
            val window = Rect(Offset((size.width - side) / 2, (size.height - side) / 2), Size(side, side))
            val corner = CornerRadius(24.dp.toPx())

            drawPath(
                Path().apply {
                    fillType = PathFillType.EvenOdd
                    addRect(Rect(Offset.Zero, size))
                    addRoundRect(RoundRect(window, corner))
                },
                Color.Black.copy(alpha = 0.6f),
            )
            drawRoundRect(tones.accent, topLeft = window.topLeft, size = window.size, cornerRadius = corner, style = Stroke(3.dp.toPx()))
        }

        Column(
            Modifier.fillMaxSize().safeDrawingPadding().padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween,
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(words.text("mobile.scan.title", "Scan the token's code"), color = Color.White, fontSize = 22.sp, textAlign = TextAlign.Center)
                Text(
                    words.text("mobile.scan.hint", "Make a token under My account, API tokens, and hold the phone over its code."),
                    color = Color.White.copy(alpha = 0.82f),
                    fontSize = 15.sp,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.widthIn(max = 420.dp),
                )
            }

            Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.widthIn(max = 420.dp)) {
                said?.let {
                    Text(
                        it,
                        color = Color.White,
                        fontSize = 14.sp,
                        textAlign = TextAlign.Center,
                        modifier = Modifier
                            .clip(RoundedCornerShape(8.dp))
                            .background(Color.Black.copy(alpha = 0.55f))
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                    )
                }

                Action(words.text("action.cancel", "Cancel"), primary = false, onClick = onCancel)
            }
        }
    }
}

/** readEvery is the least time between two frames the scanner reads, in milliseconds. */
private const val readEvery = 200L

/** codeIn reads a camera frame's brightness - its first plane - for a QR code. */
private fun codeIn(frame: ImageProxy): String? {
    val plane = frame.planes[0]
    val buffer = plane.buffer
    val bytes = ByteArray(buffer.remaining()).also { buffer.get(it) }

    return codeIn(bytes, frame.width, frame.height, plane.rowStride)
}

/**
 * codeIn is the text of the QR code in a picture's brightness - one byte a
 * pixel, [rowStride] bytes a row, as a camera's first plane holds it - or null
 * where it holds none that can be read. A code drawn light on dark is read as
 * well as one drawn dark on light, which is how a screen in dark mode may show
 * one.
 */
fun codeIn(luminance: ByteArray, width: Int, height: Int, rowStride: Int): String? {
    val packed = if (rowStride == width) {
        luminance
    } else {
        ByteArray(width * height).also { out ->
            for (row in 0 until height) System.arraycopy(luminance, row * rowStride, out, row * width, width)
        }
    }

    val source = PlanarYUVLuminanceSource(packed, width, height, 0, 0, width, height, false)

    return read(source) ?: read(source.invert())
}

private val hints = mapOf(DecodeHintType.TRY_HARDER to true)

private fun read(source: LuminanceSource): String? = try {
    QRCodeReader().decode(BinaryBitmap(HybridBinarizer(source)), hints).text
} catch (_: ReaderException) {
    null
}
