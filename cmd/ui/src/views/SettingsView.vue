<script setup>
import { onMounted, ref } from 'vue'
import { preferences, setPreference, resetPreferences, preferenceError } from '../preferences'
import { Sunny, Moon, Monitor, Check, Setting, Bell } from '@element-plus/icons-vue'
import { appearance, setAppearance, storageError } from '../appearance'
const appInfo = ref(null)
const infoError = ref('')
async function loadAppInfo() {
  try { appInfo.value = await window.ocm.getAppInfo(); infoError.value = '' }
  catch { infoError.value = '暂时无法读取程序信息，请在桌面程序中重试。' }
}
onMounted(loadAppInfo)
function restoreDefaults() { resetPreferences(); setAppearance('system') }
const themes = [
  { id: 'light', label: '浅色模式', note: '明亮、清爽', icon: Sunny },
  { id: 'dark', label: '深色模式', note: '沉静、专注', icon: Moon },
  { id: 'system', label: '跟随系统', note: '随系统外观自动切换', icon: Monitor },
]
</script>

<template>
  <section class="settings-page">
    <header class="settings-heading"><div><span>偏好设置</span><h1>设置</h1><p>让课佑更贴合你的使用习惯。</p></div><span class="settings-save">设置自动保存</span></header>
    <section class="settings-panel" aria-labelledby="appearance-heading">
      <div class="settings-panel-heading"><span class="settings-section-icon"><el-icon :size="18"><Sunny /></el-icon></span><div><h2 id="appearance-heading">外观</h2><p>选择适合当前环境的界面风格</p></div><span class="settings-section-tag">即时生效</span></div>
      <div class="theme-options" role="radiogroup" aria-label="外观模式">
        <label v-for="theme in themes" :key="theme.id" class="theme-option" :class="{ selected: appearance === theme.id }">
          <input type="radio" name="appearance" :value="theme.id" :checked="appearance === theme.id" @change="setAppearance(theme.id)" />
          <span class="theme-thumbnail" :class="theme.id" aria-hidden="true"><span class="thumbnail-rail"><i></i><b></b><b></b><b></b></span><span class="thumbnail-workspace"><span class="thumbnail-topbar"></span><span class="thumbnail-title"></span><span class="thumbnail-banner"></span><span class="thumbnail-cards"><i></i><i></i><i></i></span></span><span v-if="theme.id === 'system'" class="thumbnail-split"></span></span>
          <span class="theme-option-label"><el-icon :size="15"><component :is="theme.icon" /></el-icon><strong>{{ theme.label }}</strong><span class="theme-radio"><el-icon v-if="appearance === theme.id" :size="11"><Check /></el-icon></span></span><small>{{ theme.note }}</small>
        </label>
      </div>
      <p class="appearance-note" role="status">{{ storageError || '外观即时生效，偏好保存在本机。' }}</p>
    </section>
    <div class="settings-lower">
      <section class="settings-panel" aria-labelledby="general-heading">
        <div class="settings-panel-heading"><span class="settings-section-icon"><el-icon :size="18"><Setting /></el-icon></span><div><h2 id="general-heading">常规</h2><p>状态刷新与窗口行为</p></div></div>
        <div class="setting-row"><div><label for="refresh-interval">状态刷新</label><small>控制首页自动读取运行状态的频率</small></div><select id="refresh-interval" :value="preferences.refreshInterval" @change="setPreference('refreshInterval', Number($event.target.value))"><option :value="10">每 10 秒</option><option :value="30">每 30 秒</option><option :value="60">每 60 秒</option><option :value="0">手动刷新</option></select></div>
        <div class="setting-row"><div><strong>关闭主窗口</strong><small>点击关闭后收起到托盘，点击托盘图标恢复</small></div><span class="setting-value">后台驻留</span></div>
        <div class="setting-row"><div><strong>开机启动</strong><small>由安装与服务配置管理，当前界面不修改系统启动项</small></div><span class="setting-value">服务管理</span></div>
      </section>
      <section class="settings-panel" aria-labelledby="notifications-heading">
        <div class="settings-panel-heading"><span class="settings-section-icon"><el-icon :size="18"><Bell /></el-icon></span><div><h2 id="notifications-heading">提示与隐私</h2><p>按使用场景调整</p></div></div>
        <div class="setting-row"><div><label for="checkup-messages">体检结果提示</label><small>手动体检后显示结果提示，检查列表始终保留</small></div><input id="checkup-messages" class="setting-switch" type="checkbox" role="switch" :checked="preferences.showCheckupMessages" @change="setPreference('showCheckupMessages', $event.target.checked)" /></div>
        <div class="setting-row"><div><label for="mask-device-id">隐藏设备编号</label><small>在首页隐藏完整编号，方便课堂投屏</small></div><input id="mask-device-id" class="setting-switch" type="checkbox" role="switch" :checked="preferences.maskDeviceId" @change="setPreference('maskDeviceId', $event.target.checked)" /></div>
        <p class="settings-hint">系统通知与课堂免打扰将在消息服务接入后提供。</p>
      </section>
    </div>
    <section class="settings-panel connection-panel" aria-labelledby="connection-heading">
      <div class="settings-panel-heading"><span class="settings-section-icon"><el-icon :size="18"><Monitor /></el-icon></span><div><h2 id="connection-heading">程序与服务</h2><p>本机连接信息</p></div><button type="button" class="settings-text-button" @click="loadAppInfo">重新读取</button></div>
      <dl class="connection-facts"><div><dt>桌面程序</dt><dd>ClassAegisService</dd></div><div><dt>程序版本</dt><dd>{{ appInfo?.version || '暂未读取' }}</dd></div><div><dt>配置文件</dt><dd>{{ appInfo?.configPath || '未提供配置路径' }}</dd></div></dl>
      <p v-if="infoError" class="settings-hint" role="status">{{ infoError }}</p>
    </section>
    <footer class="settings-footer"><p role="status">{{ preferenceError || storageError || '本页设置仅影响桌面界面；防护策略由核心服务管理。' }}</p><button type="button" class="settings-text-button" @click="restoreDefaults">恢复界面默认设置</button></footer>
  </section>
