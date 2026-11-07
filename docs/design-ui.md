# 壹课管家 · 显示应用(Electron)M0 设计文档

| 项 | 值 |
|---|---|
| 仓库 | oneclassmesh-agent(cmd/ui/) |
| 定位 | 壹课管家的图形界面:本机状态可视化 + 激活引导 + 日志查看。**不是管理台**(管理台在集控端) |
| 技术栈 | Electron + Vue 3 + TypeScript + Element Plus + Vite + electron-builder |
| 安全边界 | 仅连 127.0.0.1:39817(core 环回 API),X-Ocm-Token 认证;无外网访问 |
| 状态 | 设计定稿 |
| 更新 | 2026-11-07 |

## 0. 为什么 Electron 而不是 Wails/Fyne

用户选定 Electron。如实评估:界面自由度高、Web 技术栈、生态成熟、UOS/麒麟可运行;代价是体积(安装包 ~85MB)与内存(空载 ~150MB)。它是**用户主动打开**的应用,非常驻,代价可接受。安全红线:渲染进程零 Node 能力(contextIsolation),唯一数据源是 core 环回 API。

## 1. 架构

```
┌─────────────────────────────────────────────┐
│ Electron 显示应用(壹课管家 UI)              │
│ ┌─────────────┐  IPC(contextBridge)  ┌────┐ │
│ │ Renderer    │◄────────────────────►│Main│ │
│ │ Vue 3 页面  │                       │Node│ │
│ └─────────────┘                       └──┬─┘ │
└──────────────────────────────────────────────┼─┘
                                               │ HTTP 127.0.0.1:39817 + X-Ocm-Token
┌──────────────────────────────────────────────▼─┐
│ core.exe(核心进程)/status /activate(未来)     │
└─────────────────────────────────────────────────┘
```

- main(Node):读 conf/config.toml 拿 ipc_token;持 token 调 core API;文件日志读取
- preload:contextBridge 暴露最小 API(getStatus/openLogs),不开 nodeIntegration
- renderer:Vue 3 + Element Plus,只渲染数据

## 2. 页面设计(M0 三页 + 关于)

| 页面 | 内容 | 数据源 |
|---|---|---|
| ① 状态仪表 | 设备 SN / 学校码 / MQTT 连接状态 / worker 存活 / agent 版本 / 协议版本;10s 轮询刷新 | GET /status |
| ② 激活向导 | M0:提示"当前为手动录入模式"+展示 config.toml 路径;M1:输入激活码 → core /activate | 静态 + /activate(M1) |
| ③ 日志 | 查看 %ProgramData%\oneclassmesh\logs\core.log / worker.log(分页/刷新) | main 进程读文件 |
| ④ 关于 | 产品名/版本/协议版本/开源协议/License | 静态 |

## 3. 核心侧配套改造(前置)

1. **文件日志**:core/worker 目前只打 stdout;新增 internal/core/logx:落盘 `%ProgramData%\oneclassmesh\logs\core.log`(200KB×5 轮转),worker 同理 worker.log —— UI 日志页数据源
2. /status 已具备;M1 加 /activate(激活码换凭证,写回 config)

## 4. 目录与构建

```
cmd/ui/
├─ electron/
│  ├─ main.ts          main 进程:窗口/托盘/读配置/调 core API/读日志文件
│  └─ preload.ts       contextBridge 暴露 getStatus/readLogs/getAppInfo
├─ src/
│  ├─ App.vue          布局(侧边导航:状态/激活/日志/关于)
│  ├─ views/Status.vue │ Activation.vue │ Logs.vue │ About.vue
│  └─ api/index.ts     类型与请求封装(经 preload 桥)
├─ electron-builder.yml  NSIS 打包配置
└─ package.json
```

构建链:pnpm + vite(渲染层)+ electron-builder(整体 NSIS);产物 `壹课管家-Setup-0.1.0.exe`(~85MB);M2 加 electron-updater 自动更新。

## 5. 安全清单

- contextIsolation: true / nodeIntegration: false / sandbox: true
- 渲染层 CSP:default-src 'self';connect-src 'self'
- main 进程仅允许 http://127.0.0.1:39817 出站;拒绝一切其他目标
- token 只存在 main 进程内存,不进渲染层

## 6. 里程碑

- UI-M0(本次):脚手架 + 状态仪表(读 /status)+ 关于页;联调需 core 运行
- UI-M1:激活向导(接 /activate)+ 日志页(需 core/worker 文件日志先行)+ 托盘常驻
- UI-M2:自动更新(electron-updater)+ 深浅主题

## 7. 风险

- WebView2/Chromium 在老 Win10 的兼容:electron-builder NSIS 自带运行时,Win10 1809+ 无虞
- 轮询 10s 对 core 压力可忽略(单机一 UI)
- Electron 在信创 ARM/loong64 的构建矩阵:M2 评估;必要时 UI 改 Web 模式(浏览器直开)
