# OneClassMesh 客户端 · 壹课管家(oneclassmesh-agent)

被控端 Agent:**Go 编写,Windows 10/11 优先**(Linux/UOS/麒麟为后续里程碑,platform 适配接口已预留)。协议契约见 docs 仓协议规范 v0.1。

## 目录

| 目录 | 内容 |
|---|---|
| `cmd/agent/` | 主程序:console 联调模式 / Windows 服务模式 + install/uninstall |
| `cmd/ocmctl/` | 联调工具:生成签名指令 JSON,可直发 MQTT |
| `internal/core/` | 协议层(规范化 JSON/HMAC/去重)+ 流水线 + 心跳 + 连接管理(平台无关) |
| `internal/platform/` | 平台适配接口 + `windows/` 实现(Linux/UOS/麒麟后续) |
| `conf/` | 配置样例(M0 手动录入凭证) |
| `docs/` | 设计文档 |

## M0 范围

console 模式联调:MQTT 连接 → 心跳 60s → 五指令(锁屏/解锁/消息/音量/重启)→ 回执;msgId 幂等 + HMAC 验签;M0 末完成 Windows 服务化。

## 开发

- Go 1.23;构建:`go build ./cmd/agent`
- 联调:`cmd/ocmctl` + EMQX Windows 免安装版
