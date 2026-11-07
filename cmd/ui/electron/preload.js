const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('ocm', {
  getStatus: () => ipcRenderer.invoke('get-status'),
  getAppInfo: () => ipcRenderer.invoke('get-app-info'),
})
