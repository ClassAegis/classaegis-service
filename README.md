<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# ClassAegisService · 课佑管家

被控端程序：**Go 编写，Windows 10/11 优先**(Linux/UOS/麒麟为后续里程碑，platform 适配接口已预留)。协议契约见 docs 仓协议规范 v0.1。

## 架构(三进程 + 启动器)

安装包中的程序统一使用品牌名；源码目录仍按职责命名：

| 安装程序 | 作用 |
|---|---|
| `ClassAegisService.exe` | Tauri 桌面服务界面 |
| `ClassAegisLauncher.exe` | 启动器与守护进程 |
| `ClassAegisCore.exe` | 核心通信与安全验签 |
| `ClassAegisAbility.exe` | 本地业务能力宿主 |
| `ClassAegisCtl.exe` | 联调命令行工具 |
| `ClassAegisDebug.exe` | all-in-one 调试模式 |

| 进程 | 内容 |
|---|---|
| `cmd/launcher/` | 启动器/守护者:单实例锁、依序拉起 ability + core、崩溃指数退避重启 |
| `cmd/core/` | 核心进程：**唯一出网组件**(MQTT 收发/HMAC 验签/msgId 去重/心跳/回执) |
| `cmd/ability/` | 业务能力进程：**能力宿主**，注册并执行各能力模块(仅环回通信) |
| `cmd/agent/` | all-in-one 调试模式(core + ability 单进程，联调备用) |
| `cmd/ocmctl/` | 联调工具：签名发令 + 监听回执 |

能力模块(ability)：

| 模块 | 指令集 | 说明 |
|---|---|---|
| `desktop`(内置) | LOCK_SCREEN / UNLOCK_SCREEN / SEND_MESSAGE / SET_VOLUME / REBOOT | 五指令经 platform 适配层执行 |
| `av` 本地杀毒引擎 | AV_SCAN / AV_STOP_SCAN / AV_STATUS / AV_DB_INFO / AV_DB_UPDATE / AV_QUARANTINE_LIST / AV_QUARANTINE_RESTORE / AV_QUARANTINE_DELETE | 哈希+特征码扫描、双扩展名启发式、隔离区、PROGRESS 进度回执;详见 docs/design-av.md |
| `popup` 弹窗/广告拦截 | POPUP_STATUS / POPUP_SET_ENABLED / POPUP_RULES_UPDATE / POPUP_GET_EVENTS / POPUP_TEST_MATCH | 窗口级弹窗拦截(进程+标题通配+类名规则)、事件缓冲云端协同识别;详见 docs/design-popup.md |
| `clean` 垃圾清理 | CLEAN_SCAN / CLEAN_RUN / CLEAN_STOP / CLEAN_STATUS | 分类目录扫描(临时/回收站/更新缓存/浏览器缓存等 8 类)、两段式确认清理、PROGRESS 进度回执;详见 docs/design-clean.md |
| `proc` 进程加速/流量 | PROC_SPEEDUP / PROC_LIST / PROC_PRIORITY / PROC_THROTTLE | 工作集修剪加速、TCP estats 逐进程流量统计、策略式 QoS 限速(实验性,需提权);详见 docs/design-proc.md |
| `audit` 上网行为审计 | AUDIT_SOURCES / AUDIT_BROWSE / AUDIT_DOWNLOADS / AUDIT_CACHE | 浏览器历史/下载/缓存读取:ESE(WebCache,esent.dll 副本恢复) + Chromium SQLite(自研只读解析器);详见 docs/design-audit.md |
| `wol` 局域网唤醒/发现 | WOL_WAKE / WOL_DISCOVER / WOL_ARP | WOL 魔术包 + ICMP/ARP 子网盘点;网络操作经 core /netexec 代理执行(ability 保持环回隔离);详见 docs/design-wol.md |

## 目录

| 目录 | 内容 |
|---|---|
| `internal/core/` | 协议层(规范化 JSON/HMAC/去重) + 流水线 + 心跳 + 连接管理 + core 环回服务(/status /event) |
| `internal/ability/` | 能力宿主(注册表 + 环回 /execute 服务) + desktop 内置能力 |
| `internal/ability/av/` | 本地杀毒引擎(扫描器/签名库/隔离区) |
| `internal/ability/popup/` | 弹窗拦截(规则引擎/watcher/窗口源/事件缓冲) |
| `internal/ability/clean/` | 垃圾清理(类别定义/目录扫描器/回收站 shell API) |
| `internal/ability/proc/` | 进程加速/流量(工作集修剪/estats 采样/策略 QoS) |
| `internal/ability/audit/` | 上网审计(只读 SQLite 扫描器/ESE esent.dll 绑定) |
| `internal/core/netops/` | core 网络代理操作(WOL/ICMP 探测/ARP,被 /netexec 委托) |
| `internal/ipc/` | 本地 API 契约(127.0.0.1 + token + 指令超时表) |
| `internal/platform/` | 平台适配接口 + `windows/` 实现(Linux/UOS/麒麟后续) |
| `conf/` | 配置样例 + 签名库/拦截规则种子样例 |
| `docs/` | 设计文档 |

## 开发

- Go 1.23；构建:`go build ./...`;测试:`go test ./...`
- 联调:`cmd/ocmctl` + EMQX Windows 免安装版
- Service 安装包:`cmd/ui` 下执行 `npm run dist`，使用 Tauri NSIS 安装向导。安装包会同时包含 `bin` 下的 Core、Ability、Launcher、Ctl、Debug 和 `conf` 配置样例。


## 许可证与社区规则

本项目原创代码与文档采用 [AGPL-3.0-only](LICENSE)。第三方材料保留原许可。参见 [安全策略](SECURITY.md)、[贡献指南](CONTRIBUTING.md) 和 [行为准则](CODE_OF_CONDUCT.md)。