</template>

<style scoped>
.settings-page { max-width: 1040px; margin: 0 auto; }
.settings-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }.settings-heading > div > span { font-size: 10px; color: var(--muted); letter-spacing: 1px; }.settings-heading h1 { margin: 7px 0 8px; font-size: 23px; font-weight: 600; }.settings-heading p { margin: 0; color: var(--slate); font-size: 11px; }.settings-save { font-size: 10px; color: var(--muted); }
.settings-panel { padding: 18px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); }.settings-panel-heading { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }.settings-section-icon { display: grid; place-items: center; width: 33px; height: 33px; flex-shrink: 0; border-radius: 9px; color: var(--accent-deep); background: var(--accent-soft); }.settings-panel-heading h2 { margin: 0 0 4px; font-size: 13px; font-weight: 600; }.settings-panel-heading p { margin: 0; font-size: 10px; color: var(--muted); }.settings-section-tag { margin-left: auto; color: var(--muted); font-size: 9px; white-space: nowrap; }
.theme-options { display: grid; grid-template-columns: repeat(3,minmax(0,1fr)); gap: 14px; }.theme-option { position: relative; display: block; padding: 9px; border: 1px solid var(--line); border-radius: 10px; cursor: pointer; background: var(--surface-soft); }.theme-option:hover { border-color: var(--slate); }.theme-option.selected { border-color: var(--accent-deep); box-shadow: 0 0 0 1px var(--accent-deep); }.theme-option:focus-within { outline: 2px solid var(--accent-deep); outline-offset: 3px; }.theme-option input { position: absolute; inset: 0; z-index: 1; opacity: 0; width: 100%; height: 100%; margin: 0; cursor: pointer; }
.theme-thumbnail { display: flex; position: relative; height: 80px; overflow: hidden; border-radius: 5px; background: #f3f4f2; border: 1px solid rgba(100,100,100,.15); }.thumbnail-rail { display: flex; flex-direction: column; align-items: center; width: 25px; padding-top: 9px; gap: 8px; background: #e9ebe7; }.thumbnail-rail i { width: 10px; height: 10px; background: #dfa24e; border-radius: 3px; }.thumbnail-rail b { width: 7px; height: 4px; background: #b0b5ac; border-radius: 1px; }.thumbnail-workspace { flex: 1; padding: 0 10px; }.thumbnail-topbar { display: block; height: 14px; margin: 0 -10px 8px; border-bottom: 1px solid #dde0d9; }.thumbnail-title { display: block; width: 34px; height: 4px; margin-bottom: 7px; border-radius: 2px; background: #979d91; }.thumbnail-banner { display: block; height: 24px; border: 1px solid #e8d9c0; border-radius: 4px; background: #f6ecda; }.thumbnail-cards { display: flex; gap: 4px; margin-top: 6px; }.thumbnail-cards i { height: 13px; flex: 1; border: 1px solid #dfe2dc; background: #fff; border-radius: 3px; }.dark.theme-thumbnail, .system.theme-thumbnail { background: #222425; }.dark .thumbnail-rail, .system .thumbnail-rail { background: #191b1b; }.dark .thumbnail-topbar, .system .thumbnail-topbar { border-color: #3b3e3c; }.dark .thumbnail-title, .system .thumbnail-title { background: #747a74; }.dark .thumbnail-banner, .system .thumbnail-banner { background: #353129; border-color: #554630; }.dark .thumbnail-cards i, .system .thumbnail-cards i { background: #2d302e; border-color: #434742; }.thumbnail-split { position: absolute; inset: 0 50% 0 0; background: #d6d9c9; opacity: .6; mix-blend-mode: screen; pointer-events: none; }
.theme-option-label { display: flex; align-items: center; gap: 6px; margin: 11px 1px 5px; color: var(--ink); }.theme-option-label strong { font-size: 11px; font-weight: 500; }.theme-option > small { display: block; padding-left: 1px; font-size: 9px; color: var(--muted); }.theme-radio { display: grid; place-items: center; margin-left: auto; width: 14px; height: 14px; border: 1px solid var(--line); border-radius: 50%; }.selected .theme-radio { background: var(--accent-deep); border-color: var(--accent-deep); color: white; }.appearance-note { margin: 14px 0 0; color: var(--muted); font-size: 10px; }
.settings-lower { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-top: 16px; }.settings-lower .settings-panel { padding: 18px; }.settings-lower .settings-panel-heading { margin-bottom: 4px; }.setting-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding-top: 11px; margin-top: 10px; border-top: 1px solid var(--line); }.setting-row > div { display: grid; gap: 5px; }.setting-row strong, .setting-row label { font-size: 11px; font-weight: 500; }.setting-row small { color: var(--muted); font-size: 9px; }.setting-value { color: var(--slate); font-size: 9px; white-space: nowrap; }.planned-control { font-size: 9px; color: var(--muted); border: 1px dashed var(--line); border-radius: 4px; padding: 3px 6px; white-space: nowrap; }
.setting-row > div { min-width:0; }.setting-row small { line-height:1.65; }.setting-row select { max-width:112px; flex-shrink:0; padding:6px 8px; border:1px solid var(--line); border-radius:6px; background:var(--surface-soft); color:var(--ink-soft); font:inherit; font-size:11px; }
.setting-switch { appearance:none; position:relative; flex:0 0 32px; width:32px; height:18px; margin:0; border:1px solid var(--line); border-radius:12px; background:var(--surface-raised); cursor:pointer; }.setting-switch::before { content:''; position:absolute; top:2px; left:2px; width:12px; height:12px; border-radius:50%; background:var(--muted); transition:transform .15s; }.setting-switch:checked { background:var(--accent); border-color:var(--accent); }.setting-switch:checked::before { transform:translateX(14px); background:#fff; }
.settings-hint { color:var(--muted); font-size:10px; line-height:1.7; margin:14px 0 0; }.connection-panel { margin-top:16px; }.connection-facts { display:grid; grid-template-columns:1fr 1fr; gap:15px; margin:0; }.connection-facts > div:last-child { grid-column:1/-1; }.connection-facts dt { color:var(--muted); font-size:10px; }.connection-facts dd { margin:5px 0 0; color:var(--ink-soft); font-size:11px; overflow-wrap:anywhere; }.settings-text-button { padding:5px 0; border:0; background:transparent; color:var(--accent); font:inherit; font-size:10px; cursor:pointer; }.settings-panel-heading > button { margin-left:auto; }.settings-footer { display:flex; justify-content:space-between; align-items:center; gap:16px; margin-top:14px; }.settings-footer p { margin:0; color:var(--muted); font-size:10px; }.settings-footer button { white-space:nowrap; }
button:focus-visible,select:focus-visible,.setting-switch:focus-visible { outline:2px solid var(--accent); outline-offset:3px; }
@media(max-width:860px) { .settings-lower { grid-template-columns: 1fr; }.theme-options { gap: 9px; }.settings-panel { padding: 16px; } }
</style>
