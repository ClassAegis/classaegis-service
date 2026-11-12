<script setup>
import { computed, ref } from 'vue'
import { HomeFilled, InfoFilled, Setting, Monitor, Brush, RefreshLeft, Tools, MuteNotification } from '@element-plus/icons-vue'
import StatusView from './views/StatusView.vue'
import ShieldIcon from './components/ShieldIcon.vue'
import AboutView from './views/AboutView.vue'
import FeatureView from './views/FeatureView.vue'

function minimizeWindow() { return window.ocm.minimizeWindow() }
function closeWindow() { return window.ocm.closeWindow() }

const page = ref('home')
const navItems = [
  { id: 'home', label: '首页', icon: HomeFilled },
  { id: 'antivirus', label: '杀毒', icon: ShieldIcon },
  { id: 'cleanup', label: '清理', icon: Brush },
  { id: 'freeze', label: '还原', icon: RefreshLeft },
  { id: 'popup', label: '弹窗拦截', icon: MuteNotification },
  { id: 'tools', label: '工具箱', icon: Tools },
]
const pageMeta = computed(() => [...navItems, { id: 'about', label: '关于课佑' }].find((item) => item.id === page.value) || navItems[0])
</script>

<template>
  <div class="app-shell">
    <aside class="rail">
      <div class="rail-brand" title="ClassAegisService">
        <svg viewBox="0 0 48 48" fill="none" aria-hidden="true">
          <path d="M24 5.5 39 11v10.8c0 9.7-5.8 16.8-15 21.2-9.2-4.4-15-11.5-15-21.2V11l15-5.5Z" fill="currentColor" opacity=".18" />
          <path d="M24 8.5 36 13v8.7c0 7.9-4.4 13.8-12 17.6-7.6-3.8-12-9.7-12-17.6V13l12-4.5Z" stroke="currentColor" stroke-width="2.4" />
          <path d="M16.5 24.5h6.1l4.4-6 4.5 5.8h.9" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" />
          <circle cx="16.5" cy="24.5" r="2.2" fill="currentColor" /><circle cx="27" cy="18.5" r="2.2" fill="currentColor" /><circle cx="32.4" cy="24.3" r="2.2" fill="currentColor" />
        </svg>
      </div>
      <nav class="rail-nav" aria-label="主导航">
        <button v-for="item in navItems" :key="item.id" type="button" class="rail-item" :class="{ active: page === item.id }" :title="item.label" @click="page = item.id">
          <el-icon :size="21"><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </button>
      </nav>
      <div class="rail-bottom">
        <button type="button" class="rail-item" :class="{ active: page === 'about' }" title="关于课佑" @click="page = 'about'"><el-icon :size="20"><InfoFilled /></el-icon><span>关于</span></button>
        <button type="button" class="rail-item" title="设置"><el-icon :size="21"><Setting /></el-icon><span>设置</span></button>
      </div>
    </aside>

    <div class="workspace">
      <header class="topbar">
        <div class="topbar-title"><strong>ClassAegisService</strong><span>/</span><span>{{ pageMeta.label }}</span></div>
        <div class="topbar-actions">
          <span class="protected-chip"><i></i> 本机状态监测</span>
          <span class="topbar-device"><el-icon :size="14"><Monitor /></el-icon>Windows 终端</span>
          <div class="window-controls" aria-label="窗口控制">
            <button type="button" class="window-action" title="最小化" aria-label="最小化" @click="minimizeWindow">
              <svg viewBox="0 0 12 12" fill="none" aria-hidden="true"><path d="M1 6h10" /></svg>
            </button>
            <button type="button" class="window-action close" title="关闭" aria-label="关闭" @click="closeWindow">
              <svg viewBox="0 0 12 12" fill="none" aria-hidden="true"><path d="m1.5 1.5 9 9m0-9-9 9" /></svg>
            </button>
          </div>
        </div>
      </header>

      <main class="workspace-body">
        <StatusView v-if="page === 'home'" @navigate="page = $event" />
        <AboutView v-else-if="page === 'about'" /><FeatureView v-else :key="page" :page="page" />
      </main>

      <footer class="workspace-footer">
        <span>ClassAegisService</span><span class="footer-spacer"></span><span>协议 v0.1</span>
      </footer>
    </div>
  </div>
</template>

<style>
:root {
  --canvas: #202122;
  --surface: #28292a;
  --surface-raised: #303132;
  --surface-soft: #242526;
  --line: #3b3c3d;
  --ink: #f1f1ef;
  --ink-soft: #d1d2d0;
  --slate: #a0a19e;
  --muted: #777976;
  --accent: #e3a24b;
  --accent-deep: #c88732;
  --accent-soft: rgba(227, 162, 75, .14);
  --ok: #6cc49a;
  --ok-soft: rgba(108, 196, 154, .15);
  --warning: #e3a24b;
  --warning-soft: rgba(227, 162, 75, .14);
  --danger: #e16b65;
  --danger-soft: rgba(225, 107, 101, .16);
  --shadow: 0 12px 28px rgba(0, 0, 0, .16);
  --el-color-primary: var(--accent);
  --el-text-color-primary: var(--ink);
  --el-text-color-regular: var(--ink-soft);
}

