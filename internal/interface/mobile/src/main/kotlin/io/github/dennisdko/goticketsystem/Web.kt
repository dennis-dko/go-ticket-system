package io.github.dennisdko.goticketsystem

import android.annotation.SuppressLint
import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.util.Base64
import android.view.ContextThemeWrapper
import android.view.ViewGroup
import android.webkit.JavascriptInterface
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.viewinterop.AndroidView
import java.net.URI

/**
 * WebScreen is the installation's web interface, shown whole: every view, card
 * and form the browser has, in its design, saved through its API into the same
 * database. What the app adds is only what a page cannot do inside an app by
 * itself - choose a file, save one, open a link that is not the installation's -
 * and the two things it is told of: signing out, which it watches the page's
 * requests for and which disconnects the app as well, so that a phone handed on
 * does not sign its next holder back in; and a session that ended without that,
 * which the page tells it of and [onSessionEnded] answers.
 *
 * [shown] counts the times the session was opened again, and the page is loaded
 * afresh whenever it changes.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun WebScreen(address: String, words: Language, shown: Int, onSignedOut: () -> Unit, onSessionEnded: () -> Unit) {
    val context = LocalContext.current
    var back by remember { mutableStateOf(false) }

    // What a page asked to choose, and what it asked to save, held while the
    // phone's own picker is open.
    val choosing = remember { Holder<ValueCallback<Array<Uri>>>() }
    val saving = remember { Holder<ByteArray>() }

    val choose = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        choosing.take()?.onReceiveValue(uri?.let { arrayOf(it) })
    }
    val save = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("*/*")) { uri ->
        val bytes = saving.take() ?: return@rememberLauncherForActivityResult
        if (uri == null) return@rememberLauncherForActivityResult

        context.contentResolver.openOutputStream(uri)?.use { it.write(bytes) }
        Toast.makeText(context, words.text("mobile.file.saved", "Saved"), Toast.LENGTH_SHORT).show()
    }

    // The page's files are saved through the app; see res/raw/keep_download.js.
    val resources = LocalResources.current
    val keepDownloads = remember {
        resources.openRawResource(R.raw.keep_download).bufferedReader(Charsets.UTF_8).use { it.readText() }
    }

    // The web interface's automatic appearance asks prefers-color-scheme, which a
    // WebView answers from the isLightTheme of the theme it was made with rather
    // than from the phone: the app's own theme is light, so it is made with the
    // theme of the mode the phone is in. A change of mode makes the activity
    // anew, and the WebView with it.
    val dark = isSystemInDarkTheme()

    val view = remember {
        WebView(
            ContextThemeWrapper(
                context,
                if (dark) android.R.style.Theme_Material_NoActionBar else android.R.style.Theme_Material_Light_NoActionBar,
            ),
        ).apply {
            // A WebView whose height is WRAP_CONTENT lays its page out in a viewport
            // of no height, so every vh and dvh in the page comes to nothing - and
            // WRAP_CONTENT is what AndroidView gives a view without layout params of
            // its own. The page's opened menu and its tour, each held to 100dvh, were
            // drawn as thin as their padding, under the cards they should cover.
            layoutParams = ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)

            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true

            addJavascriptInterface(
                Files { name, bytes ->
                    saving.put(bytes)
                    save.launch(name)
                },
                filesInterface,
            )

            addJavascriptInterface(Session { Handler(Looper.getMainLooper()).post(onSessionEnded) }, sessionInterface)

            webViewClient = object : WebViewClient() {
                override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                    val url = request.url.toString()
                    if (belongsTo(address, url)) return false

                    try {
                        context.startActivity(Intent(Intent.ACTION_VIEW, request.url))
                    } catch (_: ActivityNotFoundException) {
                        // Nothing on the phone opens it; the page stays as it is.
                    }

                    return true
                }

                override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                    if (request.method == "POST" && signsOut(address, request.url.toString())) {
                        Handler(Looper.getMainLooper()).post(onSignedOut)
                    }

                    return null
                }

                override fun onPageFinished(view: WebView, url: String) {
                    view.evaluateJavascript(keepDownloads, null)
                }

                override fun doUpdateVisitedHistory(view: WebView, url: String?, isReload: Boolean) {
                    back = view.canGoBack()
                }
            }

            webChromeClient = object : WebChromeClient() {
                override fun onShowFileChooser(
                    view: WebView,
                    callback: ValueCallback<Array<Uri>>,
                    params: FileChooserParams,
                ): Boolean {
                    choosing.take()?.onReceiveValue(null)
                    choosing.put(callback)

                    val types = params.acceptTypes.filter { it.isNotBlank() }.ifEmpty { listOf("*/*") }
                    choose.launch(types.toTypedArray())

                    return true
                }
            }
        }
    }

    LaunchedEffect(shown) {
        view.loadUrl(address.trimEnd('/') + "/")
    }

    DisposableEffect(view) {
        onDispose { view.destroy() }
    }

    BackHandler(enabled = back) { view.goBack() }

    AndroidView(factory = { view }, modifier = Modifier.fillMaxSize().safeDrawingPadding())
}

/** Holder keeps one thing between a request and the answer to it. */
private class Holder<T> {
    private var held: T? = null

    fun put(value: T) {
        held = value
    }

    fun take(): T? = held.also { held = null }
}

/** Files is what the page's saving reaches: a file it made, by name, to be saved where its owner chooses. */
private class Files(private val onSave: (String, ByteArray) -> Unit) {

    @JavascriptInterface
    fun save(base64: String, name: String) {
        val bytes = runCatching { Base64.decode(base64, Base64.DEFAULT) }.getOrNull() ?: return

        Handler(Looper.getMainLooper()).post { onSave(downloadName(name), bytes) }
    }
}

/** filesInterface is the name the page reaches [Files] by, which res/raw/keep_download.js calls. */
private const val filesInterface = "GoTicketSystemFiles"

/**
 * Session is how the page tells the app that its session ended without its
 * being signed out of - which the app cannot see for itself, because the page
 * learns it from an answer to one of its own requests. Signing out the app
 * watches for in the requests, and that disconnects it instead.
 */
private class Session(private val ended: () -> Unit) {

    @JavascriptInterface
    fun ended() = ended.invoke()
}

/** sessionInterface is the name the page reaches [Session] by, which app.js tells when its session has ended. */
private const val sessionInterface = "GoTicketSystemSession"

/** signsOut says whether a request is the web interface signing out of the installation the app shows. */
fun signsOut(address: String, url: String): Boolean =
    belongsTo(address, url) && runCatching { URI(url).path }.getOrNull().orEmpty().trimEnd('/').endsWith("/api/v1/auth/logout")

/** downloadName is a name a file can be saved under: the page's, without anything that names a folder. */
fun downloadName(name: String): String =
    name.substringAfterLast('/').substringAfterLast('\\').trim().ifEmpty { "download" }
