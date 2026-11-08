<script setup>
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { ElMessage } from 'element-plus'

// 快捷区「日志查看」:请求 App.vue 切到日志页
const emit = defineEmits(['open-logs-view'])

const status = ref(null)
const error = ref('')
let timer = null

async function refresh() {
  const res = await window.ocm.getStatus()
  if (res.ok) {
    status.value = res.data
    error.value = ''
  } else {
    status.value = null
    error.value = res.error || '无法连接核心进程'
  }
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, 10000)
})
onUnmounted(() => clearInterval(timer))

const allOk = computed(
  () => status.value && status.value.connected && status.value.workerAlive
)

async function selfCheck() {
  await refresh()
  if (allOk.value) {
    ElMessage.success('自检通过:MQTT 与业务进程均正常')
  } else {
    ElMessage.error('自检未通过:核心进程不可达或子系统异常')
  }
}

async function openLogs() {
  await window.ocm.openLogs()
}

const chips = computed(() => {
  const s = status.value
  return [
    { label: 'MQTT ' + (s?.connected ? '已连接' : '已断开'), ok: !!s?.connected },
    { label: '业务进程' + (s?.workerAlive ? '运行中' : '未运行'), ok: !!s?.workerAlive },
  ]
})
</script>

<template>
  <div class="page">
    <div class="hero">
      <div class="hero-left">
        <div class="eyebrow">
          设备 SN <b>{{ status?.sn || '—' }}</b>
          <span class="sep">·</span> 学校 {{ status?.schoolCode || '—' }}
          <span class="sep">·</span> 协议 v{{ status?.protocolVersion || '0.1' }}
        </div>
        <h1 class="headline">
          壹课管家<span class="accent">正在守护</span>这台电脑
        </h1>

        <div class="status-card">
          <div class="chip" v-for="c in chips" :key="c.label">
            <i class="dot" :class="c.ok ? 'ok' : 'bad'"></i>
            <span :class="c.ok ? '' : 'bad-text'">{{ c.label }}</span>
          </div>
          <div class="chip" v-if="error">
            <i class="dot bad"></i><span class="bad-text">核心进程不可达</span>
          </div>
        </div>

        <div class="actions">
          <button class="btn-primary" @click="selfCheck">立即自检</button>
          <button class="btn-ghost" @click="openLogs">打开日志目录</button>
        </div>
      </div>

      <div class="hero-art">
        <svg viewBox="0 0 340 240">
          <defs>
            <linearGradient id="screen" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stop-color="#ffffff" />
              <stop offset="1" stop-color="#eaf4ff" />
            </linearGradient>
            <linearGradient id="shield" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#3da4ff" />
              <stop offset="1" stop-color="#2196f3" />
            </linearGradient>
          </defs>
          <rect x="40" y="20" width="220" height="150" rx="12" fill="url(#screen)" stroke="#b9d9f8" stroke-width="2" />
          <rect x="58" y="38" width="184" height="114" rx="6" fill="#f2f9ff" />
          <rect x="120" y="170" width="60" height="8" rx="4" fill="#b9d9f8" />
          <path d="M235 120 c22 10 34 12 34 12 s12 -2 34 -12 v34 c0 26 -22 40 -34 46 c-12 -6 -34 -20 -34 -46 Z"
                fill="url(#shield)" opacity="0.95" />
          <path d="M252 152 l12 12 l24 -26" fill="none" stroke="#fff" stroke-width="7" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </div>
    </div>

    <div class="quickbar">
      <div class="quick" @click="openLogs">
        <div class="q-icon">📂</div>
        <div class="q-label">日志目录</div>
      </div>
      <div class="quick" title="查看运行日志" @click="emit('open-logs-view')">
        <div class="q-icon">📜</div>
        <div class="q-label">日志查看</div>
      </div>
      <div class="quick disabled" title="M2 提供">
        <div class="q-icon">⬆️</div>
        <div class="q-label">检查更新</div>
      </div>
    </div>

    <div class="verline">版本 0.1.0 · 协议 0.1 · OneClassMesh 壹课互联</div>
  </div>
</template>

<style scoped>
.page {
  position: relative;
  min-height: calc(100vh - 96px);
  display: flex;
  flex-direction: column;
}
.hero {
  display: flex;
  align-items: center;
  gap: 20px;
  flex: 1;
}
.hero-left {
  flex: 1;
  z-index: 1;
}
.eyebrow {
  font-size: 13px;
  color: var(--text-mid);
}
.eyebrow b {
  color: var(--text-strong);
}
.sep {
  margin: 0 6px;
  opacity: 0.45;
}
.headline {
  margin: 12px 0 20px;
  font-size: 32px;
  font-weight: 700;
  color: var(--text-strong);
  letter-spacing: 0.5px;
}
.accent {
  color: var(--primary-deep);
  margin: 0 4px;
}
.status-card {
  display: inline-flex;
  gap: 10px;
  padding: 10px 14px;
  background: var(--card);
  border-radius: 14px;
  box-shadow: 0 6px 22px rgba(44, 49, 68, 0.08);
  margin-bottom: 26px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  padding: 6px 14px;
  border-radius: 10px;
  background: #f2f9ff;
  color: var(--text-strong);
}
.bad-text {
  color: var(--bad);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.dot.ok {
  background: var(--ok);
  box-shadow: 0 0 6px rgba(8, 209, 82, 0.6);
}
.dot.bad {
  background: var(--bad);
}
.actions {
  display: flex;
  gap: 14px;
}
.btn-primary {
  padding: 12px 34px;
  font-size: 15px;
  font-weight: 600;
  color: #fff;
  border: none;
  border-radius: 24px;
  cursor: pointer;
  background: linear-gradient(135deg, #3da4ff, #2196f3);
  box-shadow: 0 8px 20px rgba(61, 164, 255, 0.4);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.btn-primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 26px rgba(61, 164, 255, 0.5);
}
.btn-ghost {
  padding: 12px 22px;
  font-size: 14px;
  color: var(--text-mid);
  background: rgba(255, 255, 255, 0.75);
  border: 1px solid #cfe4f8;
  border-radius: 24px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-ghost:hover {
  color: var(--primary-deep);
  border-color: var(--primary);
  background: #fff;
}

.hero-art {
  flex: none;
  width: 340px;
  z-index: 1;
}
.hero-art svg {
  width: 100%;
  height: auto;
}

.quickbar {
  display: flex;
  gap: 40px;
  justify-content: center;
  padding: 16px 0 4px;
}
.quick {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  cursor: pointer;
  color: var(--text-mid);
  transition: transform 0.15s ease;
}
.quick:hover {
  transform: translateY(-2px);
}
.quick.disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.q-icon {
  width: 50px;
  height: 50px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 19px;
  background: var(--card);
  box-shadow: 0 4px 14px rgba(44, 49, 68, 0.1);
}
.q-label {
  font-size: 12px;
  color: var(--muted);
}
.verline {
  text-align: center;
  font-size: 11.5px;
  color: var(--muted);
  padding: 8px 0 2px;
}
</style>
