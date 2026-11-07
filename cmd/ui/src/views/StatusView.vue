<script setup>
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { ElMessage } from 'element-plus'

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
        <div class="chips" v-if="status">
          <span class="chip" :class="status.connected ? 'ok' : 'bad'">
            <i class="dot"></i>MQTT {{ status.connected ? '已连接' : '已断开' }}
          </span>
          <span class="chip" :class="status.workerAlive ? 'ok' : 'bad'">
            <i class="dot"></i>业务进程{{ status.workerAlive ? '运行中' : '未运行' }}
          </span>
        </div>
        <div class="actions">
          <button class="btn-primary" @click="selfCheck">立即自检</button>
          <button class="btn-ghost" @click="openLogs">打开日志目录</button>
        </div>
      </div>

      <div class="hero-art">
        <svg viewBox="0 0 300 220" class="circuit">
          <defs>
            <linearGradient id="g1" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#4f7cff" />
              <stop offset="1" stop-color="#22d3ee" stop-opacity="0.4" />
            </linearGradient>
          </defs>
          <g fill="none" stroke="url(#g1)" stroke-width="1.5" opacity="0.85">
            <path d="M10 40 H80 V80 H120" />
            <path d="M10 110 H60 V150 H120" />
            <path d="M10 180 H90 V130 H120" />
            <path d="M290 60 H220 V90 H180" />
            <path d="M290 150 H230 V120 H180" />
            <path d="M290 195 H210 V155 H180" />
          </g>
          <g fill="#22d3ee" opacity="0.9">
            <rect x="6" y="36" width="8" height="8" rx="1" />
            <rect x="6" y="106" width="8" height="8" rx="1" />
            <rect x="6" y="176" width="8" height="8" rx="1" />
            <rect x="286" y="56" width="8" height="8" rx="1" />
            <rect x="286" y="146" width="8" height="8" rx="1" />
            <rect x="286" y="191" width="8" height="8" rx="1" />
          </g>
          <g>
            <rect x="120" y="60" width="60" height="100" rx="6" fill="none" stroke="url(#g1)" stroke-width="2" />
            <rect x="132" y="74" width="36" height="72" rx="4" fill="rgba(79,124,255,0.10)" stroke="#4f7cff" stroke-width="1.5" />
            <text x="150" y="116" text-anchor="middle" fill="#8b93a7" font-size="12">壹</text>
          </g>
        </svg>
      </div>
    </div>

    <div class="quickbar">
      <div class="quick" @click="openLogs">
        <div class="q-icon">📂</div>
        <div class="q-label">日志目录</div>
      </div>
      <div class="quick disabled" title="UI-M1 提供">
        <div class="q-icon">📜</div>
        <div class="q-label">日志查看</div>
      </div>
      <div class="quick disabled" title="M2 提供">
        <div class="q-icon">⬆️</div>
        <div class="q-label">检查更新</div>
      </div>
    </div>

    <div class="verline">
      版本 0.1.0 · 协议 0.1 · OneClassMesh 壹课互联
    </div>
  </div>
</template>

<style scoped>
.page {
  min-height: calc(100vh - 96px);
  display: flex;
  flex-direction: column;
}
.hero {
  display: flex;
  align-items: center;
  gap: 24px;
  flex: 1;
}
.hero-left {
  flex: 1;
}
.eyebrow {
  font-size: 13px;
  color: var(--muted);
}
.eyebrow b {
  color: var(--text);
  font-weight: 600;
}
.sep {
  margin: 0 6px;
  opacity: 0.5;
}
.headline {
  margin: 10px 0 18px;
  font-size: 30px;
  font-weight: 700;
  letter-spacing: 0.5px;
}
.accent {
  background: linear-gradient(90deg, #4f7cff, #22d3ee);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  margin: 0 6px;
}
.chips {
  display: flex;
  gap: 10px;
  margin-bottom: 24px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12.5px;
  padding: 5px 12px;
  border-radius: 999px;
  border: 1px solid var(--line);
  color: var(--muted);
  background: rgba(23, 26, 33, 0.7);
}
.chip.ok .dot {
  background: var(--ok);
  box-shadow: 0 0 6px var(--ok);
}
.chip.bad .dot {
  background: var(--bad);
}
.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--muted);
}
.actions {
  display: flex;
  gap: 12px;
}
.btn-primary {
  padding: 11px 30px;
  font-size: 14.5px;
  font-weight: 600;
  color: #fff;
  border: none;
  border-radius: 10px;
  cursor: pointer;
  background: linear-gradient(135deg, #4f7cff, #3b82f6);
  box-shadow: 0 6px 18px rgba(79, 124, 255, 0.35);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.btn-primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 22px rgba(79, 124, 255, 0.45);
}
.btn-ghost {
  padding: 11px 20px;
  font-size: 13.5px;
  color: var(--muted);
  background: transparent;
  border: 1px solid var(--line);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-ghost:hover {
  color: var(--text);
  border-color: #33405c;
}

.hero-art {
  flex: none;
  width: 320px;
  opacity: 0.9;
}
.circuit {
  width: 100%;
  height: auto;
}

.quickbar {
  display: flex;
  gap: 34px;
  justify-content: center;
  padding: 18px 0 6px;
  border-top: 1px solid var(--line);
  margin-top: 18px;
}
.quick {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  cursor: pointer;
  color: var(--muted);
  transition: color 0.15s ease;
}
.quick:hover {
  color: var(--text);
}
.quick.disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.q-icon {
  width: 46px;
  height: 46px;
  border-radius: 50%;
  border: 1px solid var(--line);
  display: grid;
  place-items: center;
  font-size: 18px;
  background: var(--panel);
}
.q-label {
  font-size: 12px;
}
.verline {
  text-align: center;
  font-size: 11.5px;
  color: #5c6478;
  padding: 10px 0 4px;
}
</style>
