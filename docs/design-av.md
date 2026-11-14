<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 本地杀毒引擎(ability · av)设计文档

| 项 | 值 |
|---|---|
| 所属进程 | ability.exe(业务能力进程) |
| 能力名 | av |
| 状态 | v0.1 已实现,测试全绿 |
| 依据 | 协议规范 §5.2 回执(PROGRESS)、§6 指令类型表 |
| 更新 | 2026-11-08 |

## 1. 定位与边界

课佑管家业务能力进程(ability.exe)是能力宿主,本地杀毒引擎(av)是第一个
独立能力模块。红线继承:能力进程**无任何出网代码**;签名库更新走本地文件
(网络推送在 M1 经 FILE_PUSH 落盘后由 AV_DB_UPDATE 加载)。

## 2. 指令集(8 条)

| type | payload | 行为 | 终态回执 result |
|---|---|---|---|
| AV_SCAN | `mode`(quick/full/custom)、`paths`(custom 必填)、`on_detect`(report/quarantine,缺省取配置) | 同步扫描,PROGRESS 节流上报(≥3% 或 ≥5s) | 扫描汇总(见 §4) |
| AV_STOP_SCAN | — | 取消当前扫描;扫描任务回 SUCCESS + cancelled=true | `{stopped:true}` |
| AV_STATUS | — | — | scanning/scan/db/quarantine 快照 |
| AV_DB_INFO | — | — | 签名库摘要 |
| AV_DB_UPDATE | `path`(设备本地签名库文件) | 校验→内存替换→落盘(临时文件+改名) | 签名库摘要 |
| AV_QUARANTINE_LIST | — | — | `{items,count}` |
| AV_QUARANTINE_RESTORE | `id` | 恢复原位(目标已存在→E_TARGET_EXISTS) | `{id,restored}` |
| AV_QUARANTINE_DELETE | `id` | 永久删除 | `{deleted:id}` |

## 3. 检测模型(v0.1)

1. **哈希规则**:文件流式 SHA-256,精确匹配签名库(SHA256 黑名单)
2. **特征码规则**:≤1MB 文件做字节子串匹配(签名库存 hex 编码字节串)
3. **启发式**:双扩展名可执行文件(如 `invoice.pdf.exe`)——大小写不敏感,
   可配置关闭(enable_heuristics)
4. **大小上限**:默认 512MB 以上跳过(max_file_size_mb)
5. **排除项**:配置前缀排除(边界匹配,C:\Windows 不匹配 C:\WindowsXYZ);
   引擎自保护默认排除 %ProgramData%\oneclassmesh 与隔离区自身

快速扫描 = %TEMP% + 下载 + 桌面;全盘扫描 = 全部固定盘符;自定义 = payload 路径。
并发:哈希线程池(默认 min(CPU,8)),walker/hashers 流水线,context 取消贯穿。

## 4. 回执与进度

- 执行管道四元组(ok/code/message/result)贯通 core→ability,终态回执
  result 携带扫描汇总:`scanId/mode/roots/cancelled/scannedFiles/scannedBytes/
  threatsTotal/threats[](上限 100 条)/findingsTruncated/skippedLarge/
  skippedErrors/durationMs`
- 进度:ability → core `POST /event`(令牌鉴权) → core 以 PROGRESS 状态发布
  cmd/up(协议 §5.2 原生支持),payload:`type=av_scan_progress, percent,
  processed, totalSeen, threats`
- 被停止的扫描回 SUCCESS + cancelled=true(管理员主动停止非失败)

## 5. 隔离区

- 目录:默认 `%ProgramData%\oneclassmesh\av\quarantine`(0700)
- 检出文件移入并改名 `<id>.quar`,旁挂 `<id>.json` 元数据(原路径/SHA256/
  规则/严重度/时间/大小);跨卷移动自动回退复制+删除
- 恢复:按元数据回原路径,父目录自动重建,目标已存在→`E_TARGET_EXISTS`
- id 服务端可携带,严格校验 `[0-9A-Za-z-]` 防路径穿越

## 6. 签名库

- JSON 格式(conf/av-signatures.example.json 为种子样例):
  `version/updated/source/hashes[]/patterns[]`,坏规则丢弃并告警,不整体拒绝
- AV_DB_UPDATE:整库校验通过后原子替换(临时文件+rename),失败保留旧库
- 种子库含 EICAR 测试哈希规则 + OCM-Signature-Test-Pattern 测试特征码,
  category 均为 test,便于联调自检(测试中 EICAR 以运行时拼接构造,完整
  字节串不进源码/二进制)

## 7. 错误码(本能力新增,agent 侧)

E_PARAM / E_SCAN_RUNNING / E_NO_SCAN / E_DB_FORMAT / E_QUAR_NOT_FOUND /
E_TARGET_EXISTS;传输层沿用 E_ABILITY_OFFLINE(能力进程不可达)。

## 8. 测试与实测

- 单元测试 9 个:扫描检出+隔离+恢复+删除全链路(EICAR)、特征码规则、
  启发式、排除项、取消、签名库校验、DB 更新(含坏库拒绝+旧库保留)、
  并发扫描互斥、种子库自检 —— `go test ./internal/ability/av/` 全绿
- 已知限制:v0.1 哈希/特征码/简单启发式,非完整 AV;仿真/脱壳/宏检测、
  实时防护、云查杀均不在范围
