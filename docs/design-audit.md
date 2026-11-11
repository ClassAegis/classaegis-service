# 上网行为审计(ability · audit)设计文档

| 项 | 值 |
|---|---|
| 所属进程 | ability.exe(业务能力进程) |
| 能力名 | audit |
| 状态 | v0.1 已实现,测试全绿(含 ESE 端到端) |
| 依据 | 协议规范 §6.6 |
| 更新 | 2026-11-08 |

## 1. 定位与边界

读取本机浏览器历史/下载/缓存记录,供集控侧合规审计。设备端不落盘、
不外发记录副本;临时副本即用即删。**合规提示:审计数据仅限集控侧授权
查询,面向校园终端管理场景。**

## 2. 数据源(两类)

| 源 | 库 | 读取方式 |
|---|---|---|
| ese | %LOCALAPPDATA%\Microsoft\Windows\WebCache\WebCacheV01.dat(ESE,IE/旧版 Edge) | esent.dll 绑定:复制 .dat/.log/.jrs/.chk 副本 → 软恢复(JetInit) → 只读打开 → 遍历 History / Downloads / CacheEntries 表 |
| edge / chrome | %LOCALAPPDATA%\{Microsoft\Edge,Google\Chrome}\User Data\<profile>\History(SQLite) | 复制后用**自研只读 SQLite 扫描器**解析(无 cgo/无 SQL 引擎):b-tree 遍历 + CREATE TABLE 列名解析 + 记录解码 + 溢出页链重组 |

说明:现代 Edge/Chrome 的历史/URL 主存储是 **SQLite 而非 ESE**;ESE 源
覆盖 IE/旧版 Edge 与缓存条目。WAL 未合并(副本只取主文件,最新记录可能
缺失几分钟);Chromium 缓存为块文件格式(非 SQLite),不在 v0.1 范围。

## 3. 指令集(4 条,协议 §6.6)

AUDIT_SOURCES(源枚举)、AUDIT_BROWSE(历史:url/title/lastVisit/visits)、
AUDIT_DOWNLOADS(下载:url/path/time/size)、AUDIT_CACHE(仅 ese 源)。
payload:`source`(ese|chromium|edge|chrome|all,缺省 all)+`limit`(缺省
200,上限 1000)。**逐数据源独立报错**:单源损坏计入 result.errors,不失败
整体。时间统一 RFC3339(WebKit 微秒时间戳、OLE 日期、FILETIME 均换算)。

## 4. 实现要点

- callProc 类型安全包装:指针参数在 Call 表达式内转换 + runtime.KeepAlive,
  杜绝 GC 生存期竞态(修过 0xc0000005 崩溃)
- JET_api 全参数表调用:JetCreateDatabaseW pdbid 不可为 NULL、
  JetAddColumn 7 参(含 pcolumnid)、JetUpdate 5 参(含 pcbActual)
- JET_paramBaseName ≤3 字符;ESE 临时日志 <base>tmp.log 必须从日志 base
  推导中排除
- 列名传 W 版 API 必须宽字符;ANSI 名仅用于 JetOpenTable/JetAddColumn 等
  A 版导出
- SQLite:INTEGER PRIMARY KEY 列在记录中存 NULL(值即 rowid),列名映射
  必须按 CREATE TABLE 全列序,不能按查询子集

## 5. 错误码

零新增:逐源错误以 result.errors 返回;命令级错误复用 E_PARAM /
E_UNSUPPORTED_TYPE / E_FAILED。

## 6. 配置([audit] 段)

enabled(默认 true)/ max_records(单源上限,默认 200,最大 1000)。

## 7. 测试与已知限制

- 单元测试 7 个:SQLite fixture(python 生成,含 9KB 溢出页 URL 重组、
  schema 列映射、时间转换)、OLE/WebKit 时间换算、UTF-16 解码、payload
  解析,以及 **ESE 端到端**(esent.dll 自建库→插入→副本→恢复→只读读回,
  验证整条绑定链路)
- 已知限制:Firefox(places.sqlite)未接入(同 SQLite 格式,任务清单);
  WAL 合并、Chromium 块文件缓存、加密容器(VaultSystem)均不在 v0.1
