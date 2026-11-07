<script setup>
import { ref } from 'vue'
import { Monitor, InfoFilled } from '@element-plus/icons-vue'
import StatusView from './views/StatusView.vue'
import AboutView from './views/AboutView.vue'

const page = ref('status')
</script>

<template>
  <el-container class="shell">
    <el-aside width="220px" class="aside">
      <div class="brand">
        <div class="logo-mark">壹</div>
        <div>
          <div class="logo">壹课管家</div>
          <div class="sub">OneClassMesh Agent</div>
        </div>
      </div>
      <el-menu class="menu" :default-active="page" @select="(k) => (page = k)">
        <el-menu-item index="status">
          <el-icon><Monitor /></el-icon>
          <span>状态仪表</span>
        </el-menu-item>
        <el-menu-item index="about">
          <el-icon><InfoFilled /></el-icon>
          <span>关于</span>
        </el-menu-item>
      </el-menu>
      <div class="aside-foot">
        <span class="dot ok"></span>
        <span>protocol v0.1</span>
      </div>
    </el-aside>

    <el-main class="main">
      <StatusView v-if="page === 'status'" />
      <AboutView v-else />
    </el-main>
  </el-container>
</template>

<style>
:root {
  --bg: #101216;
  --panel: #171a21;
  --panel-2: #1d222c;
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
}
html.dark {
  color-scheme: dark;
}
#app {
  height: 100%;
}
.shell {
  height: 100vh;
  padding-top: 40px; /* titlebar overlay height */
}
.aside {
  background: var(--panel);
  border-right: 1px solid var(--line);
  display: flex;
  flex-direction: column;
}
.brand {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 16px;
}
.logo-mark {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: grid;
  place-items: center;
  font-weight: 700;
  color: #fff;
  background: linear-gradient(135deg, var(--accent), var(--accent-2));
}
.logo {
  font-size: 16px;
  font-weight: 600;
}
.sub {
  font-size: 11px;
  color: var(--muted);
  letter-spacing: 0.3px;
}
.menu {
  border-right: none;
  background: transparent;
  padding: 4px 8px;
}
.menu .el-menu-item {
  border-radius: 8px;
  margin: 2px 0;
  color: var(--muted);
  height: 40px;
}
.menu .el-menu-item.is-active {
  background: linear-gradient(90deg, rgba(79, 124, 255, 0.18), rgba(34, 211, 238, 0.08));
  color: var(--text);
}
.menu .el-menu-item:hover {
  background: var(--panel-2);
}
.aside-foot {
  margin-top: auto;
  padding: 12px 16px;
  font-size: 11px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 6px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.dot.ok {
  background: var(--ok);
  box-shadow: 0 0 6px var(--ok);
}
.main {
  padding: 20px 24px;
  overflow-y: auto;
}

/* dark overrides for Element Plus */
html.dark {
  --el-bg-color: var(--panel);
  --el-bg-color-overlay: var(--panel-2);
  --el-border-color: var(--line);
  --el-fill-color-blank: var(--panel);
  --el-text-color-primary: var(--text);
  --el-text-color-regular: var(--muted);
}
.el-descriptions__body {
  background: transparent;
}
</style>
