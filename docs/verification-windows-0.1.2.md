# 0.1.2 窗口与滚动条调整

2026-11-12：按用户截图调整窗口控件和滚动条。

- 最小化和关闭使用相同的 12px SVG 画布、46px 点击区域，居中对齐；窗口控件与信息区之间增加细分隔线。
- 滚动条为 4px 灰色圆角滑块，透明轨道，无箭头及白色底栏；仍可拖动及滚轮滚动。
- Tauri 禁用最大化，默认非全屏；拦截 F11，窗口尺寸事件中纠正全屏与最大化状态。保持普通窗口可调整大小。

运行 `npm run tauri:build --prefix cmd/ui`，通过前端、Rust 和 NSIS 完整构建。
安装到 `D:\ClassAegisService` 后运行 `cmd/ui/build/test-installed.ps1`，通过组件完整性、
实际窗口 CanMaximize=false、两按钮位置尺寸对齐、三页加载、关闭隐藏、最小化及再次启动恢复测试。
真实截图 `out/verification/installed-0.1.2.png` 已核对细滚动条及按钮样式。
F11 及全屏纠正逻辑本轮仅检查代码和构建，未进行键盘快捷键的自动化实测。

安装包：`out/ClassAegisService-0.1.2-setup.exe`。
本轮没有修改 Go 业务代码，继续沿用 0.1.1 已通过的完整 Go 测试及 vet 结果。
