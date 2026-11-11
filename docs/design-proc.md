# 进程加速/逐进程流量统计/限速(ability · proc)设计文档

| 项 | 值 |
|---|---|
| 所属进程 | ability.exe(业务能力进程) |
| 能力名 | proc |
| 状态 | v0.1 已实现(限速为实验性),测试全绿 |
| 依据 | 协议规范 §6.5 |
| 更新 | 2026-11-08 |

## 1. 定位与边界

「加速球」式进程加速 + 管理台可见的逐进程流量。**不做远程杀进程**(需要
白名单与二次确认设计,后续单独评估);限速基于 Windows 策略式 QoS,
标注实验性,需管理员权限。

## 2. 进程加速(PROC_SPEEDUP)

- 修剪工作集:psapi EmptyWorkingSet,前后 GetProcessMemoryInfo 差值 = 释放量
- 范围:默认全部用户进程;payload processes 子集过滤
- 硬跳过:系统关键进程(csrss/lsass/dwm/svchost/System/Idle/MemCompression 等)
  + 课佑管家自身进程族;explorer 不跳过(修剪无害)
- 可选优先级调整:priorities[{name, priority}],支持
  idle/below_normal/normal/above_normal/high(realtime 拒绝)
- 释放量是瞬时物理内存口径(进程按需重新加载页面,数值会回升——如实呈现)

## 3. 逐进程流量统计(PROC_LIST)

- 后台轮询(默认 5s,[1000,60000] 可配):GetExtendedTcpTable
  (TCP_TABLE_OWNER_PID_ALL, v4+v6) 取 ESTAB 连接归属 PID
- GetPerTcp6ConnectionEStats/GetPerTcpConnectionEStats 读
  TCP_ESTATS_DATA_ROD_v0 前两个 u64(DataBytesIn/DataBytesOut)
- 按 PID 归并差分:累计收发 + 区间速率;计数器回退(PID 复用)钳零;
  5 个周期不可见的 PID 淡出
- **UDP 无 per-socket 计数,不统计**(如 limitation);estats 不可用的行跳过
- PROC_LIST 进程名由 toolhelp 快照实时联查(不缓存,免 PID 复用错配)

## 4. 限速(PROC_THROTTLE,实验性)

- Windows 策略式 QoS:HKLM\SOFTWARE\Policies\Microsoft\Windows\QoS\ocm-<app>
  (Version/Throttle Rate/App Name/Protocol/Direction/... 注册表值)
- psched 内核执行,仅约束匹配应用出站流量;rateKBps=0 撤销策略
- **需管理员权限**:非提权进程直接 E_PLATFORM_UNSUPPORTED(注册表 HKLM 写保护)
- 实验性说明:该注册表布局无公开稳定契约,格式不合法时 psched 忽略策略
  (最坏情况=不限速,无破坏性);正式方案 M1 服务模式下再评估

## 5. 错误码

复用:E_PARAM / E_FAILED / E_PLATFORM_UNSUPPORTED(限速需提权)。
**零新增协议错误码**。

## 6. 配置([proc] 段)

enabled(默认 true)/ poll_interval_ms(默认 5000)。无种子数据文件。

## 7. 测试与已知限制

- 单元测试 11 个:流量差分(首见播种/速率/新 PID/淡出/负差钳零)、
  优先级映射(realtime 拒绝)、QoS 策略名消毒、保护名单、exe 名校验、
  非提权拒绝、QoS 注册表读写/撤销(HKCU 注入)
- 已知限制:UDP 不计;estats 若被策略禁用则流量显示为 0;
  QoS 限速需提权+实验性;PROT_LIST 顶部 100 行截断
