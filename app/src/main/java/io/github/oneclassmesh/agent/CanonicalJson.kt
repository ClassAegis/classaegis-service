package io.github.oneclassmesh.agent

import com.google.gson.GsonBuilder
import com.google.gson.JsonArray
import com.google.gson.JsonElement
import com.google.gson.JsonObject

/**
 * Canonical JSON serialization for protocol v0.1 signature.
 * Rule: keys sorted lexicographically, compact separators (no whitespace).
 * Server (Jackson) and Agent (Gson) must produce identical output.
 */
object CanonicalJson {
    private val gson = GsonBuilder().disableHtmlEscaping().create()

    fun canonical(element: JsonElement): String = gson.toJson(sortTree(element))

    fun parse(text: String): JsonElement = gson.fromJson(text, JsonElement::class.java)

    private fun sortTree(element: JsonElement): JsonElement = when (element) {
        is JsonObject -> JsonObject().apply {
            element.entrySet()
                .sortedBy { it.key }
                .forEach { add(it.key, sortTree(it.value)) }
        }
        is JsonArray -> JsonArray().apply {
            element.forEach { add(sortTree(it)) }
        }
        else -> element
    }
}
