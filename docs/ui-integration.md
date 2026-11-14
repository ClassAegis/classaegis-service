<!-- SPDX-License-Identifier: AGPL-3.0-only -->
# 桌面界面对接说明

## 当前入口

Vue 页面通过 cmd/ui/src/bridge.js 的 window.ocm 访问 Tauri 固定命令。
原生实现位于 cmd/ui/src-tauri/src/main.rs，入口已经注册到 invoke_handler。
界面偏好由 cmd/ui/src/preferences.js 管理；外观由 appearance.js 管理。

| 页面方法 | Tauri 命令 | 返回数据 |
|---|---|---|
| getStatus() | get_status | 成功为 { ok: true, data: 状态对象 }；失败拒绝 Promise |
| getAppInfo() | get_app_info | { name, version, protocol, configPath } |
| minimizeWindow() | window_minimize | 成功无数据；失败拒绝 Promise |
| closeWindow() | window_close | 隐藏主窗口，后台驻留 |

get_status 由原生端读取本机配置并查询 127.0.0.1:39817/status，认证信息不传入页面。
Core 的 HTTP 响应直接返回状态对象，Tauri 再包裹 ok/data，不要在 Core 重复包裹。

## 状态字段

| 字段 | 类型 | 页面用途 |
|---|---|---|
| sn | string | 首页设备编号；隐藏设置只影响显示 |
| connected | boolean | 连接通道检查 |
| abilityAlive | boolean | 能力服务检查 |
| agentVersion | string | 首页服务版本，独立于桌面程序版本 |
| schoolCode | string | 当前未展示 |
| protocolVersion | string | 当前未展示 |

核心可达且 connected 与 abilityAlive 均为 true 时显示运行正常；任一为 false 时显示需要关注。
查询失败时核心显示不可达，连接与能力显示待检查。当前全面体检只刷新这三项运行状态。
首页自动刷新只在页面挂载期间运行；手动模式仍在进入首页时读取一次，并允许点击体检。

## 后续业务能力

杀毒、清理、还原、弹窗拦截、软件管理和工具箱的动作目前为设计预览，尚未发送执行指令。
接入时应扩展固定命令及结构化结果，提供任务状态、进度、失败原因与取消行为，
再替换 FeatureView.vue 中的预览说明。不要仅根据服务在线就显示查毒成功或还原已开启。
开机启动由安装与服务配置管理，系统通知和课堂免打扰等待消息服务接入。

## 验证范围

从仓库根目录执行 npm run build --prefix cmd/ui，再执行 node cmd/ui/build/test-appearance.cjs。
Playwright 模块可通过 PLAYWRIGHT_MODULE 指定，测试使用本机 Edge 和模拟桥接响应。
该测试覆盖页面、偏好与状态分支，不代表 Core、Ability 或安装后的系统集成已经验证。
本轮未替换安装包；跨程序联调应另外验证真实状态接口、认证失败、断线恢复与托盘恢复。
