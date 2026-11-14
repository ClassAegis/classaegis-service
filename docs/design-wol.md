<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 局域网唤醒/发现(ability · wol)设计文档

| 项 | 值 |
|---|---|
| 所属进程 | ability.exe(编排/校验) + core.exe(网络执行) |
| 能力名 | wol |
| 状态 | v0.1 已实现,测试全绿 |
| 依据 | 协议规范 §6.7 |
| 更新 | 2026-11-08 |

## 1. 架构:网络代理模式

ability 进程红线不变(仅环回)。wol 能力做**校验与编排**,实际网络操作
经 core 的环回端点 `POST /netexec`(token 鉴权)执行——**出网组件仍然只
有 core**:

```
集控指令 → core(验签) → ability/wol(校验 MAC/CIDR)
  → 环回 POST /netexec → core/netops(UDP 广播 / ICMP 探测 / ARP 读取)
  → 结果原路返回 → 回执
```

all-in-one 调试模式(core+ability 同进程)直接调用 netops,不走环回。
净效果:审计点集中在 core,ability 无任何出网代码。

## 2. 三个操作(netops)

| op | 实现 | 边界 |
|---|---|---|
| wol | 魔术包 = 6×0xFF + 16×目标 MAC(102B),UDP 广播 255.255.255.255:port ×3 | MAC 四种格式归一化;端口缺省 9 |
| discover | CIDR 主机枚举(≤/20,上限 4096)→ IcmpSendEcho 并发探测(32 并发,300ms,免提权)→ GetIpNetTable 关联 IP↔MAC | cidr 缺省 = 本机 /24;仅 IPv4 |
| arp | GetIpNetTable 快照,仅 dynamic/static 条目 | — |

场景:教室在线同伴设备作为 **WOL 网关**,为同广播域内关机设备代发魔术包
(集控服务器可能不在目标子网);发现 = 局域网设备盘点,为唤醒提供 MAC 清单。

## 3. 指令集(3 条,协议 §6.7)

WOL_WAKE(mac/macs + port,本地预校验 MAC 后代理)、WOL_DISCOVER(cidr +
timeout_ms)、WOL_ARP。零新增协议错误码。

## 4. 安全边界

1. 指令经 core HMAC 验签后才到 ability(与全部指令同信任级别)
2. netexec 端点 token 鉴权 + 仅环回;op 白名单(wol/discover/arp)
3. 扫描规模硬上限:/20、4096 主机、单探测 300ms、并发 32
4. MAC 在 ability 侧本地预校验,垃圾输入不产生代理往返

## 5. 配置([wol] 段)

enabled(默认 true)/ default_port(默认 9,常见备选 7)。

## 6. 测试与已知限制

- netops 测试 5 个:魔术包字节(102B 结构)、MAC 四格式归一化、CIDR 枚举
  边界(/30 /24 /8 拒绝 / IPv6 拒绝)、本机 /24 推断、ARP/ICMP 真实冒烟
  (127.0.0.0/30);wol 测试 3 个:委托/校验/代理故障映射、空代理降级
- 已知限制:仅 IPv4;魔术包走 255.255.255.255 广播(部分路由器不跨段
  转发,定向广播 x.y.z.255 为后续增强);主机名反向解析未做;UDP 广播
  的发送确认仅指"已发出",目标是否真被唤醒需 WOL_DISCOVER 复查
