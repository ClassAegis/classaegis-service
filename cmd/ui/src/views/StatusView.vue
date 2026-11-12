<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheck, CircleClose, Connection, Monitor, Refresh, Warning, ArrowRight, Brush } from '@element-plus/icons-vue'
import ShieldIcon from '../components/ShieldIcon.vue'
import SoftwareIcon from '../components/SoftwareIcon.vue'
import ToolboxIcon from '../components/ToolboxIcon.vue'

const emit = defineEmits(['navigate'])
const status = ref(null)
const error = ref('')
const loaded = ref(false)
const checking = ref(false)
const lastChecked = ref(0)
let timer
async function refreshStatus() {
  if (checking.value) return
  checking.value = true
  try {
    const res = await window.ocm.getStatus()
    if (res?.ok && res.data) { status.value = res.data; error.value = '' }
    else { status.value = null; error.value = res?.error || '无法连接核心服务' }
  } catch (e) { status.value = null; error.value = String(e?.message || e || '无法连接核心服务') }
  finally { loaded.value = true; lastChecked.value = Date.now(); checking.value = false }
}
onMounted(() => { refreshStatus(); timer = setInterval(refreshStatus, 10000) })
onUnmounted(() => clearInterval(timer))
const phase = computed(() => !loaded.value ? 'loading' : error.value || !status.value ? 'offline' : status.value.connected && status.value.abilityAlive ? 'ok' : 'partial')
const state = computed(() => ({
  loading: { label: '正在了解您的设备', note: '正在读取本机服务与连接状态。', badge: '读取中', icon: Refresh },
  ok: { label: '设备运行状态良好', note: '核心服务、连接通道与能力服务均已就绪。', badge: '运行正常', icon: CircleCheck },
  partial: { label: '部分服务需要关注', note: '发现尚未就绪的服务，请查看下方检查结果。', badge: '需要关注', icon: Warning },
  offline: { label: '暂时无法读取设备状态', note: '请确认核心服务已启动，再重新进行体检。', badge: '服务离线', icon: CircleClose },
})[phase.value])
const checkedLabel = computed(() => lastChecked.value ? new Date(lastChecked.value).toLocaleTimeString('zh-CN', {hour:'2-digit',minute:'2-digit',second:'2-digit'}) : '等待首次检查')
const serviceRows = computed(() => {
  const known = loaded.value && !error.value && !!status.value
  return [
    { label:'核心服务', detail: known ? '本机服务响应正常' : loaded.value ? '无法连接本机核心服务' : '正在读取', tone:known?'ok':loaded.value?'offline':'loading', value:known?'正常':loaded.value?'不可达':'检查中', icon:ShieldIcon },
    { label:'连接通道', detail:known ? status.value.connected?'设备连接已建立':'等待连接管理端':'核心服务就绪后检查', tone:known ? status.value.connected?'ok':'partial':'loading', value:known ? status.value.connected?'已连接':'未连接':'待检查', icon:Connection },
    { label:'能力服务', detail:known ? status.value.abilityAlive?'本地能力服务运行中':'请检查能力服务是否启动':'核心服务就绪后检查', tone:known ? status.value.abilityAlive?'ok':'partial':'loading', value:known ? status.value.abilityAlive?'运行中':'未就绪':'待检查', icon:Monitor },
  ]
})
const quickActions = [
  { id:'antivirus',label:'病毒查杀',note:'文件与威胁检查',icon:ShieldIcon },
  { id:'cleanup',label:'垃圾清理',note:'整理设备空间',icon:Brush },
  { id:'software',label:'软件管理',note:'查看与管理应用',icon:SoftwareIcon },
  { id:'tools',label:'工具箱',note:'更多实用工具',icon:ToolboxIcon },
]
async function selfCheck() {
  await refreshStatus()
  if (phase.value === 'ok') ElMessage.success('体检完成，三项运行检查均正常')
  else if (phase.value === 'partial') ElMessage.warning('体检完成，部分服务需要关注')
  else ElMessage.error('无法完成体检，请检查核心服务')
}
</script>

