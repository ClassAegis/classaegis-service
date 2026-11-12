<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheckFilled, CircleCloseFilled, Connection, Document, FolderOpened, Monitor, Refresh, WarningFilled } from '@element-plus/icons-vue'

const emit = defineEmits(['open-logs-view'])
const status = ref(null)
const error = ref('')
const loaded = ref(false)
const checking = ref(false)
const lastChecked = ref(0)
let timer = null

async function refreshStatus() {
  if (checking.value) return
  checking.value = true
  try {
    const res = await window.ocm.getStatus()
    if (res?.ok) { status.value = res.data; error.value = '' } else { status.value = null; error.value = res?.error || '无法连接核心进程' }
  } catch (e) { status.value = null; error.value = String((e && e.message) || e || '无法连接核心进程') }
  finally { loaded.value = true; lastChecked.value = Date.now(); checking.value = false }
}

onMounted(() => { refreshStatus(); timer = setInterval(refreshStatus, 10000) })
onUnmounted(() => clearInterval(timer))

const phase = computed(() => {
  if (!loaded.value) return 'loading'
  if (error.value || !status.value) return 'offline'
  if (status.value.connected && status.value.abilityAlive) return 'ok'
  return 'partial'
})

const state = computed(() => ({
  loading: { tone: 'loading', label: '正在读取运行状态', description: '正在连接本机核心进程。', icon: Refresh },
  ok: { tone: 'ok', label: '课佑正在保护您的设备', description: '核心进程、连接通道和业务能力均已就绪。', icon: CircleCheckFilled },
  partial: { tone: 'warning', label: '课佑正在建立保护', description: '有服务还未就绪，请稍后重试或查看安全日志。', icon: WarningFilled },
  offline: { tone: 'danger', label: '核心进程不可达', description: '请确认课佑管家已启动，或查看安全日志定位问题。', icon: CircleCloseFilled },
})[phase.value])

