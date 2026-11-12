# 外观模式与设置页面

2026-11-12，增加设置页面和浅色 / 深色 / 跟随系统三种外观。

外观区放在顶部，使用三张可点击的窗口预览卡片；默认跟随系统。
偏好保存到 localStorage 的 classaegis.appearance，启动时在挂载 Vue 前应用。
系统主题变化仅在跟随系统时生效，显式选择不受系统变化影响。
保存失败时保留当前外观并显示无法持久化的提示。主题控件使用原生 radio，支持键盘选择。

下方并排展示常规和通知区域；小窗口改为纵向排列。
关闭窗口现有行为显示为托盘驻留；开机启动、安全提醒、课堂免打扰为待接入布局项。
全局颜色变量覆盖侧栏、标题栏、首页、功能卡片、关于、细滚动条及预览弹窗。
切换页面时重建主滚动容器，避免继承上一页滚动位置。

验证：Vite 正式构建通过。Playwright 检查即时切换、刷新保留、显式选择不受系统影响、
跟随系统明暗变化、键盘选择、所有主要页浅色模式、预览弹窗表面及最小窗口无水平溢出。

从仓库根目录执行：

```powershell
npm run build --prefix cmd/ui
# 使用已安装的 playwright；如不在模块搜索路径中，可指定：
# $env:PLAYWRIGHT_MODULE = '绝对路径/node_modules/playwright'
node cmd/ui/build/test-appearance.cjs
```

测试使用本机 Edge 无头渲染构建页面，截图保存 out/design-preview/settings-light.png 等文件。
本轮没有生成或替换安装包，Tauri 安装内的系统主题集成尚未实测。
