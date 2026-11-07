<script setup>
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { Cpu, Link, Monitor, Box, Tag, Star } from '@element-plus/icons-vue'

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

const overall = computed(() => {
  if (!status.value) return { label: '离线', tone: 'bad' }
  const s = status.value
  const allOk = s.connected && s.workerAlive
  return allOk ? { label: '运行中', tone: 'ok' } : { label: '降级', tone: 'warn' }
})

const cards = computed(() => {
  const s = status.value || {}
  return [
    { icon: Monitor, label: '设备 SN', value: s.sn || '—', tone: '' },
    { icon: Tag, label: '学校代码', value: s.schoolCode || '—', tone: '' },
    {
      icon: Link,
      label: 'MQTT 连接',
      value: s.connected ? '已连接' : '断开',
      tone: s.connected ? 'ok' : 'bad',
    },
    {
      icon: Box,
      label: '业务进程',
      value: s.workerAlive ? '运行中' : '未运行',
      tone: s.workerAlive ? 'ok' : 'bad',
    },
    { icon: Cpu, label: 'Agent 版本', value: s.agentVersion || '—', tone: '' },
    { icon: Star, label: '协议版本', value: s.protocolVersion || '—', tone: '' },
  ]
})
</script>

<template>
  <div>
    <!-- hero -->
    <div class="hero" :class="overall.tone">
      <div class="ring">
        <svg viewBox="0 0 120 120">
          <circle cx="60" cy="60" r="52" class="ring-bg" />
          <circle cx="60" cy="60" r="52" class="ring-fg" :class="overall.tone" />
        </svg>
        <span class="pulse" :class="overall.tone"></span>
      </div>
      <div class="hero-text">
        <div class="hero-title">
          {{ overall.label }}
          <span class="hero-sub">壹课管家 · 本机</span>
        </div>
        <div class="hero-desc">
          {{ error ? '核心进程不可达:' + error : '全部子系统受控运行,指令通道正常' }}
        </div>
      </div>
      <div class="hero-meta">
        <div class="meta-k">上报间隔</div>
        <div class="meta-v">10s</div>
      </div>
      <div class="hero-meta">
        <div class="meta-k">Broker</div>
        <div class="meta-v">127.0.0.1:1883</div>
      </div>
    </div>

    <!-- metric cards -->
    <div class="grid">
      <div v-for="c in cards" :key="c.label" class="card">
        <div class="card-icon"><el-icon :size="18"><component :is="c.icon" /></el-icon></div>
        <div class="card-body">
          <div class="card-label">{{ c.label }}</div>
          <div class="card-value" :class="c.tone">{{ c.value }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hero {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 20px 24px;
  border-radius: 14px;
  background: linear-gradient(135deg, #1a2030, #151a24);
  border: 1px solid var(--line);
}
.ring {
  position: relative;
  width: 96px;
  height: 96px;
  flex: none;
}
.ring svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.ring-bg {
  fill: none;
  stroke: var(--line);
  stroke-width: 8;
}
.ring-fg {
  fill: none;
  stroke-width: 8;
  stroke-linecap: round;
  stroke-dasharray: 327;
  stroke-dashoffset: 60;
}
.ring-fg.ok {
  stroke: #34d399;
}
.ring-fg.warn {
  stroke: #fbbf24;
}
.ring-fg.bad {
  stroke: #f87171;
}
.pulse {
  position: absolute;
  inset: 0;
  margin: auto;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--ok);
  animation: pulse 2s infinite;
}
.pulse.warn {
  background: #fbbf24;
}
.pulse.bad {
  background: var(--bad);
  animation: none;
}
@keyframes pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.5);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(52, 211, 153, 0);
  }
  100% {
    kill... box-shadow: 0 0 0 0 rgba(52, 211, 153, 0);
  }
}
.hero-title {
  font-size: 22px;
  font-weight: 700;
}
.hero-sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--muted);
  margin-left: 8px;
}
.hero-desc {
  margin-top: 6px;
  font-size: 13px;
  color: var(--muted);
}
.hero-meta {
  margin-left: auto;
  text-align: right;
  padding-left: 24px;
}
.meta-k {
  font-size: 11px;
  color: var(--muted);
}
.meta-v {
  font-size: 14px;
  margin-top: 4px;
}

.grid {
  margin-top: 16px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.card {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 14px 16px;
  border-radius: 12px;
  background: var(--panel);
  border: 1px solid var(--line);
  transition: transform 0.15s ease, border-color 0.15s ease;
}
.card:hover {
  transform: translateY(-2px);
  border-color: #33405c;
}
.card-icon {
  width: 36px;
  height: 36px;
  border-radius: 9px;
  display: grid;
  place-items: center;
  background: rgba(79, 124, 255, 0.12);
  color: var(--accent-2);
  flex: none;
}
.card-label {
  font-size: 12px;
  color: var(--muted);
}
.card-value {
  font-size: 15px;
  font-weight: 600;
  margin-top: 2px;
}
.card-value.ok {
  color: var(--ok);
}
.card-value.bad {
  color: var(--bad);
}
</style>
