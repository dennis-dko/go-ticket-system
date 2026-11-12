package io.github.dennisdko.goticketsystem

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.core.content.edit
import java.net.URI
import java.net.URLDecoder
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Connection is an installation and the token the app asks it with. */
data class Connection(val address: String, val token: String)

/** connectScheme is the scheme of the address a token's code holds, which the app is registered for. */
const val connectScheme = "go-ticket-system"

/**
 * connectionIn reads the address a token's code holds -
 * go-ticket-system://connect?url=…&token=… - and answers null for anything
 * else. The order of the two values is not to be relied on, and both are
 * query-encoded.
 */
fun connectionIn(link: String): Connection? {
    val uri = runCatching { URI(link) }.getOrNull() ?: return null
    if (uri.scheme != connectScheme || uri.host != "connect") return null

    val values = uri.rawQuery.orEmpty().split('&').mapNotNull { pair ->
        val name = pair.substringBefore('=', "")
        val value = pair.substringAfter('=', "")
        if (name.isEmpty()) null else decoded(name) to decoded(value)
    }.toMap()

    val address = values["url"]?.trim()?.trimEnd('/').orEmpty()
    val token = values["token"]?.trim().orEmpty()

    if (!speaksHttp(address) || token.isEmpty()) return null

    return Connection(address, token)
}

/** speaksHttp says whether an address is one the app can ask: http or https, as the browser shows it. */
fun speaksHttp(address: String): Boolean {
    val uri = runCatching { URI(address) }.getOrNull() ?: return false

    return (uri.scheme == "https" || uri.scheme == "http") && !uri.host.isNullOrEmpty()
}

private fun decoded(part: String): String = URLDecoder.decode(part, "UTF-8")

/**
 * ConnectionStore keeps the connection on this phone between runs, and only the
 * connection: what it shows is read from the installation every time.
 *
 * The token is a bearer credential carrying the whole of its owner's role, so it
 * is kept sealed with a key the Android keystore holds and never hands out. A
 * copy of the app's data restored on another phone - which allowBackup="false"
 * mostly prevents - holds nothing that phone can open, and the app asks to be
 * connected again rather than using a token it cannot read.
 */
class ConnectionStore(context: Context) {

    private val kept = context.getSharedPreferences("connection", Context.MODE_PRIVATE)

    /** load is the connection kept on this phone, or null where there is none it can open. */
    fun load(): Connection? {
        val address = kept.getString("address", null) ?: return null
        val sealed = kept.getString("token", null) ?: return null
        val token = runCatching { open(sealed) }.getOrNull() ?: return null

        return Connection(address, token)
    }

    /** save keeps a connection, in place of any kept before. */
    fun save(connection: Connection) {
        kept.edit {
            putString("address", connection.address)
            putString("token", seal(connection.token))
        }
    }

    /** forget takes the connection off this phone, and the key that sealed its token with it. */
    fun forget() {
        kept.edit { clear() }
        runCatching { keystore().deleteEntry(keyName) }
    }

    private fun seal(text: String): String {
        val cipher = Cipher.getInstance(sealing)
        cipher.init(Cipher.ENCRYPT_MODE, key())

        return Base64.encodeToString(cipher.iv + cipher.doFinal(text.toByteArray(Charsets.UTF_8)), Base64.NO_WRAP)
    }

    private fun open(sealed: String): String {
        val bytes = Base64.decode(sealed, Base64.NO_WRAP)
        val cipher = Cipher.getInstance(sealing)
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes, 0, ivLength))

        return String(cipher.doFinal(bytes, ivLength, bytes.size - ivLength), Charsets.UTF_8)
    }

    private fun keystore(): KeyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    private fun key(): SecretKey {
        (keystore().getKey(keyName, null) as? SecretKey)?.let { return it }

        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(
            KeyGenParameterSpec.Builder(keyName, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build(),
        )

        return generator.generateKey()
    }

    private companion object {
        const val keyName = "connection-token"
        const val sealing = "AES/GCM/NoPadding"
        const val ivLength = 12
    }
}
