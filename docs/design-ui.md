# 课佑管家桌面 UI

## 产品定位

课佑管家是常驻在 Windows 终端上的本地服务界面，展示设备状态、运行体检、功能入口和产品信息，并通过系统托盘保持后台运行。Core、Ability 和 Launcher 继续由 Go 实现，UI 不直接创建或终止这些后台进程。

## 技术栈

| 层 | 技术 | 职责 |
|---|---|---|
| 桌面外壳 | Tauri 2 / Rust | 单实例、窗口、托盘、日志文件访问和 NSIS 安装包 |
| 页面 | Vue 3 / JavaScript / Element Plus / Vite | 总览、功能预览、设置、关于页面 |
| 后台服务 | Go | Launcher 生命周期管理、Core 通信和 Ability 执行 |
| 本地桥接 | Tauri commands | 固定状态查询、日志枚举与读取、窗口控制 |

UI 通过 `127.0.0.1:39817` 访问 Core 状态接口，令牌只由桌面外壳读取配置并使用。渲染层不开放任意 shell、文件路径或进程控制能力。

## 页面结构

- 总览：设备信息、三项运行体检及常用功能入口。
- 功能预览：杀毒、清理、还原、弹窗拦截、软件管理和工具箱；业务操作尚待接入。
- 设置：外观、刷新频率、体检提示、设备编号隐藏、程序信息及恢复界面默认设置。
- 关于：产品名、版本、协议和配置路径。

当前无日志页面；原生日志查询命令仍保留。具体状态字段与后续接入边界见 [对接说明](ui-integration.md)。

窗口关闭时隐藏到托盘；托盘菜单可恢复窗口或退出 UI。UI 退出不会强制结束由 Launcher 管理的后台服务。

## 构建与安装

在 `cmd/ui` 执行 `npm run dev` 启动 Vite 开发页面，执行 `npm run tauri:dev` 启动桌面开发版，执行 `npm run dist` 构建 NSIS 安装包。安装包由 `build/stage-service.ps1` 构建并校验以下组件：

- `ClassAegisCore.exe`
- `ClassAegisAbility.exe`
- `ClassAegisLauncher.exe`
- `ClassAegisCtl.exe`
- `ClassAegisDebug.exe`
- `conf/config.example.toml`

安装程序使用 Tauri 默认 NSIS 配置。

Windows 构建环境需要 Go、Rust stable、MSVC Build Tools、NSIS 和 WebView2。Tauri 使用系统 WebView2 运行 UI，安装器按配置引导 WebView2 bootstrapper。

## 当前范围

系统托盘和服务状态桥接已接入。多显示器卡片与系统通知仍需先确定事件来源、敏感字段过滤、显示器选择和通知过期策略，再接入页面。
