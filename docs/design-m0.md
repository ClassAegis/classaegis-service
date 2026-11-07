# 壹课管家(被控端)M0 设计文档

| 项 | 值 |
|---|---|
| 仓库 | oneclassmesh-agent(android/) |
| 依据 | docs 仓协议规范 v0.1 + 总览仓 PLAN.md §5 |
| 状态 | 设计评审中(代码暂停,评审通过后继续) |
| 更新 | 2026-11-07 |

## 0. 技术栈与架构形态

**架构形态:原生 Android 应用(Kotlin)**,不是 Electron / Qt / Flutter。
理由:被控端的全部核心能力——前台常驻服务、DevicePolicyManager(锁屏/重启)、悬浮窗覆盖层、MQTT 长连接——都是 Android 平台层能力,原生调用零隔阂;跨端框架(Electron 仅桌面端不存在于 Android;Flutter/RN 的服务与权限仍需原生插件)反而增加隔阂。

| 层 | 技术 | 版本 |
|---|---|---|
| 语言 | Kotlin(JVM 17 字节码) | 1.9.24 |
| 构建 | Gradle + Android Gradle Plugin | 8.9 / 8.5.2 |
| 目标 | minSdk 24(Android 7) / targetSdk 34 | 一体机主流 7~11 |
| MQTT | Eclipse Paho mqttv3(客户端自带重连线程) | 1.2.5 |
| JSON | Gson + 自研规范化序列化(签名用) | 2.11 |
| UI | 原生 Android View,程序化构建(无 XML 布局,M0 极简) | — |
| 常驻 | Foreground Service(API34 specialUse 类型)+ BOOT_COMPLETED 自启 | — |
| 存储 | SharedPreferences(凭证 + msgId 去重集) | — |
| 签名 | HMAC-SHA256(javax.crypto,与服务端 Jackson 规范串对齐) | — |

**Windows 被控端(M2)预览**:规划采用 .NET 8 Worker Service(系统服务形态、原生锁屏/会话 API、单文件发布),而非 Electron/Qt——两端各自原生,只共享协议规范。

## 0.1 进程与线程模型

- 单进程 `io.github.oneclassmesh.agent`
- MqttService 前台服务跑在主线程;Paho MqttAsyncClient 自带内部网络线程(连接/订阅均为异步)
- 心跳:java.util.Timer 独立线程,60s 周期发布 hb
- 指令到达:messageArrived 回调在 Paho 线程;流水线在该线程完成(去重/验签/执行);涉及 UI 的操作(悬浮窗、锁屏)切换主线程
- 回执发布:Paho 异步 publish,不阻塞流水线

## 1. 总体结构

```
┌──────────────────────────────────────────────────────┐
│ MainActivity(激活 UI + 状态日志)                       │
│   输入激活码+服务端地址 → HTTP 激活 → 存凭证 → 启动服务   │
├──────────────────────────────────────────────────────┤
│ MqttService(前台服务,常驻)                            │
│   连接 EMQX(username=SN, password=mqttToken)          │
│   订阅 cmd/down │ 心跳 60s │ LWT 遗嘱 │ 发布回执        │
├──────────────────────────────────────────────────────┤
│ CommandDispatcher(指令流水线)                          │
│   parse → msgId 去重(持久化) → HMAC 验签               │
│   → EXECUTING 回执 → execute → SUCCESS/FAILED 回执     │
├──────────────┬───────────────┬───────────────────────┤
│ MessageOverlay│ LockHelper     │ AudioManager / DPM    │
│ (消息覆盖层)  │ (锁屏/解锁)    │ (音量 / 重启)          │
├──────────────┴───────────────┴───────────────────────┤
│ Prefs(凭证存储) │ CanonicalJson + Hmac(协议层)         │
└──────────────────────────────────────────────────────┘
```

## 2. 模块职责

