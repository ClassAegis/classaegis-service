package io.github.oneclassmesh.agent

import android.app.admin.DeviceAdminReceiver
import android.content.Context
import android.content.Intent

/**
 * 壹课管家 设备管理员接收器
 * 通过 adb 设置 Device Owner 后,获得锁屏(lockNow)与重启(reboot)能力
 */
class AdminReceiver : DeviceAdminReceiver() {
    override fun onEnabled(context: Context, intent: Intent) {
        // 激活设备管理员后由 MainActivity 引导设置 Device Owner
    }
}
