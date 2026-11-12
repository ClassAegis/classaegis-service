<script setup>
import { computed, ref } from 'vue'
import { Brush, Search, FolderOpened, Monitor, Connection, Cpu, RefreshLeft, Lock, Box, ArrowRight, Clock, DocumentChecked } from '@element-plus/icons-vue'

import ShieldIcon from '../components/ShieldIcon.vue'
import PopupIcon from '../components/PopupIcon.vue'
import SoftwareIcon from '../components/SoftwareIcon.vue'
import ToolboxIcon from '../components/ToolboxIcon.vue'

const props = defineProps({ page: { type: String, required: true } })
const selected = ref(null)
const sections = {
  software: { eyebrow: '应用管理', title: '教学所需，井然有序', description: '集中查看已安装应用，让软件维护与课堂准备更轻松。', label: '软件管理', action: '查看已装软件', icon: SoftwareIcon, tone: 'blue', note: '按名称、类型和安装时间整理设备上的应用', cards: [
    { title: '已装软件', description: '查看应用版本与安装信息', icon: SoftwareIcon },
    { title: '软件卸载', description: '选择不再需要的应用进行管理', icon: Box },
    { title: '安装记录', description: '查看软件安装与变更记录', icon: Clock },
  ], items: ['应用列表', '安装信息', '维护记录'] },
  popup: { eyebrow: '专注课堂', title: '少一些打扰，多一份专注', description: '管理不必要的弹窗与干扰，让课堂画面保持干净。', label: '弹窗拦截', action: '设置拦截', icon: PopupIcon, tone: 'amber', note: '按需设置规则，并为需要的应用保留白名单', cards: [
    { title: '拦截规则', description: '管理需要拦截的弹窗与应用', icon: PopupIcon },
    { title: '应用白名单', description: '允许教学应用正常显示通知', icon: DocumentChecked },
    { title: '拦截记录', description: '查看最近的弹窗拦截情况', icon: Clock },
  ], items: ['规则管理', '应用白名单', '拦截记录'] },
  antivirus: { eyebrow: '设备安全', title: '让每一次开机，都更安心', description: '检查可疑文件与潜在威胁，让课堂设备保持可靠。', label: '病毒查杀', action: '快速查杀', icon: ShieldIcon, tone: 'amber', note: '优先检查系统关键位置与常用程序', cards: [
    { title: '全盘查杀', description: '全面检查所有磁盘中的文件', icon: Search },
    { title: '自定义查杀', description: '选择需要检查的文件或文件夹', icon: FolderOpened },
    { title: '隔离区', description: '集中查看和管理隔离的可疑文件', icon: Lock },
  ], items: ['本地查杀引擎', '病毒库管理', '扫描记录'] },
  cleanup: { eyebrow: '空间管理', title: '少一些冗余，多一些轻快', description: '整理临时文件和日常缓存，为教学资料留出更多空间。', label: '垃圾清理', action: '扫描垃圾', icon: Brush, tone: 'mint', note: '先查看扫描结果，再选择需要清理的内容', cards: [
    { title: '系统垃圾', description: '临时文件、更新缓存与系统日志', icon: Box },
    { title: '应用缓存', description: '整理应用运行中积累的缓存', icon: FolderOpened },
    { title: '清理记录', description: '每次清理的内容都有迹可循', icon: Clock },
  ], items: ['临时文件', '应用缓存', '回收站'] },
  freeze: { eyebrow: '环境还原', title: '熟悉的课堂环境，始终如初', description: '规划系统保护与还原，让公共设备在下次开机时回到熟悉的状态。', label: '系统还原', action: '设置保护', icon: RefreshLeft, tone: 'blue', note: '选择保护分区，安排维护与还原策略', cards: [
    { title: '分区保护', description: '为系统分区设置保护范围', icon: Lock },
    { title: '维护模式', description: '在维护期间保留更新与软件安装', icon: ToolboxIcon },
    { title: '保留空间', description: '为教学文件预留不参与还原的空间', icon: FolderOpened },
  ], items: ['保护范围', '维护计划', '还原策略'] },
}
const section = computed(() => sections[props.page])
const tools = [
  { title: '进程管理', description: '查看进程与资源占用', icon: Cpu, category: '系统' },
  { title: '网络诊断', description: '检查连接与网络状态', icon: Connection, category: '网络' },
  { title: '远程桌面', description: '连接和协助校园内的设备', icon: Monitor, category: '协作' },
  { title: '远程唤醒', description: '唤醒同一网络中的终端', icon: Clock, category: '网络' },
  { title: '启动管理', description: '查看开机启动项与启动顺序', icon: Box, category: '系统' },
  { title: '设备审计', description: '查看设备与系统信息', icon: DocumentChecked, category: '系统' },
]
</script>

