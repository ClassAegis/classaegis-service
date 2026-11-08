<script setup>
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { ElMessage } from 'element-plus'

// 快捷区「日志查看」:请求 App.vue 切到日志页
const emit = defineEmits(['open-logs-view'])

const status = ref(null)
const error = ref('')
const loaded = ref(false)
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
  loaded.value = true
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, 10000)
})
onUnmounted(() => clearInterval(timer))

// 综合状态:loading(读取中)/ ok(全部正常)/ partial(部分异常)/ offline(核心不可达)
const phase = computed(() => {
  if (!loaded.value) return 'loading'
  if (error.value || !status.value) return 'offline'
  if (status.value.connected && status.value.workerAlive) return 'ok'
  return 'partial'
})

// 状态驱动的文案:标题强调色 + 副提示随 phase 切换
const hero = computed(
  () =>
    ({
      loading: { lead: '壹课管家', accent: '状态读取中', tail: '', hint: '正在连接核心进程,读取守护状态…' },
      ok: { lead: '壹课管家', accent: '正在守护', tail: '这台电脑', hint: 'MQTT 与业务进程均正常,指令通道畅通。' },
      partial: { lead: '壹课管家', accent: '部分服务异常', tail: '', hint: '有子系统未就绪,点击「立即自检」定位问题。' },
      offline: { lead: '壹课管家', accent: '尚未连接', tail: '核心进程', hint: '请确认启动器正在运行;也可打开日志目录排查原因。' },
    })[phase.value]
)

const chips = computed(() => {
  if (phase.value === 'offline') {
    return [{ label: '核心进程不可达', tone: 'bad' }]
  }
  const s = status.value
  return [
    { label: 'MQTT ' + (s?.connected ? '已连接' : '未连接'), tone: s?.connected ? 'ok' : 'bad' },
    { label: '业务进程' + (s?.workerAlive ? '运行中' : '未运行'), tone: s?.workerAlive ? 'ok' : 'bad' },
  ]
})

// 顶部元信息胶囊;核心不可达时值显示「未读取」而不是裸"—"
const metaItems = computed(() => {
  const s = status.value
  const na = phase.value === 'offline' ? '未读取' : '—'
  return [
    { label: '设备 SN', value: s?.sn || na },
    { label: '学校', value: s?.schoolCode || na },
    { label: '协议', value: 'v' + (s?.protocolVersion || '0.1') },
  ]
})

// 插画状态色:盾牌渐变 id + 屏内徽标底色
const shieldFill = computed(
  () =>
    ({
      loading: 'url(#shBlue)',
      ok: 'url(#shGreen)',
      partial: 'url(#shAmber)',
      offline: 'url(#shGray)',
    })[phase.value]
)
const badgeFill = computed(
  () =>
    ({
      loading: '#3da4ff',
      ok: '#0ec66a',
      partial: '#f59e0b',
      offline: '#9aa6ba',
    })[phase.value]
)

