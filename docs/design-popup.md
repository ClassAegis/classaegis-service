<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 弹窗/广告拦截(ability · popup)设计文档

| 项 | 值 |
|---|---|
| 所属进程 | ability.exe(业务能力进程) |
| 能力名 | popup |
| 状态 | v0.1 已实现,测试全绿 |
| 依据 | 协议规范 §6.3、§9 |
| 更新 | 2026-11-08 |

## 1. 定位与边界

拦截教室大屏/一体机上安装软件的广告弹窗。采用**窗口级拦截**(火绒式思路):
不装驱动、不注入进程,周期枚举顶层可见窗口并按规则处置,课堂软件正常窗口
不受影响。浏览器/hosts 域名级广告拦截为后续扩展(见任务清单)。

## 2. 工作模型

```
watcher(默认 1s 周期,可配 200ms~60s)
→ EnumWindows 取可见顶层窗口
→ 标题/类名/PID→进程名解析
→ 硬跳过:无法识别进程 / 系统关键进程 / 课佑管家自身进程
→ 白名单命中 → 跳过
→ 拦截规则命中(进程名 + 标题通配 + 类名,三维至少一维)
→ 处置:WM_CLOSE 优雅关闭 或 SW_HIDE 隐藏(规则可指定,缺省 close)
→ 事件写入本地环形缓冲(默认 500 条)
```

- 冷却:同一 hwnd 10s 内不重复处置(顽固弹窗防事件风暴)
- 未命中处置失败的窗口不会重复记录(冷却兜底)

## 3. 指令集(5 条,协议 §6.3)

| type | payload | 行为 |
|---|---|---|
| POPUP_STATUS | — | watcher 状态/规则版本/统计 |
| POPUP_SET_ENABLED | {enabled} | 运行时启停 watcher(规则保留,懒启动) |
| POPUP_RULES_UPDATE | {path} 或 {rules} | 校验→内存替换→原子落盘;坏规则丢弃告警,坏文件整体拒绝(E_RULES_FORMAT)保留旧规则 |
| POPUP_GET_EVENTS | {clear} | 拉取事件(最旧优先,单次≤100 条,hasMore 翻页;clear 移除已返回) |
| POPUP_TEST_MATCH | {process,title,class} | 干跑匹配当前规则(联调排障) |

## 4. 云端协同识别闭环

```
设备端拦截 → 事件缓冲
→ 集控 POPUP_GET_EVENTS 定期拉取(多校聚合到云端)
→ 云端统计/人工审核 → 生成新规则
→ POPUP_RULES_UPDATE 下发(内联 rules 直接走 MQTT;大库走 FILE_PUSH+path)
→ 设备秒级生效
```

v0.1 只上报**已拦截**事件,不上报疑似候选(隐私与噪声考量,候选上报需
单独协议与合规评估)。规则集 JSON:`{version, updated, source, block[],
whitelist[]}`,单条 `{process, title, class, action, note}`。

## 5. 安全边界(硬约束)

1. 进程名无法解析的窗口**一律不动**
2. 系统关键进程硬跳过(csrss/winlogon/dwm/svchost/资源管理器壳等)
3. 课佑管家自身进程族硬跳过(launcher/core/ability/agent/ocmctl/UI)
4. 白名单优先于一切拦截规则
5. 规则来源是 HMAC 验签后的服务端指令,与指令通道同一信任级别

## 6. 配置(conf/config.toml [popup] 段)

enabled(默认 true)/ rules_path(默认 %ProgramData%\oneclassmesh\popup\rules.json)
/ interval_ms(默认 1000)/ action(默认 close)/ max_events(默认 500)。
种子规则样例:conf/popup-rules.example.json(含联调用测试规则)。

## 7. 错误码(本能力新增,agent 侧)

E_RULES_FORMAT;其余复用 E_PARAM / E_UNSUPPORTED_TYPE / E_FAILED。

## 8. 测试与已知限制

- 单元测试 9 个(假窗口源注入):通配匹配、拦截+事件记录、冷却去重、
  安全跳过(未识别/系统/自身/白名单)、事件拉取+清空、规则更新
  (内联+文件+坏文件拒绝保旧库)、启停与参数校验、规则集解析校验
- 已知限制:v0.1 不含进程启动阻止(IFEO)、浏览器广告/hosts 拦截、
  UWP/cloaked 窗口特殊处理;提升权限进程的 WM_CLOSE 可能静默失败