| 文件 | 职责 | 状态 |
|---|---|---|
| AndroidManifest.xml | 组件装配 + 权限声明 | TODO |
| MainActivity.kt | 激活码输入 → POST /api/v1/devices/activate → 存 Prefs → 启动 MqttService;状态日志区 | TODO |
| MqttService.kt | 前台服务;MQTT 连接/订阅/心跳/LWT/回执发布 | ✅ 已写(待接线修正) |
| CommandDispatcher.kt | 指令流水线:去重/验签/分发/回执 | ✅ 已写 |
| MessageOverlay.kt | SEND_MESSAGE 悬浮覆盖层(TTL 自动消失) | TODO |
| AdminReceiver.kt + device_admin.xml | 设备管理员(force-lock);DO 才有 reboot | ✅ 已写 |
| Prefs.kt | 凭证与配置存储 | ✅ 已写 |
| CanonicalJson.kt + Hmac.kt | 协议层:规范化 JSON + HMAC-SHA256 验签 | ✅ 协议规范 §5.1 |
| LockHelper.kt | lockNow / 解锁语义 | TODO(并入 Dispatcher 或独立) |

## 3. 关键流程

### 3.1 激活流程(M0 简化:暂无真实服务端)
M0 联调阶段无真实服务端激活接口,提供两种激活方式:
1. **手动录入模式**:MainActivity 提供开发者输入面板,直接填 schoolCode / SN / broker 地址 / mqttToken / deviceSecret(M0 联调用,等价于模拟激活)
2. 真实激活接口接入 = M0 后续任务(服务端就绪后)

### 3.2 指令流水线
```
cmd/down JSON
  → parse 失败:丢弃 + log
  → msgId 已见:静默丢弃(幂等)
  → HMAC 验签失败:回执 FAILED E_BAD_SIG,不执行
  → 回执 EXECUTING(证明已收到)
  → execute(type, payload)
  → 回执 SUCCESS / FAILED(error.code)
```

### 3.3 心跳与 LWT
- 心跳 60s,QoS0,hb topic,payload 含 sn/protocolVersion/agentVersion/metrics
- 连接时设置 LWT:`{"msgId":null,"status":"OFFLINE",...}` 到 lwt topic

### 3.4 开机自启
BOOT_COMPLETED → startForegroundService(MqttService)——manifest 声明 + receiver 类(待写)

## 4. 权限清单

| 权限/机制 | 用途 | 授予方式 |
|---|---|---|
| INTERNET / ACCESS_NETWORK_STATE | MQTT 连接 | 安装即有 |
| FOREGROUND_SERVICE + specialUse(API34) | 常驻服务 | 安装即有(需 manifest property 声明) |
| RECEIVE_BOOT_COMPLETED | 开机自启 | 安装即有 |
| SYSTEM_ALERT_WINDOW | 消息覆盖层 | 用户手动到设置授权(主页引导按钮) |
| POST_NOTIFICATIONS(API33+) | 前台通知 | 动态请求 |
| Device Admin(force-lock) | lockNow 锁屏 | MainActivity 引导激活设备管理员 |
| Device Owner | reboot | adb: dpm set-device-owner(仅首次刷机后可用) |

## 5. 存储(Prefs keys)

| key | 内容 |
|---|---|
| schoolCode / sn | 命名空间与身份 |
| deviceSecret | HMAC 验签密钥 |
| mqttToken | MQTT 密码 + HTTP Bearer |
| brokerHost / brokerPort | Broker 地址 |
| seenIds | msgId 去重集(上限 500) |

## 6. 异常与降级

| 场景 | 行为 |
|---|---|
| 非 Device Owner 下发 REBOOT | 回执 FAILED E_PLATFORM_UNSUPPORTED("requires device owner") |
| 设备管理员未激活时 LOCK_SCREEN | 回执 FAILED E_PLATFORM_UNSUPPORTED("device admin not enabled") |
| HMAC 验签失败 | 不执行,回执 FAILED E_BAD_SIG;服务端应告警 |
| 悬浮窗权限未授予时 SEND_MESSAGE | M0 降级:回执 FAILED,提示需悬浮窗权限 |
| 重复 msgId | 静默丢弃(幂等) |
| MQTT 断线 | paho 自动重连 + 心跳暂停,恢复后重订阅重发心跳 |

## 7. 已知待接线问题(已写代码)
- MqttService 中 `CommandDispatcher(this)` 需改为 `CommandDispatcher(this) { publishReceipt(it) }`(构造签名已含回执回调)
- MqttService.messageArrived 调 dispatcher.onCommand ✓(已对齐)

## 8. 联调计划(无服务端)
1. 本机 docker 起 EMQX(deploy/ 编排已有,需 Docker——未装)
2. Agent 手动录入模式连上 EMQX
3. EMQX Dashboard(Web)手动 publish cmd/down 测试指令
4. 观察:EXECUTING→SUCCESS 回执出现在 Dashboard 订阅窗口
