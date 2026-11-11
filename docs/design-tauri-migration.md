# 课佑管家 Tauri 迁移方案

## 结论

迁移目标是把 `cmd/ui` 的桌面外壳从 Electron 替换为 Tauri，同时保留现有 Vue 页面、Core/Ability/Launcher、环回 API 和完整安装包结构。
迁移采用可回退方案：Electron 版本继续作为基线，Tauri 版本先以独立构建产物验证，指标达标后再切换默认安装器。

## WebView2 进程说明

Windows Tauri 使用系统 WebView2。任务管理器中可能看到多个 `msedgewebview2.exe`，这是 WebView2 的浏览器、渲染、GPU 和辅助进程模型，并不代表每个页面都启动了一个完整应用。进程数量会随窗口、页面和硬件加速状态变化。

Electron 也使用 Chromium 多进程，只是 Chromium 随应用分发，任务管理器通常显示为 `ClassAegisService.exe` 及其子进程。迁移到 Tauri 的主要收益是安装包和常驻内存通常更小，代价是依赖系统 WebView2、引入 Rust 构建链，并需要重写 Electron 主进程能力。

## 目标架构

```text
ClassAegisService.exe (Tauri UI)
        |
        | 127.0.0.1:39817 + X-Ocm-Token
        v
ClassAegisLauncher.exe
   |-- ClassAegisCore.exe
   |-- ClassAegisAbility.exe
   `-- logs/config
```

Tauri 只负责窗口、托盘、单实例、日志目录打开、启动参数和 UI 与 Core 的安全桥接。Core、Ability 和 Launcher 不迁移到 Rust，也不复制业务逻辑。

## 分阶段实施

### 阶段 0：建立基线

- 记录 Electron 安装包大小。
- 记录空闲启动后 30 秒的进程树、工作集和启动耗时。
- 验证安装到盘符根目录时自动使用 `ClassAegisService` 子目录。
- 验证安装目录包含 `bin/ClassAegisCore.exe`、`ClassAegisAbility.exe`、`ClassAegisLauncher.exe`、`ClassAegisCtl.exe`、`ClassAegisDebug.exe` 和 `conf`。

### 阶段 1：Tauri 壳原型

- 安装 MSVC、Rust stable、`cargo` 和 Windows WebView2 Runtime 检查工具。
- 在 `cmd/ui-tauri` 创建 Tauri 2 项目，复用现有 `src`、Vite 配置和静态资源。
- 实现无边框窗口、拖动区域、最小化、隐藏到托盘、退出、单实例。
- 用 Tauri command 代替 Electron preload，只暴露固定的 `getStatus`、日志列表、日志读取和打开日志目录接口。

### 阶段 2：Service 集成

- 继续由 `ClassAegisLauncher.exe` 管理 Core 和 Ability。
- Tauri UI 只连接 `127.0.0.1:39817`，不直接启动业务进程。
- 将 `bin`、`conf` 作为 Tauri 安装器的额外资源，保持现有目录结构。
- 保留 Electron 构建脚本，增加 `dist:tauri`，两套产物使用相同版本号和组件清单。

### 阶段 3：行为对齐与验收

- 页面：首页状态、安全日志、关于页显示一致。
- 窗口：拖动、最小化、关闭隐藏、托盘恢复、真正退出一致。
- 安全：Tauri 只允许白名单命令；不开放任意 shell、文件路径或 Node 能力。
- 安装：自定义安装目录、盘符根目录补 `ClassAegisService`、完整 Service payload。
- 兼容：Windows 10/11，WebView2 缺失时给出明确安装提示或由安装器引导安装。

## 建议验收门槛

在同一台机器、同一配置下比较 Electron 与 Tauri：

- 安装包体积至少减少 40%。
- UI 空闲内存降低 30% 以上，或至少不高于 Electron。
- 首次显示时间不超过 Electron 的 1.3 倍。
- 任务管理器中的 WebView2 进程数量记录为可接受范围，不以“必须只有一个进程”为要求。
- Core/Ability 的启动、重启、日志和配置行为零回归。

任一核心行为回归，默认继续发布 Electron 版本，Tauri 只作为实验构建。

## 当前阻塞项

本机当前找不到 `rustc`、`cargo` 和 `rustup`。安装 Rust 工具链并准备 Windows 编译环境后，才能执行阶段 1；在此之前不应把 Tauri 产物宣称为可安装版本。

## 已落地的第一批代码

- `cmd/ui/src-tauri` 已建立 Tauri 2 外壳配置、Windows NSIS 目标和资源映射。
- Vue 入口增加 Electron/Tauri 兼容桥，页面继续使用原有 `window.ocm` 接口。
- Tauri 外壳实现单实例、托盘显示、关闭隐藏、窗口最小化、日志目录与日志只读命令。
- Tauri 状态命令只访问 `127.0.0.1:39817/status`，并从本机配置读取 `ipc_token`；不直接管理或终止 Core/Ability/Launcher。
- 多显示器卡片和系统通知暂不接入。接入前需要定义事件来源、敏感字段过滤、显示器选择和过期策略，避免前端任意文本直接弹到用户屏幕。

前端 Tauri bundle 已在隔离的 `cmd/ui/dist-tauri` 目录构建成功；Rust 构建仍待安装工具链后验证。

