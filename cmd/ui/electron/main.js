const { app, BrowserWindow, Menu, Tray, ipcMain, nativeImage, shell } = require('electron')
const path = require('path')
const fs = require('fs')
const http = require('http')

// 单实例:重复启动时激活已有窗口(托盘常驻,不允许双开)
if (!app.requestSingleInstanceLock()) {
  app.quit()
}

const CORE_PORT = 39817
let config = { schoolCode: '', sn: '', ipcToken: '', path: '' }

function readConfig() {
  const candidates = [
    process.env.OCM_CONFIG,
    // packaged layout: <install>\conf\config.toml (exe lives in <install>\壹课管家\)
    app.isPackaged ? path.join(path.dirname(process.execPath), '..', 'conf', 'config.toml') : null,
    path.join(process.cwd(), 'conf', 'config.toml'),
    path.join(__dirname, '..', '..', 'conf', 'config.toml'),
  ].filter(Boolean)
  for (const p of candidates) {
    try {
      const text = fs.readFileSync(p, 'utf8')
      const school = text.match(/^\s*school_code\s*=\s*"([^"]*)"/m)
      const sn = text.match(/^\s*sn\s*=\s*"([^"]*)"/m)
      const ipc = text.match(/^\s*ipc_token\s*=\s*"([^"]*)"/m)
      const mqtt = text.match(/^\s*mqtt_token\s*=\s*"([^"]*)"/m)
      config = {
        schoolCode: school ? school[1] : '',
        sn: sn ? sn[1] : '',
        ipcToken: (ipc && ipc[1]) ? ipc[1] : (mqtt ? 'ocm-' + mqtt[1] : ''),
        path: p,
      }
      return
    } catch (_) {
      continue
    }
  }
}

function fetchStatus() {
  return new Promise((resolve) => {
    const req = http.request(
      {
        host: '127.0.0.1',
        port: CORE_PORT,
        path: '/status',
        method: 'GET',
        timeout: 3000,
        headers: { 'X-Ocm-Token': config.ipcToken },
      },
      (res) => {
        let data = ''
        res.on('data', (chunk) => (data += chunk))
        res.on('end', () => {
          try {
            resolve({ ok: true, data: JSON.parse(data) })
          } catch (_) {
            resolve({ ok: false, error: 'bad json' })
          }
        })
      }
    )
    req.on('timeout', () => {
      req.destroy()
      resolve({ ok: false, error: 'timeout' })
    })
    req.on('error', (e) => resolve({ ok: false, error: String(e.message || e) }))
    req.end()
  })
}

// ---------- 日志页支持(U1):main 进程直读 %ProgramData%\oneclassmesh\logs ----------
function logDir() {
  return path.join(process.env.ProgramData || '.', 'oneclassmesh', 'logs')
}

// 文件名白名单校验:只允许本目录内出现的文件名,防目录穿越
function validLogName(name) {
  return /^[A-Za-z0-9_.-]+$/.test(name)
}

function listLogFiles() {
  const dir = logDir()
  let names = []
  try {
    names = fs.readdirSync(dir)
  } catch (_) {
    return []
  }
  // 主文件固定序 core → worker → launcher,备份 .N 排在其来源之后(新→旧)
  const mains = ['core.log', 'worker.log', 'launcher.log']
  const files = []
  for (const main of mains) {
    if (names.includes(main)) {
      files.push({ name: main, backup: false })
    }
    const backups = names
      .filter((n) => n.startsWith(main + '.') && /^\.\d+$/.test(n.slice(main.length)))
      .map((n) => ({ name: n, backup: true }))
      .sort((a, b) => b.name.localeCompare(a.name))
    files.push(...backups)
    }
  // 非白名单文件(如 ui.log)排在最后
  for (const n of names) {
    if (/\.log(\.\d+)?$/.test(n) && !files.some((f) => f.name === n)) {
      files.push({ name: n, backup: false })
    }
  }
  const result = []
  for (const f of files) {
    try {
      const st = fs.statSync(path.join(dir, f.name))
      result.push({ name: f.name, backup: f.backup, size: st.size, mtimeMs: st.mtimeMs })
    } catch (_) {}
  }
  return result
}