<template>
  <div class="overview-home">
    <header class="overview-heading"><div><span>设备与服务，一目了然</span><h1>设备概览</h1></div><span class="overview-live"><i></i>每 10 秒更新</span></header>
    <section class="checkup-hero" :class="phase" aria-label="设备体检">
      <div class="checkup-emblem" aria-hidden="true"><span class="emblem-ring"></span><ShieldIcon /><span class="emblem-state"><el-icon><component :is="state.icon" /></el-icon></span></div>
      <div class="checkup-copy"><span class="checkup-badge">{{ state.badge }}</span><h2>{{ state.label }}</h2><p>{{ state.note }}</p><div class="checkup-action"><button type="button" class="checkup-button" :disabled="checking" @click="selfCheck"><el-icon :class="{ spinning:checking }"><Refresh /></el-icon>{{ checking ? '正在体检…' : '全面体检' }}</button><span>检查服务运行与设备连接</span></div></div>
    </section>
    <section class="device-facts" aria-label="设备信息">
      <div><span>设备编号</span><strong :title="status?.sn">{{ status?.sn || '暂未读取' }}</strong></div>
      <div><span>服务版本</span><strong>{{ status?.agentVersion || '暂未读取' }}</strong></div>
      <div><span>最近检查</span><strong>{{ checkedLabel }}</strong></div>
    </section>
    <div class="overview-details">
      <section class="check-results" aria-labelledby="check-results-title"><div class="overview-section-title"><h2 id="check-results-title">运行检查</h2><span>共 3 项</span></div><div class="checks-panel"><div v-for="row in serviceRows" :key="row.label" class="check-row"><el-icon class="check-row-icon"><component :is="row.icon" /></el-icon><div><strong>{{ row.label }}</strong><small>{{ row.detail }}</small></div><span class="check-result" :class="row.tone"><i></i>{{ row.value }}</span></div></div></section>
      <section class="home-shortcuts" aria-labelledby="home-shortcuts-title"><div class="overview-section-title"><h2 id="home-shortcuts-title">常用功能</h2><span>按需维护</span></div><div class="shortcut-panel"><button v-for="action in quickActions" :key="action.id" type="button" @click="emit('navigate',action.id)"><el-icon class="shortcut-icon"><component :is="action.icon" /></el-icon><span><strong>{{ action.label }}</strong><small>{{ action.note }}</small></span><el-icon class="shortcut-arrow"><ArrowRight /></el-icon></button></div></section>
    </div>
    <p class="checkup-scope">当前体检覆盖服务运行与连接状态；病毒扫描、垃圾清理和系统还原尚未接入体检。</p>
  </div>
</template>

