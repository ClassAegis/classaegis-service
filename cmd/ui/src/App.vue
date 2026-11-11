<script setup>
import { computed, ref } from 'vue'
import { HomeFilled, Document, InfoFilled, Setting, Monitor, Connection } from '@element-plus/icons-vue'
import StatusView from './views/StatusView.vue'
import LogView from './views/LogView.vue'
import AboutView from './views/AboutView.vue'

const page = ref('home')
const navItems = [
  { id: 'home', label: '首页', icon: HomeFilled },
  { id: 'logs', label: '安全日志', icon: Document },
  { id: 'about', label: '关于课佑', icon: InfoFilled },
]
const pageMeta = computed(() => navItems.find((item) => item.id === page.value) || navItems[0])
</script>

<template>
  <div class="app-shell">
    <aside class="rail">
      <div class="rail-brand" title="课佑管家">
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
        <button type="button" class="rail-item" title="本机服务"><el-icon :size="20"><Connection /></el-icon><span>本机</span></button>
        <button type="button" class="rail-item" title="设置"><el-icon :size="21"><Setting /></el-icon><span>设置</span></button>
      </div>
    </aside>

    <div class="workspace">
      <header class="topbar">
        <div class="topbar-title"><strong>课佑管家</strong><span>/</span><span>{{ pageMeta.label }}</span></div>
        <div class="topbar-actions">
          <span class="protected-chip"><i></i> 本机防护已开启</span>
          <span class="topbar-device"><el-icon :size="14"><Monitor /></el-icon>Windows 终端</span>
          <button type="button" class="window-action" title="最小化" @click="window.ocm.minimizeWindow">—</button><button type="button" class="window-action close" title="关闭" @click="window.ocm.closeWindow">×</button>
        </div>
      </header>

      <main class="workspace-body">
        <StatusView v-if="page === 'home'" @open-logs-view="page = 'logs'" />
        <LogView v-else-if="page === 'logs'" />
        <AboutView v-else />
      </main>

      <footer class="workspace-footer">
        <span>课佑管家</span><span class="footer-divider"></span><span>ClassAegisService</span><span class="footer-spacer"></span><span>协议 v0.1</span>
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
html, body { margin: 0; height: 100%; overflow: hidden; background: var(--canvas); color: var(--ink); font-family: 'Microsoft YaHei UI', 'Microsoft YaHei', 'Segoe UI', system-ui, sans-serif; -webkit-font-smoothing: antialiased; }
button { font: inherit; }
#app { height: 100vh; }
.app-shell { display: flex; height: 100%; min-width: 0; background: var(--canvas); }

.rail { display: flex; width: 76px; flex: 0 0 76px; flex-direction: column; align-items: center; padding: 20px 8px 14px; border-right: 1px solid #343536; background: #1a1b1c; }
.rail-brand { display: grid; place-items: center; width: 42px; height: 42px; margin-bottom: 25px; border-radius: 12px; background: var(--accent); color: #2a2118; box-shadow: 0 5px 16px rgba(227, 162, 75, .18); }
.rail-brand svg { width: 32px; height: 32px; }
.rail-nav, .rail-bottom { display: grid; width: 100%; gap: 8px; }
.rail-bottom { margin-top: auto; }
.rail-item { position: relative; display: grid; place-items: center; gap: 5px; width: 100%; min-height: 58px; padding: 7px 0; border: 0; border-radius: 10px; background: transparent; color: #868783; cursor: pointer; font-size: 9px; transition: background .15s ease, color .15s ease; }
.rail-item:hover { background: #2a2b2c; color: #dddeda; }
.rail-item.active { background: #343536; color: var(--accent); }
.rail-item.active::before { position: absolute; left: 0; width: 3px; height: 30px; border-radius: 0 3px 3px 0; background: var(--accent); content: ''; }

.workspace { display: flex; min-width: 0; flex: 1; flex-direction: column; background: var(--canvas); }
.topbar { display: flex; align-items: center; justify-content: space-between; height: 56px; flex: 0 0 56px; padding: 0 26px; border-bottom: 1px solid #343536; background: #222324; -webkit-app-region: drag; }
.topbar-title, .topbar-actions, .protected-chip, .topbar-device { display: flex; align-items: center; }
.topbar-title { gap: 9px; color: var(--muted); font-size: 12px; }
.topbar-title strong { color: var(--ink); font-size: 13px; }
.topbar-title span:nth-child(2) { color: #5f605e; }
.topbar-actions { gap: 12px; color: var(--muted); }
.protected-chip { gap: 6px; color: var(--ok); font-size: 10px; }
.protected-chip i { width: 6px; height: 6px; border-radius: 50%; background: var(--ok); box-shadow: 0 0 0 3px rgba(108, 196, 154, .1); }
.topbar-device { gap: 5px; font-size: 10px; }
.window-action { width: 32px; height: 30px; padding: 0; border: 0; border-radius: 7px; background: transparent; color: #8e8f8d; cursor: pointer; font-family: 'Segoe UI', sans-serif; font-size: 17px; line-height: 1; -webkit-app-region: no-drag; }
.window-action:hover { background: #3a3b3c; color: var(--ink); }
.window-action.close { font-size: 22px; }.window-action.close:hover { background: #b83d3d; color: #fff; }
.workspace-body { min-height: 0; flex: 1; overflow: auto; padding: 28px 44px 22px; }
.workspace-footer { display: flex; align-items: center; gap: 9px; min-height: 34px; padding: 0 42px 12px; color: #70716f; font-family: 'Segoe UI', sans-serif; font-size: 9px; }
.footer-divider { width: 3px; height: 3px; border-radius: 50%; background: #50514f; }
.footer-spacer { flex: 1; }

@media (max-width: 760px) {
  .rail { width: 66px; flex-basis: 66px; }
  .rail-item span { display: none; }
  .workspace-body { padding: 24px 20px 18px; }
  .workspace-footer { padding-left: 20px; padding-right: 20px; }
  .topbar { padding: 0 17px; }
  .topbar-device, .window-action { display: none; }
}
</style>
