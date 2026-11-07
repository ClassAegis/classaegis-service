package io.github.oneclassmesh.agent

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import org.eclipse.paho.client.mqttv3.IMqttDeliveryToken
import org.eclipse.paho.client.mqttv3.MqttAsyncClient
import org.eclipse.paho.client.mqttv3.MqttCallbackExtended
import org.eclipse.paho.client.mqttv3.MqttConnectOptions
import org.eclipse.paho.client.mqttv3.MqttMessage
import org.eclipse.paho.client.mqttv3.persist.MqttDefaultFilePersistence
import java.util.Timer
import java.util.TimerTask

class MqttService : Service() {

    companion object {
        const val TAG = "OcmAgent"
        const val CHANNEL_ID = "oneclassmesh-agent"
        const val NOTIFICATION_ID = 1
        const val TOPIC_DOWN_FMT = "oneclassmesh/%s/%s/cmd/down"
        const val TOPIC_UP_FMT = "oneclassmesh/%s/%s/cmd/up"
        const val TOPIC_HB_FMT = "oneclassmesh/%s/%s/hb"
        const val TOPIC_LWT_FMT = "oneclassmesh/%s/%s/lwt"
    }

    private var client: MqttAsyncClient? = null
    private var heartbeat: Timer? = null
    private var dispatcher: CommandDispatcher? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
        startForeground(NOTIFICATION_ID, buildNotification("starting"))
        dispatcher = CommandDispatcher(this)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        connectIfReady()
        return START_STICKY
    }

    override fun onDestroy() {
        heartbeat?.cancel()
        client?.let { c ->
            try {
                c.disconnect().waitForCompletion(2000)
            } catch (_: Exception) {
            }
        }
        super.onDestroy()
    }

    private fun connectIfReady() {
        if (!Prefs.isActivated(this)) {
            updateNotification("not activated")
            return
        }
        if (client?.isConnected == true) return
        try {
            val code = Prefs.schoolCode(this)
            val sn = Prefs.sn(this)
            val client = MqttAsyncClient(
                "tcp://" + Prefs.brokerHost(this) + ":" + Prefs.brokerPort(this),
                sn,
                MqttDefaultFilePersistence(cacheDir.absolutePath)
            )
            val options = MqttConnectOptions().apply {
                userName = sn
                password = Prefs.mqttToken(this@MqttService).toCharArray()
                isCleanSession = false
                keepAliveInterval = 30
                connectionTimeout = 10
                setWill(
                    String.format(TOPIC_LWT_FMT, code, sn),
                    """{"msgId":null,"status":"OFFLINE","ts":${System.currentTimeMillis()}}""".toByteArray(),
                    1,
                    false
                )
                isAutomaticReconnect = true
            }
            client.setCallback(object : MqttCallbackExtended {
                override fun connectComplete(reconnect: Boolean, serverURI: String) {
                    client.subscribe(String.format(TOPIC_DOWN_FMT, code, sn), 1)
                    startHeartbeat(code, sn)
                    updateNotification("online")
                }

                override fun connectionLost(cause: Throwable?) {
                    heartbeat?.cancel()
                    updateNotification("reconnecting")
                }

                override fun deliveryComplete(token: IMqttDeliveryToken?) {}

                override fun messageArrived(topic: String, message: MqttMessage) {
                    dispatcher?.onCommand(message.payload.toString(Charsets.UTF_8))
                }
            })
            this.client = client
            client.connect(options)
        } catch (e: Exception) {
            Log.e(TAG, "connect failed", e)
            updateNotification("connect failed: " + e.message)
        }
    }

    private fun startHeartbeat(code: String, sn: String) {
        heartbeat?.cancel()
        heartbeat = Timer().apply {
            schedule(object : TimerTask() {
                override fun run() {
                    publish(TOPIC_HB_FMT, code, sn, buildHeartbeatJson())
                }
            }, 0, 60_000)
        }
    }

    fun publishHeartbeat() {
        val code = Prefs.schoolCode(this)
        val sn = Prefs.sn(this)
        publish(TOPIC_HB_FMT, code, sn, buildHeartbeatJson())
    }

    fun publishReceipt(json: String) {
        val code = Prefs.schoolCode(this)
        val sn = Prefs.sn(this)
        publish(TOPIC_UP_FMT, code, sn, json)
    }

    private fun publish(fmt: String, code: String, sn: String, payload: String) {
        try {
            val msg = MqttMessage(payload.toByteArray(Charsets.UTF_8)).apply { qos = 1 }
            client?.publish(String.format(fmt, code, sn), msg)
        } catch (e: Exception) {
            Log.w(TAG, "publish failed", e)
        }
    }

    private fun buildHeartbeatJson(): String {
        val mem = Runtime.getRuntime()
        val metrics = """{"memoryFreeMB":${(mem.freeMemory() / 1048576)}}"""
        return """{"msgId":null,"sn":"${Prefs.sn(this)}","protocolVersion":"0.1","agentVersion":"0.1.0","status":"ONLINE","metrics":$metrics,"ts":${System.currentTimeMillis()}}"""
    }

    private fun createChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "agent service",
            NotificationManager.IMPORTANCE_LOW
        )
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun buildNotification(text: String): android.app.Notification {
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("壹课管家")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val nm = getSystemService(NotificationManager::class.java)
        nm.notify(NOTIFICATION_ID, buildNotification(text))
    }
}