const protectedDays = computed(() => (phase.value === 'ok' ? 1 : 0))
const checkedLabel = computed(() => {
  if (!lastChecked.value) return '等待检查'
  const diff = Date.now() - lastChecked.value
  if (diff < 5000) return '刚刚'
  if (diff < 60000) return `${Math.floor(diff / 1000)} 秒前`
  return new Date(lastChecked.value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
})
const metrics = computed(() => {
  const s = status.value
  const offline = phase.value === 'offline'
  return [
    { label: '连接通道', value: offline ? '未读取' : s?.connected ? '已连接' : '未连接', note: s?.connected ? 'MQTT 指令通道正常' : '等待 Broker 连接', tone: !offline && s?.connected ? 'ok' : offline ? 'muted' : 'danger', icon: Connection },
    { label: '业务能力', value: offline ? '未读取' : s?.abilityAlive ? '运行中' : '未运行', note: s?.abilityAlive ? '本地能力已注册' : '等待 ability 启动', tone: !offline && s?.abilityAlive ? 'ok' : offline ? 'muted' : 'danger', icon: Monitor },
    { label: '服务版本', value: offline ? '未读取' : s?.agentVersion || '—', note: 'ClassAegisService 当前版本', tone: 'neutral', icon: Document },
  ]
})
const serviceRows = computed(() => {
  const s = status.value
  return [
    { name: 'ClassAegisLauncher', label: '守护进程', detail: '负责启动与重启', tone: 'loading' },
    { name: 'ClassAegisCore', label: '核心进程', detail: error.value ? '环回接口不可达' : 'MQTT 与安全验签', tone: error.value ? 'danger' : loaded.value ? 'ok' : 'loading' },
    { name: 'ClassAegisAbility', label: '业务能力', detail: s?.abilityAlive ? '本地能力已注册' : '等待能力宿主', tone: s?.abilityAlive ? 'ok' : error.value ? 'danger' : 'warning' },
  ]
})

async function selfCheck() {
  await refreshStatus()
  if (phase.value === 'ok') ElMessage.success('检查完成，运行正常')
  else if (phase.value === 'partial') ElMessage.warning('检查完成，存在异常状态')
  else ElMessage.error('检查失败，核心进程不可达')
}
async function openLogs() { await window.ocm.openLogs() }
</script>

<template>
  <div class="security-home">
    <section class="hero-heading">
      <div>
        <div class="protected-days">已守护 <strong>{{ protectedDays }}</strong> 天</div>
        <h1>{{ state.label }}</h1>
        <p>{{ state.description }}</p>
      </div>
      <div class="hero-actions">
        <button type="button" class="scan-button" :disabled="checking" @click="selfCheck"><el-icon :class="{ spinning: checking }"><Refresh /></el-icon>{{ checking ? '检查中' : '立即检查' }}</button>
        <button type="button" class="scan-more" title="查看安全日志" @click="emit('open-logs-view')">⌄</button>
      </div>
    </section>

    <section class="hero-visual" aria-label="课佑守护状态">
      <svg class="circuit-art" viewBox="0 0 740 330" fill="none" aria-hidden="true">
        <g stroke="#76552b" stroke-width="1.2" opacity=".7">
          <path d="M20 270h115l45-44h100l43-48h126l44-58h85l48-42h90" /><path d="M45 305h115l42-32h128l38-47h90l42-40h112l41-31h80" />
          <path d="M180 10v52l50 47v53l72 70v65" /><path d="M300 18v44l42 42v39l72 66v65" /><path d="M486 4v45l-37 37v47l-47 47" />
        </g>
        <g fill="#b17b34" opacity=".8"><circle cx="135" cy="270" r="4"/><circle cx="280" cy="178" r="4"/><circle cx="449" cy="120" r="4"/><circle cx="583" cy="79" r="4"/><circle cx="160" cy="273" r="4"/><circle cx="429" cy="226" r="4"/></g>
        <g fill="#343638" stroke="#76552b" stroke-width="1"><rect x="74" y="233" width="54" height="31"/><rect x="544" y="26" width="66" height="39"/><rect x="610" y="214" width="75" height="42"/><rect x="365" y="257" width="56" height="31"/></g>
        <path d="M489 51 598 93v78c0 82-42 131-109 169-67-38-109-87-109-169V93l109-42Z" fill="#242628" stroke="#b17b34" stroke-width="3" />
        <path d="M489 72 573 105v65c0 67-31 108-84 141-53-33-84-74-84-141v-65l84-33Z" fill="#2f3132" stroke="#e1a34e" stroke-width="2" />
        <path d="M432 164h43l32-41 35 43h15" stroke="#e3a24b" stroke-width="10" stroke-linecap="round" stroke-linejoin="round" />
        <circle cx="432" cy="164" r="11" fill="#e3a24b"/><circle cx="507" cy="123" r="11" fill="#e3a24b"/><circle cx="557" cy="166" r="11" fill="#7bcda6"/>
      </svg>
      <div class="hero-status-badge" :class="state.tone"><el-icon><component :is="state.icon" /></el-icon><span>{{ phase === 'ok' ? '实时防护中' : phase === 'loading' ? '正在检查' : '需要关注' }}</span></div>
    </section>

    <section class="quick-grid" aria-label="快捷入口">
      <button type="button" class="quick-tile" @click="selfCheck"><span class="tile-icon"><el-icon><Refresh /></el-icon></span><span><strong>立即检查</strong><small>刷新本机防护状态</small></span></button>
      <button type="button" class="quick-tile" @click="emit('open-logs-view')"><span class="tile-icon"><el-icon><Document /></el-icon></span><span><strong>安全日志</strong><small>查看核心与能力记录</small></span></button>
      <button type="button" class="quick-tile" @click="openLogs"><span class="tile-icon"><el-icon><FolderOpened /></el-icon></span><span><strong>打开日志目录</strong><small>在资源管理器中查看</small></span></button>
      <div class="quick-summary"><span>上次检查</span><strong>{{ checkedLabel }}</strong><small>{{ status?.sn || '设备信息待读取' }}</small></div>
    </section>

    <section class="service-strip">
      <div class="strip-title"><span>服务状态</span><strong>三层防护链路</strong></div>
      <div v-for="row in serviceRows" :key="row.name" class="service-row"><i :class="row.tone"></i><span><strong>{{ row.name }}</strong><small>{{ row.label }} · {{ row.detail }}</small></span><b :class="row.tone">{{ row.tone === 'ok' ? '正常' : row.tone === 'danger' ? '异常' : '等待' }}</b></div>
    </section>
  </div>
</template>

<style scoped>
.security-home { max-width: 1120px; margin: 0 auto; }
.hero-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.protected-days { color: var(--ink-soft); font-size: 17px; }
.protected-days strong { color: var(--accent); font-size: 21px; font-weight: 500; }
.hero-heading h1 { margin: 11px 0 8px; color: var(--ink); font-size: 28px; line-height: 1.2; font-weight: 600; letter-spacing: 0; }
.hero-heading p { margin: 0; color: var(--slate); font-size: 12px; }
.hero-actions { display: flex; align-items: stretch; }
.scan-button, .scan-more { border: 0; background: var(--accent); color: #2d2419; cursor: pointer; font-size: 15px; }
.scan-button { display: inline-flex; align-items: center; gap: 9px; min-width: 145px; min-height: 42px; padding: 0 19px; font-weight: 700; }
.scan-button { border-radius: 8px 0 0 8px; }
.scan-button:hover:not(:disabled) { background: #edb15d; }
.scan-button:disabled { cursor: wait; opacity: .65; }
.scan-more { width: 34px; border-left: 1px solid rgba(45, 36, 25, .25); border-radius: 0 8px 8px 0; font-family: 'Segoe UI', sans-serif; font-size: 21px; }
.hero-visual { position: relative; min-height: 286px; margin-top: 16px; overflow: hidden; border: 1px solid #3c3d3e; border-radius: 12px; background: #242526; }
.circuit-art { display: block; width: 100%; height: 286px; }
.hero-status-badge { position: absolute; right: 12px; bottom: 11px; display: inline-flex; align-items: center; gap: 7px; padding: 7px 10px; border: 1px solid #4a4b4c; background: rgba(32, 33, 34, .88); color: var(--slate); font-size: 10px; }
.hero-status-badge.ok { border-color: rgba(108, 196, 154, .45); color: var(--ok); }
.hero-status-badge.warning { border-color: rgba(227, 162, 75, .45); color: var(--warning); }
.hero-status-badge.danger { border-color: rgba(225, 107, 101, .45); color: var(--danger); }
.quick-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)) 1.2fr; gap: 12px; margin-top: 4px; }
.quick-tile { display: flex; align-items: center; gap: 12px; min-height: 78px; padding: 14px 15px; border: 1px solid #3d3e3f; border-radius: 9px; background: var(--surface); color: var(--ink); cursor: pointer; text-align: left; transition: border-color .15s ease, background .15s ease, transform .15s ease; }
.quick-tile:hover { border-color: #665232; background: var(--surface-raised); }
.tile-icon { display: grid; place-items: center; width: 34px; height: 34px; flex: 0 0 34px; border: 1px solid #56504a; border-radius: 50%; color: var(--accent); }
.quick-tile > span:last-child { display: grid; gap: 4px; min-width: 0; }
.quick-tile strong { color: var(--ink-soft); font-size: 12px; }
.quick-tile small { color: var(--muted); font-size: 10px; }
.quick-summary { display: grid; align-content: center; gap: 5px; padding: 14px 16px; border: 1px solid #3d3e3f; border-radius: 9px; background: var(--surface-soft); color: var(--muted); font-size: 10px; }
.quick-summary strong { color: var(--ink-soft); font-family: 'Segoe UI', sans-serif; font-size: 14px; font-weight: 500; }
.quick-summary small { overflow: hidden; color: #686966; text-overflow: ellipsis; white-space: nowrap; }
.service-strip { display: grid; grid-template-columns: 1.2fr repeat(3, 1fr); align-items: center; gap: 0; margin-top: 24px; border: 1px solid #3c3d3e; border-radius: 10px; background: rgba(39, 40, 41, .65); overflow: hidden; }
.strip-title { display: grid; gap: 4px; padding: 13px 16px 13px 0; }
.strip-title span { color: var(--accent); font-size: 10px; }
.strip-title strong { color: var(--ink-soft); font-size: 12px; font-weight: 500; }
.service-row { display: flex; align-items: center; gap: 8px; min-width: 0; min-height: 60px; padding: 10px 12px; border-left: 1px solid #3c3d3e; }
.service-row > i { width: 7px; height: 7px; flex: 0 0 7px; border-radius: 50%; background: var(--muted); }
.service-row > i.ok { background: var(--ok); box-shadow: 0 0 0 3px var(--ok-soft); }
.service-row > i.warning { background: var(--warning); box-shadow: 0 0 0 3px var(--warning-soft); }
.service-row > i.danger { background: var(--danger); box-shadow: 0 0 0 3px var(--danger-soft); }
.service-row > span { display: grid; gap: 3px; min-width: 0; }
.service-row strong { overflow: hidden; color: var(--ink-soft); font-family: Consolas, 'Cascadia Mono', monospace; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.service-row small { overflow: hidden; color: var(--muted); font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.service-row b { margin-left: auto; color: var(--muted); font-size: 9px; font-weight: 500; }
.service-row b.ok { color: var(--ok); }.service-row b.warning { color: var(--warning); }.service-row b.danger { color: var(--danger); }
.spinning { animation: spin .9s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 900px) { .quick-grid { grid-template-columns: repeat(3, 1fr); }.quick-summary { grid-column: 1 / -1; border-top: 1px solid #3d3e3f; border-left: 0; }.service-strip { grid-template-columns: 1fr 1fr; }.strip-title { grid-column: 1 / -1; padding: 12px 0; }.service-row:nth-child(2) { border-left: 0; } }
@media (max-width: 640px) { .hero-heading { align-items: flex-start; flex-direction: column; }.hero-actions { width: 100%; }.scan-button { flex: 1; }.hero-visual { min-height: 218px; }.circuit-art { height: 218px; }.quick-grid, .service-strip { grid-template-columns: 1fr; }.quick-summary { grid-column: auto; border-top: 1px solid #3d3e3f; }.service-row { border-top: 1px solid #3c3d3e; border-left: 0; }.strip-title { grid-column: auto; } }
</style>