* { box-sizing: border-box; }
*::-webkit-scrollbar { width: 4px; height: 4px; background: transparent; }
*::-webkit-scrollbar-track { background: transparent; margin-block: 10px; }
*::-webkit-scrollbar-thumb { border-radius: 4px; background: #555756; }
*::-webkit-scrollbar-thumb:hover { background: #777a77; }
*::-webkit-scrollbar-button { display: none; width: 0; height: 0; }
*::-webkit-scrollbar-corner { background: transparent; }
.el-scrollbar__bar.is-vertical { width: 4px; }
.el-scrollbar__bar.is-horizontal { height: 4px; }
.el-scrollbar__thumb { background-color: #777a77; }
html, body { margin: 0; height: 100%; overflow: hidden; background: var(--canvas); color: var(--ink); font-family: 'Microsoft YaHei UI', 'Microsoft YaHei', 'Segoe UI', system-ui, sans-serif; -webkit-font-smoothing: antialiased; }
button { font: inherit; }
#app { height: 100vh; }
.app-shell { display: flex; height: 100%; min-width: 0; background: var(--canvas); }

.rail { display: flex; width: 76px; flex: 0 0 76px; flex-direction: column; align-items: center; padding: 17px 8px 12px; border-right: 1px solid #343536; background: #1a1b1c; }
.rail-brand { display: grid; place-items: center; width: 42px; height: 42px; margin-bottom: 17px; border-radius: 12px; background: var(--accent); color: #2a2118; box-shadow: 0 5px 16px rgba(227, 162, 75, .18); }
.rail-brand svg { width: 32px; height: 32px; }
.rail-nav, .rail-bottom { display: grid; width: 100%; gap: 4px; }
.rail-bottom { margin-top: auto; padding-top: 10px; }
.rail-item { position: relative; display: grid; place-items: center; gap: 5px; width: 100%; min-height: 50px; padding: 7px 0; border: 0; border-radius: 10px; background: transparent; color: #868783; cursor: pointer; font-size: 10px; transition: background .15s ease, color .15s ease; }
.rail-item:hover { background: #2a2b2c; color: #dddeda; }
.rail-item.active { background: #343536; color: var(--accent); }
.rail-item.active::before { position: absolute; left: 0; width: 3px; height: 30px; border-radius: 0 3px 3px 0; background: var(--accent); content: ''; }

.workspace { display: flex; min-width: 0; flex: 1; flex-direction: column; background: var(--canvas); }
.topbar { display: flex; align-items: center; justify-content: space-between; height: 40px; flex: 0 0 40px; padding: 0 0 0 26px; border-bottom: 1px solid #343536; background: #222324; -webkit-app-region: drag; }
.topbar-title, .topbar-actions, .protected-chip, .topbar-device { display: flex; align-items: center; }
.topbar-title { gap: 9px; color: var(--muted); font-size: 12px; }
.topbar-title strong { color: var(--ink); font-size: 13px; }
.topbar-title span:nth-child(2) { color: #5f605e; }
.topbar-actions { align-self: stretch; gap: 12px; color: var(--muted); }
.protected-chip { gap: 6px; color: var(--ok); font-size: 10px; }
.protected-chip i { width: 6px; height: 6px; border-radius: 50%; background: var(--ok); box-shadow: 0 0 0 3px rgba(108, 196, 154, .1); }
.topbar-device { gap: 5px; font-size: 10px; }
.window-controls { position: relative; display: flex; align-self: stretch; margin-left: 4px; padding-left: 8px; }
.window-controls::before { position: absolute; left: 0; top: 50%; width: 1px; height: 18px; background: var(--line); transform: translateY(-50%); content: ''; }
.window-action { display: grid; place-items: center; width: 46px; height: 100%; padding: 0; border: 0; background: transparent; color: #b0b1af; cursor: pointer; -webkit-app-region: no-drag; }
.window-action svg { display: block; width: 12px; height: 12px; stroke: currentColor; stroke-width: 1; }
.window-action:focus-visible { outline: 1px solid var(--accent); outline-offset: -4px; }
.window-action:hover { background: #3a3b3c; color: var(--ink); }
.window-action.close:hover { background: #b83d3d; color: #fff; }
.workspace-body { min-height: 0; flex: 1; overflow: auto; padding: 28px 44px 22px; }
.workspace-footer { display: flex; align-items: center; gap: 9px; min-height: 34px; padding: 0 42px 12px; color: #70716f; font-family: 'Segoe UI', sans-serif; font-size: 9px; }
.footer-divider { width: 3px; height: 3px; border-radius: 50%; background: #50514f; }
.footer-spacer { flex: 1; }

@media (max-width: 760px) {
  .rail { width: 66px; flex-basis: 66px; }
  .rail-item span { display: none; }
  .workspace-body { padding: 24px 20px 18px; }
  .workspace-footer { padding-left: 20px; padding-right: 20px; }
  .topbar { padding: 0 0 0 17px; }
  .topbar-device { display: none; }
}
</style>
