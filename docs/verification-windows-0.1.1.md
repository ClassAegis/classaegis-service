<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# Windows 安装版启动修复与验证（0.1.1）

2026-11-12 在 `D:\ClassAegisService` 的真实安装版复现了 `ERR_FILE_NOT_FOUND`。
窗口实际访问 `file:///C:/Users/Administrator/AppData/Local/Temp/classaegis-ui-...`。

## 原因与修复

打包脚本将 Windows 临时目录的绝对路径覆盖到 `frontendDist`。Tauri 配置先匹配 URL，
导致未嵌入前端资源，清理临时目录后安装版无法加载页面。

现在 Vite 输出到 `cmd/ui/dist-tauri`，Tauri 固定使用相对目录 `../dist-tauri`，
打包完成后仍会清理输出目录。发布构建检查会拒绝 URL、绝对路径和缺失的 `index.html`。
实际验证过原错误配置会被该检查拦截。

同时修复 Vue 模板中的窗口按钮绑定及最小化后的恢复逻辑；移除未确认后端运行时的
“防护已开启”和 Launcher 正常状态。未获取到配置或 Core 状态时，页面保留离线提示。

## 验证方法

```powershell
npm run tauri:build --prefix cmd/ui
# 安装生成的 NSIS 安装包，启动安装后的程序，再运行：
powershell -NoProfile -ExecutionPolicy Bypass -File cmd/ui/build/test-installed.ps1 -InstallRoot D:\ClassAegisService

go test ./...
go vet ./...
```

0.1.1 完成全套 Go 测试和 vet、前端及 Rust 发布构建、NSIS 安装，
并在前端构建目录已删除的条件下，通过以下安装版检查：

- 五个 Service 组件及配置模板存在。
- 首页、日志页、关于页的实际页面文本及安装版本号正确。
- 自绘关闭按钮隐藏主窗口，原进程保持运行；再次启动恢复同一实例。
- 自绘最小化按钮生效；再次启动恢复真正的 Tauri 主窗口。

测试通过 UI Automation 获取实际主窗口，按进程 ID 和 `Tauri Window` 类筛选，
防止把 single-instance 插件的 16×16 辅助窗口误判为应用主窗口。

本地验证截图及输出在 `out/verification/`，该目录不提交。
最终安装包为 `out/ClassAegisService-0.1.1-setup.exe`。

## 验证边界

本机安装目录只有 `conf/config.example.toml`，没有实际的 `config.toml`。
已验证无配置时 UI 正常加载且呈现“核心进程不可达”；没有将该状态报告为整套后端已在线。
校园 MQTT 连接、真实环境业务执行和托盘退出菜单未纳入此次安装窗口回归测试。