<style scoped>
.overview-home { max-width:1120px; margin:0 auto; }
.overview-heading { display:flex; align-items:center; justify-content:space-between; margin-bottom:16px; }
.overview-heading > div > span { color:var(--muted); font-size:11px; }
.overview-heading h1 { margin:5px 0 0; font-size:23px; font-weight:600; }
.overview-live { display:flex; align-items:center; gap:6px; color:var(--muted); font-size:10px; }
.overview-live i { width:5px; height:5px; border-radius:50%; background:var(--muted); }
.checkup-hero { display:flex; align-items:center; gap:32px; min-height:160px; padding:20px 28px; border:1px solid var(--card-line); border-radius:16px 16px 0 0; background:var(--surface); }
.checkup-emblem { position:relative; display:grid; place-items:center; flex:0 0 106px; width:106px; height:106px; border-radius:50%; background:var(--accent-soft); color:var(--accent); }
.emblem-ring { position:absolute; inset:9px; border:1px solid currentColor; border-radius:50%; opacity:.2; }
.checkup-emblem > svg { width:65px; height:65px; stroke-width:1.1; }
.emblem-state { position:absolute; right:5px; bottom:7px; display:grid; place-items:center; width:29px; height:29px; border:4px solid var(--surface); border-radius:50%; background:var(--surface); color:var(--muted); font-size:23px; }
.ok .emblem-state { color:var(--ok); }.partial .emblem-state { color:var(--warning); }.offline .emblem-state { color:var(--danger); }
.checkup-copy { min-width:0; }.checkup-badge { color:var(--muted); font-size:10px; }.ok .checkup-badge { color:var(--ok); }.partial .checkup-badge { color:var(--warning); }.offline .checkup-badge { color:var(--danger); }
.checkup-copy h2 { margin:5px 0 7px; font-size:24px; font-weight:600; letter-spacing:.3px; }.checkup-copy p { margin:0; color:var(--slate); font-size:11px; line-height:1.7; }
.checkup-action { display:flex; align-items:center; gap:14px; margin-top:14px; }.checkup-action > span { font-size:10px; color:var(--muted); }
.checkup-button { display:inline-flex; align-items:center; justify-content:center; gap:8px; min-width:132px; height:38px; padding:0 19px; border:0; border-radius:8px; background:var(--accent); color:#231d15; cursor:pointer; font-size:13px; font-weight:600; }.checkup-button:hover { filter:brightness(1.08); }.checkup-button:disabled { opacity:.6; cursor:wait; }:global(html[data-theme='light']) .checkup-button { color:white; }
.device-facts { display:grid; grid-template-columns:1.3fr 1fr 1fr; padding:13px 24px; border:1px solid var(--card-line); border-top:0; border-radius:0 0 16px 16px; background:var(--surface-soft); }
.device-facts > div { display:flex; flex-direction:column; gap:6px; min-width:0; padding:0 18px; border-left:1px solid var(--line); }.device-facts > div:first-child { padding-left:0; border:0; }.device-facts span { color:var(--muted); font-size:10px; }.device-facts strong { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:12px; font-weight:500; color:var(--ink-soft); font-variant-numeric:tabular-nums; }
.overview-details { display:grid; grid-template-columns:1.1fr 1fr; gap:20px; margin-top:18px; }.overview-section-title { display:flex; align-items:center; justify-content:space-between; margin-bottom:12px; }.overview-section-title h2 { margin:0; font-size:13px; font-weight:500; }.overview-section-title > span { color:var(--muted); font-size:10px; }
.checks-panel,.shortcut-panel { min-height:144px; padding:0 16px; border:1px solid var(--card-line); border-radius:12px; background:var(--surface); }
.check-row { display:flex; align-items:center; gap:11px; min-height:48px; border-bottom:1px solid var(--line); }.check-row:last-child { border:0; }.check-row-icon { color:var(--slate); font-size:20px; flex-shrink:0; }.check-row > div { display:grid; gap:5px; min-width:0; }.check-row strong { font-size:11px; font-weight:500; }.check-row small { color:var(--muted); font-size:10px; }.check-result { margin-left:auto; display:flex; align-items:center; gap:5px; white-space:nowrap; color:var(--muted); font-size:10px; }.check-result i { width:4px; height:4px; border-radius:50%; background:currentColor; }.check-result.ok { color:var(--ok); }.check-result.partial { color:var(--warning); }.check-result.offline { color:var(--danger); }
.shortcut-panel { display:grid; grid-template-columns:1fr 1fr; padding:10px; gap:4px; }.shortcut-panel button { display:flex; align-items:center; gap:9px; min-width:0; padding:9px; border:0; border-radius:8px; background:transparent; color:var(--ink); text-align:left; cursor:pointer; }.shortcut-panel button:hover { background:var(--surface-soft); }.shortcut-icon { flex-shrink:0; font-size:23px; color:var(--slate); }.shortcut-panel button > span { display:grid; gap:5px; }.shortcut-panel strong { font-size:11px; font-weight:500; }.shortcut-panel small { color:var(--muted); font-size:9px; }.shortcut-arrow { margin-left:auto; color:var(--muted); font-size:10px; }
.checkup-scope { margin:11px 0 0; color:var(--muted); font-size:10px; line-height:1.7; }
button:focus-visible { outline:2px solid var(--accent); outline-offset:3px; }.spinning { animation:spin .9s linear infinite; }@keyframes spin { to { transform:rotate(360deg); } }
@media (max-width:860px) { .checkup-hero { gap:23px; padding:20px; }.checkup-copy h2 { font-size:21px; }.checkup-action > span { display:none; }.overview-details { gap:14px; }.shortcut-panel { grid-template-columns:1fr 1fr; }.shortcut-panel small,.shortcut-arrow { display:none; }.shortcut-panel { min-height:144px; padding:5px; }.shortcut-panel button { padding:5px 9px; }.shortcut-icon { font-size:18px; } }
@media (max-height:600px) and (min-width:641px) {
.overview-heading > div > span { display:none; }.overview-heading { margin-bottom:12px; }.overview-heading h1 { margin:0; font-size:21px; }
.checkup-hero { min-height:140px; padding:16px 22px; }.checkup-emblem { flex-basis:86px; width:86px; height:86px; }.checkup-emblem > svg { width:50px; height:50px; }.checkup-copy h2 { font-size:20px; }.checkup-action { margin-top:10px; }.checkup-button { height:34px; }
.device-facts { padding:10px 20px; }.overview-details { margin-top:15px; }.checks-panel,.shortcut-panel { min-height:132px; }.check-row { min-height:44px; }.check-row small { font-size:9px; }.checkup-scope { font-size:9px; margin-top:9px; }
}
@media (max-width:640px) { .checkup-hero { flex-direction:column; text-align:center; }.checkup-action { justify-content:center; }.overview-details { grid-template-columns:1fr; }.device-facts { padding:14px; }.device-facts > div { padding:0 9px; }.overview-live { display:none; } }
</style>
