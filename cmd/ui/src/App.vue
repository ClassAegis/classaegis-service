<script setup>
import { ref } from 'vue'
import { HomeFilled, Document, InfoFilled, Setting } from '@element-plus/icons-vue'
import StatusView from './views/StatusView.vue'
import AboutView from './views/AboutView.vue'

const page = ref('home')
</script>

<template>
  <div class="rail">
    <div class="logo-mark">壹</div>

    <div class="nav">
      <div class="nav-item" :class="{ active: page === 'home' }" @click="page = 'home'" title="首页">
        <el-icon :size="20"><HomeFilled /></el-icon>
      </div>
      <div class="nav-item disabled" title="日志(UI-M1)">
        <el-icon :size="20"><Document /></el-icon>
      </div>
      <div class="nav-item" :class="{ active: page === 'about' }" @click="page = 'about'" title="关于">
        <el-icon :size="20"><InfoFilled /></el-icon>
      </div>
    </div>

    <div class="rail-bottom">
      <div class="nav-item disabled" title="设置(UI-M1)">
        <el-icon :size="20"><Setting /></el-icon>
      </div>
    </div>
  </div>

  <div class="content">
    <StatusView v-if="page === 'home'" />
    <AboutView v-else />
  </div>
</template>

<style>
:root {
  --bg: #101216;
  --panel: #171a21;
  --line: #262b36;
  --text: #e6e9f0;
  --muted: #8b93a7;
  --accent: #4f7cff;
  --accent-2: #22d3ee;
  --ok: #34d399;
  --bad: #f87171;
}
html,
body {
  margin: 0;
  height: 100%;
  background: var(--bg);
  color: var(--text);
  font-family: 'Microsoft YaHei UI', 'Microsoft YaHei', system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  overflow: hidden;
}
html.dark {
  color-scheme: dark;
}
#app {
  display: flex;
  height: 100vh;
  padding-top: 40px; /* native titlebar overlay */
  box-sizing: border-box;
}

/* icon rail */
.rail {
  width: 64px;
  flex: none;
  background: #0d1015;
  border-right: 1px solid var(--line);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
}
.logo-mark {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  display: grid;
  place-items: center;
  font-size: 17px;
  font-weight: 700;
  color: #fff;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
  margin-bottom: 22px;
  cursor: default;
  user-select: none;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.nav-item {
  width: 42px;
  height: 42px;
  border-radius: 11px;
  display: grid;
  place-items: center;
  color: var(--muted);
  cursor: pointer;
  transition: all 0.15s ease;
}
.nav-item:hover {
  background: var(--panel);
  color: var(--text);
}
.nav-item.active {
  background: linear-gradient(135deg, rgba(79, 124, 255, 0.22), rgba(34, 211, 238, 0.1));
  color: #fff;
}
.nav-item.disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.nav-item.disabled:hover {
  background: transparent;
  color: var(--muted);
}
.rail-bottom {
  margin-top: auto;
}
.content {
  flex: 1;
  overflow-y: auto;
  padding: 28px 34px;
}

/* element plus dark vars */
html.dark {
  --el-bg-color: var(--panel);
  --el-bg-color-overlay: #1d222c;
  --el-border-color: var(--line);
  --el-fill-color-blank: var(--panel);
  --el-text-color-primary: var(--text);
  --el-text-color-regular: var(--muted);
  --el-color-primary: var(--accent);
}
</style>
