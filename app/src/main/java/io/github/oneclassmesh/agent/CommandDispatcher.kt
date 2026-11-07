package io.github.oneclassmesh.agent

import android.app.admin.DevicePolicyManager
import android.content.ComponentName
import android.content.Context
import android.media.AudioManager
import android.util.Log
import com.google.gson.JsonNull
import com.google.gson.JsonObject
import com.google.gson.JsonParser
import java.util.Collections

/**
 * Protocol v0.1 command dispatcher.
 * Pipeline: parse -> msgId dedup -> HMAC verify -> EXECUTING receipt -> execute -> final receipt.
 */
class CommandDispatcher(
    private val context: Context,
    private val sendReceipt: (String) -> Unit
) {
    companion object {
        const val TAG = "OcmAgent"
        const val LOCK = "LOCK_SCREEN"
        const val UNLOCK = "UNLOCK_SCREEN"
        const val MESSAGE = "SEND_MESSAGE"
        const val VOLUME = "SET_VOLUME"
        const val REBOOT = "REBOOT"
        private const val MAX_SEEN = 500
    }

    private val seenIds: MutableSet<String> =
        Collections.synchronizedSet(HashSet(loadSeen()))

    fun onCommand(json: String) {
        try {
            val root = JsonParser.parseString(json).asJsonObject
            val msgId = root.get("msgId")?.asString ?: return
            val type = root.get("type")?.asString ?: return
            val ts = root.get("ts")?.asLong ?: System.currentTimeMillis()

            if (!seenIds.add(msgId)) {
                Log.i(TAG, "duplicate msgId, skipped: $msgId")
                return
            }
            persistSeen()

            val sig = root.get("sig")?.asString ?: ""
            val payloadElement = root.get("payload") ?: JsonNull.INSTANCE
            val payload = if (payloadElement.isJsonObject) payloadElement.asJsonObject else JsonObject()

            val canonical = msgId + "\n" + type + "\n" +
                CanonicalJson.canonical(payloadElement) + "\n" + ts
            val secret = Prefs.deviceSecret(context)
            if (secret.isNotEmpty() && !Hmac.verify(secret, canonical, sig)) {
                sendReceiptJson(msgId, "FAILED", null, "E_BAD_SIG", "signature mismatch")
                seenIds.remove(msgId)
                return
            }

            sendReceiptJson(msgId, "EXECUTING", null, null, null)

            val error = execute(type, payload)
            if (error == null) {
                sendReceiptJson(msgId, "SUCCESS", null, null, null)
            } else {
                seenIds.remove(msgId)
                sendReceiptJson(msgId, "FAILED", null, error.first, error.second)
            }
            persistSeen()
        } catch (e: Exception) {
            Log.e(TAG, "dispatch error", e)
        }
    }

    private fun execute(type: String, payload: JsonObject): Pair<String, String>? {
        return when (type) {
            LOCK -> {
                val dpm = context.getSystemService(Context.DEVICE_POLICY_SERVICE) as DevicePolicyManager
                val admin = ComponentName(context, AdminReceiver::class.java)
                if (!dpm.isAdminActive(admin)) {
                    Pair("E_PLATFORM_UNSUPPORTED", "device admin not enabled")
                } else {
                    dpm.lockNow()
                    null
                }
            }
            UNLOCK -> {
                MessageOverlay.dismiss()
                null
                unlock... null
            }
            MESSAGE -> {
                val title = payload.get("title")?.asString ?: ""
                val body = payload.get("body")?.asString ?: ""
                val ttl = payload.get("ttlSeconds")?.asInt ?: 10
                MessageOverlay.show(context, title, body, ttl * 1000L)
                null
            SEND_MESSAGE... }
            VOLUME -> {
                val volume = payload.get("volume")?.asInt ?: 50
                val am = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
                val max = am.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
                val index = (volume.coerceIn(0, 100) * max + 99) / 100
                am.setStreamVolume(AudioManager.STREAM_MUSIC, index, 0)
                null
            }
            REBOOT -> {
                val dpm = context.getSystemService(Context.DEVICE_POLICY_SERVICE) as DevicePolicyManager
                val admin = ComponentName(context, AdminReceiver::class.java)
                if (!dpm.isDeviceOwnerApp(context.packageName)) {
                    Pair("E_PLATFORM_UNSUPPORTED", "requires device owner")
                } else {
                    dpm.reboot(admin)
                    null
                }
            }
            else -> Pair("E_UNSUPPORTED_TYPE", "unknown type: $type")
        }
    }

    private fun sendReceiptJson(
        msgId: String,
        status: String,
        result: JsonObject?,
        errorCode: String?,
        errorMessage: String?
    ) {
        val receipt = JsonObject().apply {
            addProperty("msgId", msgId)
            addProperty("status", status)
            add("result", result ?: JsonObject())
            if (errorCode != null) {
                val err = JsonObject().apply {
                    addProperty("code", errorCode)
                    addProperty("message", errorMessage ?: "")
                }
                add("error", err)
            } else {
                add("error", JsonNull.INSTANCE)
            }
            addProperty("ts", System.currentTimeMillis())
        }
        sendReceipt(receipt.toString())
    }

    private fun loadSeen(): Set<String> =
        context.getSharedPreferences(Prefs.FILE, Context.MODE_PRIVATE)
            .getStringSet("seenIds", emptySet()) ?: emptySet()

    private fun persistSeen() {
        val sp = context.getSharedPreferences(Prefs.FILE, Context.MODE_PRIVATE)
        val snapshot = seenIds.toSet()
        val trimmed = if (snapshot.size > MAX_SEEN) snapshot.takeLast(MAX_SEEN).toSet() else snapshot
        sp.edit().putStringSet("seenIds", trimmed).apply()
    }
}
