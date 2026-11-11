// Keep the renderer API identical across Electron and Tauri.
// Electron injects window.ocm through preload; Tauri exposes its allowlisted
// commands through __TAURI__.core when withGlobalTauri is enabled.
export function installDesktopBridge() {
  if (window.ocm || !window.__TAURI__?.core?.invoke) return

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