function readLogFile(name) {
  if (!validLogName(name)) return { ok: false, error: '非法文件名' }
  const dir = logDir()
  // 必须真实存在于目录中(防穿越的最终防线)
  if (!fs.existsSync(path.join(dir, name))) return { ok: false, error: '文件不存在' }
  try {
    const st = fs.statSync(path.join(dir, name))
    const text = fs.readFileSync(path.join(dir, name), 'utf8')
    return { ok: true, text, size: st.size, mtimeMs: st.mtimeMs }
  } catch (e) {
    return { ok: false, error: String((e && e.message) || e) }
  }
}

function createWindow() {
  const win = new BrowserWindow({
    width: 980,
    height: 660,
    minWidth: 820,
    minHeight: 560,
    title: '壹课管家',
    backgroundColor: '#eaf4ff',
    // 开发态窗口图标;打包态用 exe 内嵌图标
    icon: app.isPackaged ? undefined : path.join(__dirname, '..', 'build', 'icon.ico'),
    titleBarStyle: 'hidden',
    titleBarOverlay: {
      color: '#eaf4ff',
      symbolColor: '#5a6273',
      height: 40,
    },
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  })
  win.loadFile(path.join(__dirname, '..', 'dist', 'index.html'))
  win.setMenu(null)

  // 关窗 = 隐藏到托盘(U2);只有托盘菜单「退出」真正退出
  win.on('close', (e) => {
    if (!quitting) {
      e.preventDefault()
      win.hide()
      notifyHiddenOnce()
    }
  })
}

// ---------- 托盘(U2) ----------
let tray = null
let quitting = false
let shownHiddenTip = false

function showMainWindow() {
  const win = BrowserWindow.getAllWindows()[0]
  if (!win) return
  if (win.isMinimized()) win.restore()
  win.show()
  win.focus()
}

// 首次最小化到托盘时,气泡提示一次
function notifyHiddenOnce() {
  if (shownHiddenTip || !tray) return
  shownHiddenTip = true
  if (process.platform === 'win32') {
    tray.displayBalloon({
      icon: nativeImage.createFromPath(path.join(__dirname, 'assets', 'tray-32.png')),
      title: '壹课管家',
      content: '已最小化到系统托盘,后台继续守护;点击托盘图标可重新打开。',
    })
  }
}

// 托盘菜单:显示主窗口 / 开机自启(当前用户,HKCU Run)/ 退出
function createTray() {
  const iconPath = path.join(__dirname, 'assets', 'tray-32.png')
  tray = new Tray(iconPath)
  tray.setToolTip('壹课管家 · 后台守护运行中')
  tray.on('click', showMainWindow)
  tray.on('double-click', showMainWindow)
  const menu = Menu.buildFromTemplate([
    { label: '显示主窗口', click: showMainWindow },
    {
      label: '开机自启(当前用户)',
      type: 'checkbox',
      checked: app.getLoginItemSettings().openAtLogin,
      click: (item) => {
        app.setLoginItemSettings({ openAtLogin: item.checked })
      },
    },
    { type: 'separator' },
    {
      label: '退出',
      click: () => {
        quitting = true
        app.quit()
      },
    },
  ])
  tray.setContextMenu(menu)
}

// IPC:日志列表与读取;autostart 由托盘菜单直控,不经渲染层
app.whenReady().then(() => {
  readConfig()
  ipcMain.handle('get-status', () => fetchStatus())
  ipcMain.handle('get-app-info', () => ({
    name: '壹课管家',
    version: app.getVersion(),
    protocol: '0.1',
    configPath: config.path || '',
  }))
  ipcMain.handle('open-logs', async () => {
    const base = process.env.ProgramData || '.'
    const dir = path.join(base, 'oneclassmesh', 'logs')
    try {
      fs.mkdirSync(dir, { recursive: true })
    } catch (_) {}
    const err = await shell.openPath(dir)
    return { ok: !err, error: err || '' }
  })
  ipcMain.handle('log:list', () => ({ ok: true, files: listLogFiles() }))
  ipcMain.handle('log:read', (_e, name) => readLogFile(String(name || '')))
  createTray()
  createWindow()
  app.on('second-instance', showMainWindow)
})

app.on('before-quit', () => {
  quitting = true
})
