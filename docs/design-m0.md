# 壹课管家(被控端)M0 设计文档 — Go / Windows

| 项 | 值 |
|---|---|
| 仓库 | oneclassmesh-agent |
| 语言 | **Go 1.23**(静态编译,单文件,无运行时依赖) |
| 目标平台 | **Windows 10/11 x64(M0 唯一目标)**;Linux/UOS/麒麟为后续里程碑(platform 适配接口已预留) |
| 运行形态 | M0:控制台模式联调;M0 末:M1:Windows 服务(LOCAL SYSTEM) |
| 依据 | docs 仓协议规范 v0.1 |
| 状态 | 设计定稿,按此实现 |
| 更新 | 2026-11-07 |

## 0. 为什么是 Go

- 单代码库 → windows/amd64、linux/amd64、后续 linux/arm64、**linux/loong64(龙芯信创)**,静态编译单文件零依赖
- 长连接并发模型天然契合 MQTT 常驻
- Windows 服务集成(x/sys/windows/svc)与 D-Bus(未来 Linux)都有成熟库
- .NET 无官方 loong64,Java 被控端太重 —— 详见总览仓 PLAN §3

## 1. 目录结构

```
oneclassmesh-agent/
├─ cmd/agent/main.go            入口:console 联调模式 + Windows 服务模式 + install/uninstall
├─ cmd/ocmctl/main.go           联调工具:生成签名指令 JSON、可直发 MQTT
├─ internal/core/
│  ├─ config.go                 TOML 配置加载
│  ├─ protocol.go               消息解析/规范化 JSON/HMAC/去重
│  ├─ dispatcher.go             指令流水线
│  ├─ heartbeat.go              心跳 60s
│  └─ mqtt.go                   连接管理(订阅/回执/LWT/自动重连)
├─ internal/platform/
│  ├─ platform.go               适配接口(Lock/Unlock/Notify/Volume/Reboot)
│  └─ windows/impl.go           Windows 实现
├─ conf/config.example.toml     配置样例
├─ docs/design-m0.md            本文档
└─ README.md
```

## 2. 配置文件(conf/config.toml,M0 手动录入)

```toml
[device]
school_code = "demo"
sn = "WIN-001"
device_secret = "dev-secret-001"
mqtt_token = "dev-token-001"

[broker]
host = "127.0.0.1"
port = 1883

[agent]
heartbeat_seconds = 60
log_level = "info"
```

M0 手动录入模式(服务端激活接口 M0 后半段接入);真实激活 = 换 config 来源。

## 2.1 核心库清单

| 用途 | 库 | 版本 |
|---|---|---|
| MQTT | github.com/eclipse/paho.mqtt.golang | v1.5.0 |
| 配置 | github.com/BurntSushi/toml | v1.4.0 |
| Windows API | golang.org/x/sys/windows | latest |
| Toast 通知 | github.com/go-toast/toast | latest |
| 音量 | github.com/moutend/go-wca | latest |
| 服务化 | golang.org/x/sys/windows/svc | 同 x/sys |

## 3. 指令流水线(继承协议规范)

```
cmd/down JSON
→ parse 失败:丢弃+log
→ msgId 已见:静默丢弃(去重集持久化于 %ProgramData%\oneclassmesh\seen.json)
→ HMAC 验签失败:回执 FAILED E_BAD_SIG,不执行
→ 回执 EXECUTING
→ execute(type, payload)
→ 终态回执 SUCCESS / FAILED(error.code)
```

## 4. 五个指令的 Windows 实现

| type | 实现 | 依赖 | 失败模式 |
|---|---|---|---|
| LOCK_SCREEN | user32!LockWorkStation | syscall | 返回 false → E_FAILED |
| UNLOCK_SCREEN | M0 语义 = 关闭本 Agent 的通知/覆盖层(Windows 锁屏不可编程解锁,这是安全设计) | — | — |
| SEND_MESSAGE | Windows 10 Toast 通知(title/body/TTL) | go-toast/toast | 通知被系统策略禁用 → E_PLATFORM_UNSUPPORTED |
| SET_VOLUME | Core Audio ISimpleAudioVolume | moutend/go-wca | COM 失败 → E_FAILED |
| REBOOT | InitiateSystemShutdownEx(服务模式 SYSTEM 权限下可直接执行) | x/sys/windows | 无权限 → E_PLATFORM_UNSUPPORTED |

## 5. 关键风险:Session 0 隔离 ⚠️

Windows 服务运行在 Session 0,与用户桌面隔离:
- Toast 通知、锁屏等 UI 类指令在服务上下文中可能不可见/不可执行
- 规划方案:服务进程(通信+系统级指令)+ **用户态伴随进程**(登录后启动,负责 Toast/锁屏等桌面指令),两者经本地 named pipe 通信
- **M0 简化决策**:先以控制台模式(当前用户运行)联调全部指令,流水线与执行器已解耦,后续加服务化外壳不改核心;服务化+伴随进程列入 M0 末/M1

## 6. 联调计划(无服务端)

1. **EMQX Windows 免安装版**(zip 解压即用,免 Docker)起 Broker
2. agent 以 console 模式 + config.toml 手动凭证连接
3. `cmd/ocmctl` 生成签名指令并直发 MQTT(构造 msgId/type/payload/ts + HMAC 规范串)
4. 验证:锁屏生效/Toast 弹出 + Dashboard 订阅 cmd/up 观察回执 EXECUTING→SUCCESS

## 7. 与已废弃 Android 方案的关系

- android/ 目录与 Kotlin 代码已删除(git 历史保留)
- 协议/流水线/命名体系原样继承,执行器从 Android 平台层换为 Windows 平台层
- 协议规范 §6 指令类型表中 Android 实现要点将随 M0 联调修订为 Windows 实现

## 8. 里程碑映射

- M0:console 模式闭环(本文档)→ M0 末:Windows 服务安装模式 + ocmctl 完善
- M1:文件分发(FILE_PUSH)+ 真实激活接口对接 + 服务化+伴随进程
- M2:Linux/UOS/麒麟适配(platform/linux/)+ 信创验证 + 壹课中枢对接
```
