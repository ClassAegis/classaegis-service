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
    <div class="hills">
      <svg viewBox="0 0 900 600" preserveAspectRatio="xMaxYMax slice">
        <path d="M0 600 L180 420 L360 600 Z" fill="rgba(255,255,255,0.35)" />
        <path d="M220 600 L430 380 L640 600 Z" fill="rgba(255,255,255,0.22)" />
        <path d="M520 600 L700 460 L900 600 Z" fill="rgba(255,255,255,0.3)" />
      </svg>
    </div>
    <StatusView v-if="page === 'home'" />
    <AboutView v-else />
  </div>
</template>

<style>
:root {
  --sidebar: #2c3144;
  --text-strong: #2c3144;
  --text-mid: #5a6273;
  --muted: #8a92a6;
  --primary: #3da4ff;
  --primary-deep: #2196f3;
  --ok: #08d152;
  --bad: #eb4240;
  --card: #ffffff;
}
html,
body {
  margin: 0;
  height: 100%;
  background: #eaf4ff;
  color: var(--text-strong);
  font-family: 'Microsoft YaHei UI', 'Microsoft YaHei', system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  overflow: hidden;
}
#app {
  display: flex;
  height: 100vh;
  padding-top: 40px;
  box-sizing: border-box;
}

/* dark-blue icon rail */
.rail {
  width: 64px;
  flex: none;
  background: var(--sidebar);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  z-index: 2;
}
.logo-mark {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  display: grid;
  place-items: center;
  font-size: 17px;
  font-weight: 700;
  color: var(--sidebar);
  background: #fff;
  margin-bottom: 24px;
  user-select: none;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.nav-item {
  width: 42px;
  height: 42px;
  border-radius: 11px;
  display: grid;
  place-items: center;
  color: rgba(255, 255, 255, 0.55);
  cursor: pointer;
  transition: all 0.15s ease;
}
.nav-item:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
}
.nav-item.active {
  background: var(--primary);
  color: #fff;
  box-shadow: 0 4px 12px rgba(61, 164, 255, 0.45);
}
.nav-item.disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.rail-bottom {
  margin-top: auto;
}

/* light gradient content with hill silhouettes */
.content {
  flex: 1;
  position: relative;
  overflow-y: auto;
  padding: 26px 36px;
  background: linear-gradient(160deg, #eaf4ff 0%, #d8eaff 55%, #cfe9ff 100%);
}
.hills {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.hills svg {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 100%;
  height: 100%;
}

/* element plus light alignment */
:root {
  --el-color-primary: var(--primary);
  --el-text-color-primary: var(--text-strong);
  --el-text-color-regular: var(--text-mid);
}
</style>