async function selfCheck() {
  await refresh()
  if (phase.value === 'ok') {
    ElMessage.success('自检通过:MQTT 与业务进程均正常')
  } else if (phase.value === 'partial') {
    ElMessage.warning('自检发现部分异常,请查看状态卡片')
  } else {
    ElMessage.error('自检未通过:核心进程不可达,请确认启动器正在运行')
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
        <div class="meta">
          <span class="m-item" v-for="m in metaItems" :key="m.label">
            <i>{{ m.label }}</i><b>{{ m.value }}</b>
          </span>
        </div>

        <h1 class="headline">
          {{ hero.lead }}<span class="accent" :class="phase">{{ hero.accent }}</span>{{ hero.tail }}
        </h1>
        <p class="hint">{{ hero.hint }}</p>

        <div class="status-card">
          <div class="chip" :class="c.tone" v-for="c in chips" :key="c.label">
            <i class="dot"></i><span>{{ c.label }}</span>
          </div>
        </div>

        <div class="actions">
          <button class="btn-primary" @click="selfCheck">立即自检</button>
          <button class="btn-ghost" @click="openLogs">打开日志目录</button>
        </div>
      </div>

      <div class="hero-art" aria-hidden="true">
        <svg viewBox="0 0 380 250">
          <defs>
            <linearGradient id="screen" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stop-color="#ffffff" />
              <stop offset="1" stop-color="#e9f3fe" />
            </linearGradient>
            <linearGradient id="shBlue" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#3da4ff" />
              <stop offset="1" stop-color="#2196f3" />
            </linearGradient>
            <linearGradient id="shGreen" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#14d479" />
              <stop offset="1" stop-color="#07b158" />
            </linearGradient>
            <linearGradient id="shAmber" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#ffb02e" />
              <stop offset="1" stop-color="#f08c00" />
            </linearGradient>
            <linearGradient id="shGray" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#b6c0d2" />
              <stop offset="1" stop-color="#8e9bb3" />
            </linearGradient>
            <filter id="soft" x="-30%" y="-30%" width="160%" height="160%">
              <feDropShadow dx="0" dy="5" stdDeviation="7" flood-color="#2c3144" flood-opacity="0.16" />
            </filter>
          </defs>

          <!-- 背景光晕 -->
          <circle cx="185" cy="112" r="98" fill="rgba(255,255,255,0.55)" />
          <circle cx="318" cy="52" r="34" fill="rgba(255,255,255,0.5)" />

          <!-- 显示器 -->
          <rect x="40" y="22" width="232" height="150" rx="16" fill="url(#screen)" stroke="#b9d9f8" stroke-width="1.5" filter="url(#soft)" />
          <rect x="54" y="36" width="204" height="122" rx="10" fill="#f6faff" />
          <circle cx="156" cy="74" r="15" :fill="badgeFill" />
          <!-- 徽标图形随状态切换:正常/读取=对勾,部分异常=叹号,离线=横线 -->
          <path v-if="phase === 'ok' || phase === 'loading'" d="M149 74 l5 5 l10 -11" fill="none" stroke="#fff" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round" />
          <path v-else-if="phase === 'partial'" d="M156 66 v10" fill="none" stroke="#fff" stroke-width="3.5" stroke-linecap="round" />
          <circle v-else-if="phase === 'partial'" cx="156" cy="80.5" r="1.9" fill="#fff" />
          <path v-else d="M149 74 h14" fill="none" stroke="#fff" stroke-width="3.5" stroke-linecap="round" />
          <rect x="114" y="102" width="84" height="7" rx="3.5" fill="#dceaf8" />
          <rect x="126" y="115" width="60" height="7" rx="3.5" fill="#e6f0fb" />
          <rect x="146" y="172" width="20" height="14" fill="#c8dcf2" />
          <rect x="118" y="186" width="76" height="9" rx="4.5" fill="#b3cde9" />

          <!-- 地面阴影 -->
          <ellipse cx="190" cy="228" rx="112" ry="8" fill="rgba(44,49,68,0.07)" />

          <!-- 盾牌(状态着色,叠于显示器右下) -->
          <path d="M262 106 h76 v34 c0 30 -17 46 -38 56 c-21 -10 -38 -26 -38 -56 Z" :fill="shieldFill" filter="url(#soft)" />
          <path d="M283 143 l13 13 l24 -27" fill="none" stroke="#fff" stroke-width="8" stroke-linecap="round" stroke-linejoin="round" />

          <!-- 点缀 -->
          <circle cx="354" cy="30" r="6" fill="rgba(255,255,255,0.9)" />
          <circle cx="24" cy="196" r="8" fill="rgba(255,255,255,0.7)" />
          <circle cx="342" cy="210" r="4" fill="rgba(61,164,255,0.35)" />
        </svg>
      </div>
    </div>

    <div class="quickbar">
      <div class="quick" @click="openLogs">
        <div class="q-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M3 7 a2 2 0 0 1 2 -2 h4 l2 2 h8 a2 2 0 0 1 2 2 v9 a2 2 0 0 1 -2 2 H5 a2 2 0 0 1 -2 -2 Z" />
          </svg>
        </div>
        <div class="q-label">日志目录</div>
      </div>
      <div class="quick" title="查看运行日志" @click="emit('open-logs-view')">
        <div class="q-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M6 3 h9 l4 4 v14 H6 Z" />
            <path d="M9 11 h6 M9 15 h6" />
          </svg>
        </div>
        <div class="q-label">日志查看</div>
      </div>
      <div class="quick disabled" title="M2 提供">
        <div class="q-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 16 V8 M8.5 11.5 L12 8 l3.5 3.5" />
          </svg>
        </div>
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
  gap: 10px;
  flex: 1;
}
.hero-left {
  flex: 1;
  z-index: 1;
}

/* 元信息胶囊 */
.meta {
  display: flex;
  gap: 8px;
  margin-bottom: 18px;
}
.m-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: rgba(255, 255, 255, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.9);
  border-radius: 999px;
  font-size: 12px;
  color: var(--text-mid);
}
.m-item i {
  font-style: normal;
  color: var(--muted);
}
.m-item b {
  color: var(--text-strong);
  font-weight: 600;
}

.headline {
  margin: 0 0 10px;
  font-size: 33px;
  font-weight: 700;
  color: var(--text-strong);
  letter-spacing: 0.5px;
}
.accent {
  margin: 0 5px;
}
.accent.loading,
.accent.ok {
  background: linear-gradient(120deg, #3da4ff, #2196f3);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.accent.partial {
  color: #f08c00;
}
.accent.offline {
  color: #8e9bb3;
}
.hint {
  margin: 0 0 22px;
  font-size: 13.5px;
  color: var(--text-mid);
}

/* 状态胶囊卡 */
.status-card {
  display: inline-flex;
  gap: 10px;
  padding: 9px;
  background: var(--card);
  border-radius: 16px;
  box-shadow: 0 6px 22px rgba(44, 49, 68, 0.08);
  margin-bottom: 26px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  font-weight: 500;
  padding: 7px 14px;
  border-radius: 10px;
}
.chip.ok {
  background: #e5f9ee;
  color: #0a9d4f;
}
.chip.bad {
  background: #fdeaea;
  color: #d64541;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.chip.ok .dot {
  background: #0ec66a;
  box-shadow: 0 0 6px rgba(14, 198, 106, 0.55);
}
.chip.bad .dot {
  background: #eb4240;
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
  width: 400px;
  z-index: 1;
}
.hero-art svg {
  width: 100%;
  height: auto;
  display: block;
}

.quickbar {
  display: flex;
  gap: 44px;
  justify-content: center;
  padding: 12px 0 2px;
}
.quick {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: transform 0.15s ease;
}
.quick:hover {
  transform: translateY(-2px);
}
.quick.disabled {
  opacity: 0.45;
  cursor: not-allowed;
  transform: none;
}
.q-icon {
  width: 52px;
  height: 52px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: var(--primary-deep);
  background: var(--card);
  box-shadow: 0 4px 14px rgba(44, 49, 68, 0.1);
  transition: box-shadow 0.15s ease;
}
.quick:hover .q-icon {
  box-shadow: 0 8px 20px rgba(61, 164, 255, 0.28);
}
.q-icon svg {
  width: 21px;
  height: 21px;
}
.q-label {
  font-size: 12px;
  color: var(--text-mid);
}
.verline {
  text-align: center;
  font-size: 11.5px;
  color: var(--muted);
  padding: 10px 0 2px;
}
</style>
