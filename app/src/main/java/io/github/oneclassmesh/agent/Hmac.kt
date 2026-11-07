package io.github.oneclassmesh.agent

import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

/**
 * HMAC-SHA256 signature helper.
 * Canonical string per protocol v0.1 section 5.1:
 *   msgId + "\n" + type + "\n" + payloadJson(sorted keys, compact) + "\n" + ts
 */
object Hmac {
    private const val ALGORITHM = "HmacSHA256"

    fun sign(deviceSecret: String, canonical: String): String {
        val mac = Mac.getInstance(ALGORITHM)
        mac.init(SecretKeySpec(deviceSecret.toByteArray(Charsets.UTF_8), ALGORITHM))
        return mac.doFinal(canonical.toByteArray(Charsets.UTF_8)).toHexString()
    }

    fun verify(deviceSecret: String, canonical: String, expectedHex: String): Boolean {
        val computed = sign(deviceSecret, canonical)
        return constantTimeEquals(computed, expectedHex)
    }

    private fun constantTimeEquals(a: String, b: String): Boolean {
        if (a.length != b.length) return false
        var result = 0
        for (i in a.indices) result = result or (a[i].code xor b[i].code)
        return result == 0
    }

    private fun ByteArray.toHexString(): String =
        joinToString("") { "%02x".format(it) }
}
