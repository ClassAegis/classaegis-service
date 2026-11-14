// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from 'vue'

const key = 'classaegis.appearance'
const options = ['light', 'dark', 'system']
const system = window.matchMedia('(prefers-color-scheme: dark)')
function readPreference() {
  try { const value = localStorage.getItem(key); return options.includes(value) ? value : 'system' }
  catch { return 'system' }
}
export const appearance = ref(readPreference())
export const storageError = ref('')
function applyTheme() {
  const resolved = appearance.value === 'system' ? (system.matches ? 'dark' : 'light') : appearance.value
  document.documentElement.dataset.theme = resolved
  document.documentElement.style.colorScheme = resolved
}
export function setAppearance(value) {
  if (!options.includes(value)) return
  appearance.value = value
  applyTheme()
  try { localStorage.setItem(key, value); storageError.value = '' }
  catch { storageError.value = '当前无法保存偏好，本次切换仍然有效。' }
}
system.addEventListener('change', () => { if (appearance.value === 'system') applyTheme() })
window.addEventListener('storage', (event) => { if (event.key === key || event.key === null) { appearance.value = readPreference(); applyTheme() } })
applyTheme()
