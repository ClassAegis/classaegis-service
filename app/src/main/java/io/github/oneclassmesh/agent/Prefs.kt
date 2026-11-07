package io.github.oneclassmesh.agent

import android.content.Context
import android.content.SharedPreferences

/**
 * Device-side credentials and config storage.
 * Written on activation success: schoolCode / sn / deviceSecret / mqttToken / broker address.
 */
object Prefs {
    private const val FILE = "oneclassmesh"

    private fun sp(context: Context): SharedPreferences =
        context.getSharedPreferences(FILE, Context.MODE_PRIVATE)

    fun saveActivation(
        context: Context,
        schoolCode: String,
        sn: String,
        deviceSecret: String,
        mqttToken: String,
        brokerHost: String,
        brokerPort: Int
    ) {
        sp(context).edit()
            .putString("schoolCode", schoolCode)
            .putString("sn", sn)
            .putString("deviceSecret", deviceSecret)
            .putString("mqttToken", mqttToken)
            .putString("brokerHost", brokerHost)
            .putString("brokerPort", brokerPort.toString())
            .apply()
    }

    fun isActivated(context: Context): Boolean = sp(context).contains("sn")

    fun schoolCode(context: Context): String = sp(context).getString("schoolCode", "") ?: ""
    fun sn(context: Context): String = sp(context).getString("sn", "") ?: ""
    fun deviceSecret(context: Context): String = sp(context).getString("deviceSecret", "") ?: ""
    fun mqttToken(context: Context): String = sp(context).getString("mqttToken", "") ?: ""
    fun brokerHost(context: Context): String = sp(context).getString("brokerHost", "") ?: ""
    fun brokerPort(context: Context): Int = sp(context).getString("brokerPort", "1883")!!.toInt()
}
