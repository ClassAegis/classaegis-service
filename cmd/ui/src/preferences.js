// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from 'vue'

export const preferenceDefaults = Object.freeze({ refreshInterval: 10, showCheckupMessages: true, maskDeviceId: false })
const key = 'classaegis.preferences'
const valid = {
  refreshInterval: value => [0, 10, 30, 60].includes(value),
  showCheckupMessages: value => typeof value === 'boolean',
  maskDeviceId: value => typeof value === 'boolean',
}
function read() {
  try {
    const stored = JSON.parse(localStorage.getItem(key))
    if (stored?.version !== 1 || !stored.values || typeof stored.values !== 'object') return { ...preferenceDefaults }
    return Object.fromEntries(Object.entries(preferenceDefaults).map(([name, fallback]) => [name, valid[name](stored.values[name]) ? stored.values[name] : fallback]))
  } catch { return { ...preferenceDefaults } }
}
export const preferences = ref(read())
export const preferenceError = ref('')
function save() {
  try { localStorage.setItem(key, JSON.stringify({ version: 1, values: preferences.value })); preferenceError.value = '' }
  catch { preferenceError.value = '偏好暂时无法保存，当前设置仅在本次运行中生效。' }
}
export function setPreference(name, value) {
  if (!Object.hasOwn(valid, name) || !valid[name](value)) return
  preferences.value = { ...preferences.value, [name]: value }
  save()
}
export function resetPreferences() { preferences.value = { ...preferenceDefaults }; save() }
window.addEventListener('storage', event => {
  if (event.key === key || event.key === null) preferences.value = read()
})
