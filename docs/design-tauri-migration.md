<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 课佑管家 Tauri 迁移方案

## 结论

迁移目标是把 `cmd/ui` 的桌面外壳从 Electron 替换为 Tauri，同时保留现有 Vue 页面、Core/Ability/Launcher、环回 API 和完整安装包结构。当前发布链已切换为 Tauri，Electron 仅作为历史决策背景保留在文档中。

## WebView2 进程说明

Windows Tauri 使用系统 WebView2。任务管理器中可能看到多个 `msedgewebview2.exe`，这是 WebView2 的浏览器、渲染、GPU 和辅助进程模型，并不代表每个页面都启动了一个完整应用。进程数量会随窗口、页面和硬件加速状态变化。

迁移到 Tauri 的主要收益是安装包和常驻内存通常更小，代价是依赖系统 WebView2、引入 Rust 构建链，并需要把原先的桌面桥接改为 Tauri command。

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

## 实施状态

- `cmd/ui/src-tauri` 已建立 Tauri 2 外壳配置、Windows NSIS 目标和资源映射。
- Vue 入口通过 Tauri bridge 暴露固定的 `window.ocm` 接口。
- Tauri 外壳实现单实例、托盘显示、关闭隐藏、窗口最小化、日志目录与日志只读命令。
- Tauri 状态命令只访问 `127.0.0.1:39817/status`，并从本机配置读取 `ipc_token`；不直接管理或终止 Core/Ability/Launcher。
- `npm run dist` 默认构建 Tauri 安装程序；构建前会生成并校验 Core、Ability、Launcher、Ctl、Debug 和配置样例。
- 安装程序使用 Tauri 默认 NSIS 配置，并通过安装器引导 WebView2。
- 多显示器卡片和系统通知暂不接入。接入前需要定义事件来源、敏感字段过滤、显示器选择和过期策略，避免前端任意文本直接弹到用户屏幕。

Tauri 前端、Rust 外壳和 NSIS 安装包已构建成功，安装包包含完整 Service 组件。当前测试覆盖 Go 服务包和 Tauri 安装器构建；多显示器卡片与通知仍待后续实施。

