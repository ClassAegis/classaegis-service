const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('ocm', {
  getStatus: () => ipcRenderer.invoke('get-status'),
  getAppInfo: () => ipcRenderer.invoke('get-app-info'),
  openLogs: () => ipcRenderer.invoke('open-logs'),
  listLogs: () => ipcRenderer.invoke('log:list'),
  readLog: (name) => ipcRenderer.invoke('log:read', name),
})
