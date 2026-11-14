<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 垃圾清理/目录扫描(ability · clean)设计文档

| 项 | 值 |
|---|---|
| 所属进程 | ability.exe(业务能力进程) |
| 能力名 | clean |
| 状态 | v0.1 已实现,测试全绿 |
| 依据 | 协议规范 §6.4 |
| 更新 | 2026-11-08 |

## 1. 定位与边界

清理教室终端的垃圾文件释放磁盘。**只扫描类别声明的根目录**,绝不越界;
两段式流程供管理台确认:CLEAN_SCAN(只读统计)→ 管理员确认 → CLEAN_RUN
(重新扫描并当场删除)。

## 2. 垃圾类别(8 类)

| id | 名称 | 根目录 | 防护 |
|---|---|---|---|
| temp | 系统临时文件 | %TEMP% / %TMP% / %SystemRoot%\Temp | 最小年龄默认 24h;根目录去重(%TEMP%≈%TMP%) |
| recycle_bin | 回收站 | shell32 API(SHQueryRecycleBin/SHEmptyRecycleBin) | 按固定盘符聚合统计 |
| windows_update | Windows 更新缓存 | %SystemRoot%\SoftwareDistribution\Download | — |
| browser_cache | 浏览器缓存 | Chrome/Edge User Data\Default\Cache\Cache_Data | 占用文件删除失败计入 failed |
| wer | 错误报告 | %LOCALAPPDATA%\...\WER\ReportQueue/ReportArchive | — |
| d3dscache | DirectX 着色器缓存 | %LOCALAPPDATA%\D3DSCache | — |
| thumbcache | 缩略图缓存 | %LOCALAPPDATA%\...\Explorer\thumbcache_*.db | 文件名模式过滤 |
| windows_old | 旧系统安装 | %SystemDrive%\Windows.old | **默认禁用**(删了就无法回滚系统),需显式启用 |

环境变量在扫描时展开,不存在的根目录静默跳过(如未装 Chrome)。

## 3. 指令集(4 条,协议 §6.4)

CLEAN_SCAN(categories 子集可选,含未启用/未知类别→E_PARAM)、
CLEAN_RUN(重新扫描+删除,不引用历史结果——垃圾文件时刻在变,快照会失真)、
CLEAN_STOP(cancelled=true 终态)、CLEAN_STATUS(最近一次扫描/清理摘要)。
扫描与清理共用一个运行槽位:同时只允许一个 pass(E_SCAN_RUNNING)。

## 4. 回执与进度

- PROGRESS 节流(≥3% 或 ≥5s):`type=clean_progress, phase(scan|run),
  percent, files, bytes, deletedBytes`
- 终态 result:分类别统计(files/bytes/deletedFiles/deletedBytes/failedFiles)
  + 总计;scan 模式附 findings(按大小降序 ≤100 条)+ findingsTruncated
- 删除失败(文件占用)计入 failedFiles 不重试——下次清理自然覆盖

## 5. 安全边界

1. 只走类别根目录(构造上不可能越界)
2. 排除项前缀匹配(边界校验,与 av 同规则)
3. temp 类最小年龄门槛(默认 24h;0/省略=24h,-1=关闭门槛)
4. 删除仅 os.Remove(文件),不删目录、不删只读/系统属性外的特殊项
5. WalkDir 不跟随符号链接/junction

## 6. 配置([clean] 段)

enabled / min_age_hours / exclusions / enable_categories / disable_categories。
无种子规则文件(类别内建)。

## 7. 错误码

全部复用:E_PARAM / E_SCAN_RUNNING / E_NO_SCAN / E_UNSUPPORTED_TYPE /
E_FAILED。**本能力零新增协议错误码**。

## 8. 测试与已知限制

- 单元测试 8 个:环境变量展开(注入式,不改进程 env)、类别解析矩阵、
  年龄门槛扫描、删除+占用文件跳过、回收站扫描/清空流程(假 API)、
  参数错误、取消、状态回显
- 已知限制:浏览器缓存只覆盖 Chrome/Edge 主缓存目录(未含 Code Cache/GPUCache);
  Windows.old 删除大且慢(1h 环回窗口内一般足够);删除的文件不进回收站
  (直接删除);服务模式(SYSTEM)下 %LOCALAPPDATA% 指向
  C:\Windows\System32\config\systemprofile\AppData 需伴随进程配合(M1)