<template>
  <section class="feature-page" :key="page">
    <header class="feature-heading">
      <div><span class="feature-eyebrow">{{ section?.eyebrow || '实用工具' }}</span><h1>{{ section?.label || '工具箱' }}</h1></div>
      <span class="design-tag">界面预览</span>
    </header>

    <template v-if="section">
      <div class="feature-hero" :class="section.tone">
        <div class="feature-copy"><h2>{{ section.title }}</h2><p>{{ section.description }}</p><button class="feature-primary" @click="selected = { title: section.action, description: section.note }"><el-icon :size="17"><component :is="section.icon" /></el-icon>{{ section.action }}<el-icon :size="14"><ArrowRight /></el-icon></button><small>{{ section.note }}</small></div>
        <div class="feature-art" aria-hidden="true"><span class="orbit orbit-one"></span><span class="orbit orbit-two"></span><span class="art-dot"></span><div class="art-core"><el-icon :size="72"><component :is="section.icon" /></el-icon></div></div>
      </div>
      <div class="feature-section-title"><h3>常用功能</h3><span>按需管理，更有条理</span></div>
      <div class="feature-cards"><button v-for="card in section.cards" :key="card.title" class="feature-card" @click="selected = card"><span class="feature-card-icon" :class="section.tone"><el-icon :size="23"><component :is="card.icon" /></el-icon></span><strong>{{ card.title }}</strong><p>{{ card.description }}</p><el-icon class="card-arrow" :size="13"><ArrowRight /></el-icon></button></div>
      <div class="feature-summary"><span class="summary-label">{{ page === 'freeze' ? '保护策略' : '管理范围' }}</span><span v-for="item in section.items" :key="item"><i></i>{{ item }}</span><small>等待配置</small></div>
    </template>

    <template v-else>
      <div class="tools-intro"><span class="tools-intro-icon"><el-icon :size="25"><ToolboxIcon /></el-icon></span><div><h2>常用能力，随手可达</h2><p>设备维护、网络排查与课堂协作，集中在这里。</p></div><span class="tool-count">06 <small>项工具</small></span></div>
      <div class="feature-section-title"><h3>全部工具</h3><span>系统 · 网络 · 协作</span></div>
      <div class="tools-grid"><button v-for="tool in tools" :key="tool.title" class="tool-card" @click="selected = tool"><span class="tool-icon"><el-icon :size="25"><component :is="tool.icon" /></el-icon></span><span class="tool-copy"><strong>{{ tool.title }}</strong><small>{{ tool.description }}</small></span><span class="tool-category">{{ tool.category }}</span><el-icon class="tool-arrow" :size="14"><ArrowRight /></el-icon></button></div>
      <p class="tools-note">让维护更简单，让课堂更专注。</p>
    </template>
    <el-dialog :model-value="!!selected" :title="selected?.title" width="360px" class="feature-preview-dialog" @close="selected = null"><p>{{ selected?.description }}</p><p>当前为界面样式预览，功能操作将在后续接入。</p><template #footer><el-button type="primary" @click="selected = null">知道了</el-button></template></el-dialog>
  </section>
</template>

