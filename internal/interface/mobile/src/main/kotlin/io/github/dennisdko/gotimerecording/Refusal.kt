package io.github.dennisdko.gotimerecording

import org.json.JSONArray
import org.json.JSONObject
import java.time.LocalDate

/** Refused is what the installation answered instead of doing something, in words the person holding the phone can read. */
open class Refused(message: String) : Exception(message)

/**
 * TokenRefused is the installation no longer knowing the app's token - taken
 * back under API tokens, or run out - which nothing the app retries can mend, so
 * the app disconnects and says so rather than trying again.
 */
class TokenRefused(message: String) : Refused(message)

/**
 * refusalValues names which values of which refusal the web interface writes in
 * its own words, by the name of the page's function for each - REFUSAL_VALUES in
 * its script, which a test in internal/interface/mobile holds this to, so that a
 * status, a kind of record or a day reaches a sentence the way the screen writes
 * it rather than as the word it travels in.
 */
val refusalValues: Map<String, Map<Int, String>> = mapOf(
    "archiveNeedsCompleted" to mapOf(0 to "statusName"),
    "notFound" to mapOf(0 to "entityName"),
    "overDailyLimit" to mapOf(2 to "fmtDate"),
    "projectClosedForBooking" to mapOf(1 to "statusName"),
)

/**
 * refused reads what the installation said when it refused a request: not
 * knowing the token in the app's own words, because the installation's are
 * about a session; anything else in the web interface's words for the code it
 * sent, and otherwise in the installation's own.
 */
fun Language.refused(status: Int, reason: String, body: String): String {
    if (status == 401) {
        return text(
            "mobile.tokenUnknown",
            "the installation does not know this token - make a new one under My account, API tokens, and scan its code",
        )
    }

    val error = runCatching { JSONObject(body).optJSONObject("error") }.getOrNull()

    if (error != null) {
        inPageWords(error)?.let { return it }

        val message = error.optString("message")
        if (message.isNotEmpty()) return message
    }

    return text("mobile.answered", "the installation answered {0} {1}", status.toString(), reason)
}

/**
 * inPageWords is a refusal as the web interface says it, or null where its
 * dictionary has no sentence for it - which in English is always, the page's
 * English being the fallbacks in its own script. The fields named first,
 * because which ones is more use than any sentence.
 */
private fun Language.inPageWords(error: JSONObject): String? {
    val code = error.optString("code")
    if (code == "maintenance") return lookup("err.maintenance")

    val fields = lookup("err.invalidFields")
    val named = error.optJSONArray("param")
    if (fields != null && named != null && named.length() > 0) {
        return fields + ": " + (0 until named.length()).joinToString(", ") { pageWord("field.", named.optString(it)) }
    }

    if (code.isEmpty()) return null

    val sentence = lookup("err.$code") ?: return null

    return Language.fill(sentence, *valuesOf(code, error.optJSONArray("values")))
}

/**
 * valuesOf writes the values of a refusal the way the page's REFUSAL_VALUES and
 * fillIn do: a status and a kind of record in its words, a day the way the
 * reader writes one, a whole number as it is and a fraction to two places.
 */
private fun Language.valuesOf(code: String, values: JSONArray?): Array<String> {
    if (values == null) return emptyArray()

    val named = refusalValues[code].orEmpty()

    return Array(values.length()) { i ->
        when (val value = values.opt(i)) {
            is String -> when (named[i]) {
                "statusName" -> pageWord("status.", value)
                "entityName" -> pageWord("entity.", value)
                "fmtDate" -> runCatching { day(LocalDate.parse(value)) }.getOrDefault(value)
                else -> value
            }
            is Number -> value.toDouble().let { if (it == Math.floor(it) && !it.isInfinite()) it.toLong().toString() else number(it) }
            null, JSONObject.NULL -> ""
            else -> value.toString()
        }
    }
}

/** pageWord is one of the web interface's words, or the word itself where it has none. */
private fun Language.pageWord(prefix: String, word: String): String = lookup(prefix + word) ?: word
