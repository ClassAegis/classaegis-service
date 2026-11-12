export function installDesktopBridge() {
  if (!window.__TAURI__?.core?.invoke) return

  document.addEventListener('keydown', (event) => {
    if (event.key === 'F11') event.preventDefault()
  }, true)

  const invoke = window.__TAURI__.core.invoke
  window.ocm = {
    getStatus: () => invoke('get_status'),
    minimizeWindow: () => invoke('window_minimize'),
    closeWindow: () => invoke('window_close'),
    getAppInfo: () => invoke('get_app_info'),
    openLogs: () => invoke('open_logs'),
    listLogs: () => invoke('list_logs'),
    readLog: (name) => invoke('read_log', { name }),
  }
}