<style scoped>
.feature-page { max-width: 1040px; margin: 0 auto; }
.feature-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 22px; }
.feature-eyebrow { color: var(--muted); font-size: 10px; letter-spacing: 1px; }
.feature-heading h1 { margin: 7px 0 0; font-size: 23px; font-weight: 600; }
.design-tag { padding: 5px 9px; border: 1px solid var(--line); border-radius: 5px; color: var(--slate); font-size: 10px; }
.feature-hero { position: relative; display: flex; align-items: center; justify-content: space-between; min-height: 222px; padding: 30px; overflow: hidden; border: 1px solid var(--hero-amber-line); border-radius: 14px; background: var(--feature-hero-amber); }
.amber { --feature-color: var(--feature-amber); --feature-rgb: var(--feature-amber-rgb); }
.mint { --feature-color: var(--feature-mint); --feature-rgb: var(--feature-mint-rgb); }
.blue { --feature-color: var(--feature-blue); --feature-rgb: var(--feature-blue-rgb); }
.feature-hero.mint { border-color: var(--hero-mint-line); background: var(--feature-hero-mint); }
.feature-hero.blue { border-color: var(--hero-blue-line); background: var(--feature-hero-blue); }
.feature-copy { position: relative; z-index: 1; max-width: 68%; }
.feature-copy h2 { margin: 0 0 12px; font-size: 24px; font-weight: 500; line-height: 1.5; }
.feature-copy p { max-width: 355px; margin: 0; color: var(--slate); font-size: 12px; line-height: 1.9; }
.feature-primary { display: flex; align-items: center; gap: 10px; margin-top: 25px; padding: 11px 18px; border: 0; border-radius: 8px; background: var(--feature-color); color: #202622; font-weight: 600; font-size: 13px; cursor: pointer; }
.feature-primary:hover { filter: brightness(1.08); }
.feature-primary .el-icon:last-child { margin-left: 15px; }
.feature-copy > small { display: block; margin-top: 11px; color: var(--muted); font-size: 10px; }
.feature-art { position: absolute; right: 32px; top: 50%; display: grid; place-items: center; width: 192px; height: 192px; transform: translateY(-50%); color: var(--feature-color); }
.orbit { position: absolute; border: 1px solid rgba(var(--feature-rgb), .13); border-radius: 50%; }
.orbit-one { inset: 0; }.orbit-two { inset: 20px; }
.art-core { display: grid; place-items: center; width: 110px; height: 110px; border: 1px solid rgba(var(--feature-rgb), .35); border-radius: 30px; background: rgba(var(--feature-rgb), .07); transform: rotate(-8deg); }
.art-core > * { transform: rotate(8deg); }.art-core svg { width: 66px; height: 66px; }
.art-dot { position: absolute; top: 29px; right: 24px; width: 7px; height: 7px; border-radius: 50%; background: var(--feature-color); box-shadow: 0 0 0 5px rgba(var(--feature-rgb), .08); }
.feature-section-title { display: flex; align-items: center; justify-content: space-between; margin: 25px 0 13px; }.feature-section-title h3 { margin: 0; font-size: 13px; font-weight: 500; }.feature-section-title > span { font-size: 10px; color: var(--muted); }
.feature-cards { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.feature-card { position: relative; padding: 18px; border: 1px solid var(--line); border-radius: 11px; background: var(--card-bg); color: var(--ink); text-align: left; cursor: pointer; }.feature-card:hover, .tool-card:hover { border-color: var(--card-hover-line); background: var(--card-hover); }
.feature-card-icon { display: grid; place-items: center; width: 36px; height: 36px; margin-bottom: 14px; border-radius: 10px; color: var(--feature-color); background: rgba(var(--feature-rgb), .08); }
.feature-card strong { font-size: 12px; font-weight: 500; }.feature-card p { margin: 7px 0 0; font-size: 10px; line-height: 1.7; color: var(--muted); }.card-arrow { position: absolute; right: 18px; top: 29px; color: var(--muted); }
.feature-summary { display: flex; align-items: center; flex-wrap: wrap; gap: 20px; margin-top: 18px; color: var(--slate); font-size: 10px; }.summary-label { color: var(--muted); }.feature-summary > span { display: flex; align-items: center; gap: 6px; }.feature-summary i { width: 4px; height: 4px; border-radius: 50%; background: #727774; }.feature-summary small { margin-left: auto; color: var(--muted); font-size: 10px; }
.tools-intro { display: flex; align-items: center; gap: 18px; padding: 19px 24px; border: 1px solid var(--line); border-radius: 12px; background: var(--tools-intro-bg); }.tools-intro-icon { display: grid; place-items: center; width: 48px; height: 48px; border-radius: 13px; background: var(--accent-soft); color: var(--accent); }.tools-intro h2 { margin: 0 0 8px; font-size: 20px; font-weight: 500; }.tools-intro p { margin: 0; font-size: 11px; color: var(--slate); }.tool-count { display: grid; gap: 4px; margin-left: auto; color: var(--accent); font-size: 29px; }.tool-count small { font-size: 10px; color: var(--muted); }
.tools-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 13px; }.tool-card { position: relative; display: flex; align-items: center; gap: 15px; min-height: 88px; padding: 20px; border: 1px solid var(--line); border-radius: 11px; background: var(--card-bg); color: var(--ink); text-align: left; cursor: pointer; }.tool-icon { color: #c3b08e; }.tool-copy { display: grid; gap: 9px; }.tool-copy strong { font-size: 13px; font-weight: 500; }.tool-copy small { color: var(--muted); font-size: 10px; }.tool-category { position: absolute; right: 13px; top: 12px; font-size: 9px; color: #858783; }.tool-arrow { margin-left: auto; color: var(--muted); }.tools-note { margin: 16px 0 0; color: var(--muted); font-size: 10px; text-align: center; }
@media (max-width: 880px) { .feature-art { right: 10px; opacity: .55; transform: translateY(-50%) scale(.85); }.feature-copy { max-width: 72%; }.feature-copy h2 { font-size: 21px; }.feature-hero { padding: 24px; }.feature-card { padding: 14px; }.feature-summary { gap: 12px; } }
</style>
<style>
.feature-preview-dialog { --el-dialog-bg-color: #28292a; --el-text-color-primary: #f1f1ef; --el-text-color-regular: #a0a19e; border: 1px solid #414344; border-radius: 12px; }.feature-preview-dialog p { line-height: 1.8; font-size: 12px; }
</style>
