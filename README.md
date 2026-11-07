# OneClassMesh 客户端 · 壹课管家(oneclassmesh-agent)

设备侧 Agent:Android(一体机/班牌,优先)与 Windows(M2)。协议契约见 docs 仓协议规范 v0.1。

## 目录

| 目录 | 内容 | 状态 |
|---|---|---|
| `android/` | Kotlin Agent(minSdk 24,Device Owner 激活) | 🚧 M0 |
| `windows/` | .NET 8 Worker Service | 🚧 M2 |

## M0 范围

激活 → MQTT 连接 → 心跳(60s)→ 执行 LOCK_SCREEN / SEND_MESSAGE / SET_VOLUME / REBOOT → 回执;前台服务常驻 + 开机自启 + msgId 幂等 + HMAC 验签。
