const { app, BrowserWindow, ipcMain } = require('electron')
const path = require('path')
const fs = require('fs')
const http = require('http')

const CORE_PORT = 39817
let config = { schoolCode: '', sn: '', ipcToken: '', path: '' }

function readConfig() {
  const candidates = [
    process.env.OCM_CONFIG,
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

function createWindow() {
  const win = new BrowserWindow({
    width: 980,
    height: 660,
    minWidth: 820,
    minHeight: 560,
    title: '壹课管家',
    backgroundColor: '#101216',
    titleBarStyle: 'hidden',
    titleBarOverlay: {
      color: '#101216',
      symbolColor: '#aeb6c5',
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
}

app.whenReady().then(() => {
  readConfig()
  ipcMain.handle('get-status', () => fetchStatus())
  ipcMain.handle('get-app-info', () => ({
    name: '壹课管家',
    version: app.getVersion(),
    protocol: '0.1',
    configPath: config.path || '',
  }))
  createWindow()
})
