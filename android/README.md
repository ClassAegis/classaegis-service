# android/ — Kotlin Agent(M0)

- minSdk 24(Android 7,一体机主流 7~11)
- Device Owner 激活:`adb shell dpm set-device-owner`,M0 验收用旧平板
- M0 指令:LOCK_SCREEN / UNLOCK_SCREEN / SEND_MESSAGE / SET_VOLUME / REBOOT
- 前台服务 + BOOT_COMPLETED 开机自启 + msgId 幂等 + HMAC 验签
- 构建工具 Gradle 8.x + AGP 8.x(需 JDK 17)
