<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument, Document, FolderOpened, Refresh } from '@element-plus/icons-vue'

const PAGE_SIZE = 500

const files = ref([])
const selected = ref('')
const text = ref('')
const size = ref(0)
const mtime = ref(0)
const listLoading = ref(false)
const readLoading = ref(false)
const readError = ref('')
const pageNo = ref(1)
const bodyEl = ref(null)

const lines = computed(() => {
  if (!text.value) return []
  const result = text.value.split(/\r?\n/)
  if (result[result.length - 1] === '') result.pop()
  return result
})

const totalPages = computed(() => (lines.value.length ? Math.ceil(lines.value.length / PAGE_SIZE) : 0))
const pagedLines = computed(() => {
  const start = (pageNo.value - 1) * PAGE_SIZE
  return lines.value.slice(start, start + PAGE_SIZE)
})
const selectedFile = computed(() => files.value.find((file) => file.name === selected.value))

const groups = computed(() => {
  const order = []
  const map = new Map()
  const keyOf = (name) => {
    if (name.startsWith('core.log')) return 'core · 核心进程'
    if (name.startsWith('ability.log')) return 'ability · 业务能力'
    if (name.startsWith('launcher.log')) return 'launcher · 守护进程'
    return '其他日志'
  }
  for (const file of files.value) {
    const key = keyOf(file.name)
    if (!map.has(key)) {
      const group = { key, files: [] }
      map.set(key, group)
      order.push(group)
    }
    map.get(key).files.push(file)
  }
  return order
})

const fileMeta = computed(() => {
  const parts = []
  if (lines.value.length) parts.push(`${lines.value.length} 行`)
  if (size.value) parts.push(fmtSize(size.value))
  if (mtime.value) parts.push(`更新于 ${fmtTime(mtime.value)}`)
  return parts.join(' · ')
})

async function loadList() {
  listLoading.value = true
  try {
    const result = await window.ocm.listLogs()
    files.value = Array.isArray(result?.files) ? result.files : []
    if (!files.value.some((file) => file.name === selected.value)) {
      selected.value = files.value.length ? files.value[0].name : ''
    }
  } catch (_) {
    files.value = []
  } finally {
    listLoading.value = false
  }
}

async function readFile(toTail) {
  if (!selected.value) {
    text.value = ''
    size.value = 0
    mtime.value = 0
    readError.value = ''
    return
  }
  readLoading.value = true
  readError.value = ''
  try {
    const result = await window.ocm.readLog(selected.value)
    if (result?.ok) {
      text.value = result.text || ''
      size.value = result.size ?? 0
      mtime.value = result.mtimeMs ?? 0
      pageNo.value = toTail ? totalPages.value || 1 : Math.min(pageNo.value, totalPages.value || 1)
    } else {
      text.value = ''
      size.value = 0
      mtime.value = 0
      readError.value = result?.error || '读取失败'
    }
  } catch (e) {
    text.value = ''
    readError.value = String((e && e.message) || e)
  } finally {
    readLoading.value = false
  }
}

async function select(name) {
  if (name === selected.value) return
  selected.value = name
  await readFile(true)
}

async function refreshAll() {
  const wasTail = pageNo.value >= totalPages.value
  await loadList()
  if (selected.value) await readFile(wasTail)
}

async function openDir() {
  const result = await window.ocm.openLogs()
  if (result && result.ok === false && result.error) ElMessage.error(`打开日志目录失败：${result.error}`)
}

async function copyText(value) {
  try {
    await navigator.clipboard.writeText(value)
    return true
  } catch (_) {
    try {
      const area = document.createElement('textarea')
      area.value = value
      area.style.position = 'fixed'
      area.style.opacity = '0'
      document.body.appendChild(area)
      area.select()
      const ok = document.execCommand('copy')
      area.remove()
      return ok
    } catch (_) {
      return false
    }
  }
}

async function copyAll() {
  if (!text.value) return
  const ok = await copyText(text.value)
  if (ok) ElMessage.success('已复制日志全文')
  else ElMessage.error('复制失败，请手动选择日志')
}

watch(pageNo, async () => {
  await nextTick()
  if (!bodyEl.value) return
  bodyEl.value.scrollTop = pageNo.value >= totalPages.value ? bodyEl.value.scrollHeight : 0
})

onMounted(async () => {
  await loadList()
  if (selected.value) await readFile(true)
})

function fmtSize(value) {
  if (value == null) return '—'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(2)} MB`
}

function fmtTime(value) {
  if (!value) return '—'
  const diff = Date.now() - value
  if (diff < 60e3) return '刚刚'
  if (diff < 3600e3) return `${Math.floor(diff / 60e3)} 分钟前`
  if (diff < 86400e3) return `${Math.floor(diff / 3600e3)} 小时前`
  const date = new Date(value)
  const pad = (item) => String(item).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const reErr = /(\bERROR\b|\bERRO\b|\bERR\b|\bFATAL\b|\bPANIC\b|\bfailed\b|\bTraceback\b)/i
const reWarn = /(\bWARN\b|\bWARNING\b)/i
function levelOf(line) {
  if (reErr.test(line)) return 'err'
  if (reWarn.test(line)) return 'warn'
  return ''
}
</script>

<template>
  <div class="logs-page">
    <section class="logs-heading">
      <div>
        <span class="eyebrow">诊断工具</span>
        <h1>运行日志</h1>
        <p>按进程查看本机日志，快速定位连接和执行问题。</p>
      </div>
      <div class="logs-actions">
        <button class="button button-secondary" type="button" title="重新读取日志列表" @click="refreshAll">
          <el-icon><Refresh /></el-icon>
          刷新
        </button>
        <button class="button button-secondary" type="button" title="在资源管理器中打开日志目录" @click="openDir">
          <el-icon><FolderOpened /></el-icon>
          打开目录
        </button>
      </div>
    </section>

    <div v-if="files.length || listLoading" class="log-workspace">
      <aside class="file-panel" v-loading="listLoading && !files.length">
        <div class="file-panel-head">
          <div>
            <span class="section-label">文件列表</span>
            <strong>日志文件</strong>
          </div>
          <span class="file-count">{{ files.length }}</span>
        </div>
        <div class="file-panel-note">主文件与轮转备份</div>

        <div class="file-groups">
          <div v-for="group in groups" :key="group.key" class="file-group">
            <div class="group-title">{{ group.key }}</div>
            <button
              v-for="file in group.files"
              :key="file.name"
              type="button"
              class="file-item"
              :class="{ active: file.name === selected, backup: file.backup }"
              :title="new Date(file.mtimeMs).toLocaleString()"
              @click="select(file.name)"
            >
              <span class="file-name"><span v-if="file.backup" class="backup-mark">└</span>{{ file.name }}</span>
              <span class="file-meta">{{ fmtSize(file.size) }} · {{ fmtTime(file.mtimeMs) }}</span>
            </button>
          </div>
        </div>
      </aside>

      <section class="viewer-panel" v-loading="readLoading">
        <header class="viewer-head">
          <div class="viewer-title">
            <span class="terminal-lights"><i></i><i></i><i></i></span>
            <el-icon><Document /></el-icon>
            <strong>{{ selected || '未选择文件' }}</strong>
          </div>
          <div class="viewer-tools">
            <span class="viewer-meta">{{ fileMeta }}</span>
            <button class="icon-button" type="button" title="复制全文" :disabled="!text" @click="copyAll">
              <el-icon><CopyDocument /></el-icon>
            </button>
          </div>
        </header>

        <div ref="bodyEl" class="viewer-body">
          <div v-if="readError" class="viewer-message error">读取失败：{{ readError }}</div>
          <div v-else-if="!pagedLines.length" class="viewer-message">该文件暂无内容</div>
          <template v-else>
            <div v-for="(line, index) in pagedLines" :key="`${pageNo}-${index}`" class="log-line" :class="levelOf(line)">
              <span class="line-number">{{ (pageNo - 1) * PAGE_SIZE + index + 1 }}</span>
              <span>{{ line || ' ' }}</span>
            </div>
          </template>
        </div>

        <footer v-if="totalPages > 0" class="viewer-foot">
          <button class="page-button" type="button" :disabled="pageNo <= 1" @click="pageNo = 1">首页</button>
          <button class="page-button" type="button" :disabled="pageNo <= 1" @click="pageNo--">上一页</button>
          <span>第 {{ pageNo }} / {{ totalPages }} 页</span>
          <button class="page-button" type="button" :disabled="pageNo >= totalPages" @click="pageNo++">下一页</button>
          <button class="page-button" type="button" :disabled="pageNo >= totalPages" @click="pageNo = totalPages">最新</button>
        </footer>
      </section>
    </div>

    <div v-else class="logs-empty">
      <div class="empty-icon"><el-icon><Document /></el-icon></div>
      <strong>暂无日志文件</strong>
      <span>核心进程启动后，日志会自动出现在本机目录。</span>
      <button class="button button-secondary" type="button" @click="openDir">
        <el-icon><FolderOpened /></el-icon>
        打开日志目录
      </button>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  max-width: 1280px;
  min-height: calc(100vh - 150px);
  margin: 0 auto;
  display: flex;
  flex-direction: column;
}

.logs-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}

.eyebrow,
.section-label {
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1.2px;
  text-transform: uppercase;
}

.logs-heading h1 {
  margin: 5px 0;
  color: var(--ink);
  font-size: 28px;
  line-height: 1.1;
}

.logs-heading p {
  margin: 0;
  color: var(--slate);
  font-size: 13px;
}

.logs-actions {
  display: flex;
  gap: 9px;
}

.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 36px;
  padding: 0 14px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--slate);
  cursor: pointer;
  font-size: 12px;
  font-weight: 700;
  transition: border-color 0.16s ease, color 0.16s ease, transform 0.16s ease;
}

.button:hover {
  border-color: #b8c8d7;
  color: var(--accent);
  transform: translateY(-1px);
}

.log-workspace {
  display: flex;
  min-height: 0;
  flex: 1;
  gap: 14px;
}

.file-panel,
.viewer-panel,
.logs-empty {
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow);
}

.file-panel {
  width: 250px;
  flex: 0 0 250px;
  min-height: 420px;
  overflow: auto;
  padding: 17px 10px 12px;
}

.file-panel-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  padding: 0 7px;
}

.file-panel-head > div {
  display: grid;
  gap: 5px;
}

.file-panel-head strong {
  color: var(--ink);
  font-size: 15px;
}

.file-count {
  display: grid;
  place-items: center;
  min-width: 22px;
  height: 22px;
  padding: 0 6px;
  border-radius: 11px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
}

.file-panel-note {
  margin: 10px 7px 14px;
  color: var(--muted);
  font-size: 11px;
}

.file-group + .file-group {
  margin-top: 13px;
}

.group-title {
  padding: 0 8px 6px;
  color: var(--muted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.4px;
}

.file-item {
  display: grid;
  gap: 4px;
  width: 100%;
  min-height: 49px;
  padding: 8px 9px;
  border: 0;
  border-left: 3px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: var(--ink);
  cursor: pointer;
  text-align: left;
}

.file-item:hover {
  background: #f4f7fa;
}

.file-item.active {
  border-left-color: var(--accent);
  background: var(--accent-soft);
}

.file-name {
  overflow: hidden;
  font-family: Consolas, 'Cascadia Mono', monospace;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-item.backup .file-name {
  color: var(--slate);
}

.backup-mark {
  margin-right: 4px;
  color: var(--muted);
}

.file-meta {
  color: var(--muted);
  font-size: 10px;
}

.viewer-panel {
  display: flex;
  min-width: 0;
  min-height: 420px;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
  background: #1b2631;
}

.viewer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 52px;
  gap: 14px;
  padding: 0 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.09);
}

.viewer-title,
.viewer-tools {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 9px;
}

.viewer-title {
  color: #dce6ee;
}

.viewer-title strong {
  overflow: hidden;
  font-family: Consolas, 'Cascadia Mono', monospace;
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-lights {
  display: flex;
  gap: 4px;
  margin-right: 3px;
}

.terminal-lights i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #6a7b8c;
}

.terminal-lights i:first-child {
  background: #d47b77;
}

.terminal-lights i:nth-child(2) {
  background: #d1aa63;
}

.terminal-lights i:last-child {
  background: #6fb18f;
}

.viewer-meta {
  overflow: hidden;
  color: #81919e;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.icon-button {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 5px;
  background: rgba(255, 255, 255, 0.06);
  color: #c6d4df;
  cursor: pointer;
}

.icon-button:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.13);
  color: #fff;
}

.icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}

.viewer-body {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 13px 14px 18px;
  font-family: Consolas, 'Cascadia Mono', monospace;
  font-size: 11px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
  color: #c7d2dc;
}

.log-line.warn {
  color: #e5c274;
}

.log-line.err {
  color: #f18c8c;
}

.line-number {
  padding-right: 10px;
  color: #657786;
  text-align: right;
  user-select: none;
}

.viewer-message {
  padding: 28px 6px;
  color: #8697a4;
  text-align: center;
}

.viewer-message.error {
  color: #f18c8c;
}

.viewer-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 47px;
  padding: 0 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.09);
  color: #82929f;
  font-size: 10px;
}

.page-button {
  min-height: 25px;
  padding: 0 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.05);
  color: #bdcbd6;
  cursor: pointer;
  font-size: 10px;
}

.page-button:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
}

.page-button:disabled {
  cursor: not-allowed;
  opacity: 0.3;
}

.logs-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 420px;
  flex: 1;
  flex-direction: column;
  gap: 9px;
}

.empty-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  margin-bottom: 3px;
  border-radius: 8px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 19px;
}

.logs-empty strong {
  color: var(--ink);
  font-size: 14px;
}

.logs-empty > span {
  margin-bottom: 7px;
  color: var(--muted);
  font-size: 12px;
}

.file-panel::-webkit-scrollbar,
.viewer-body::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.file-panel::-webkit-scrollbar-thumb {
  border-radius: 4px;
  background: #d3dce4;
}

.viewer-body::-webkit-scrollbar-thumb {
  border-radius: 4px;
  background: #425565;
}

@media (max-width: 820px) {
  .logs-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .log-workspace {
    flex-direction: column;
  }

  .file-panel {
    width: 100%;
    flex-basis: auto;
    min-height: 0;
    max-height: 220px;
  }

  .viewer-panel {
    min-height: 440px;
  }
}

@media (max-width: 500px) {
  .logs-actions {
    width: 100%;
  }

  .logs-actions .button {
    flex: 1;
  }

  .viewer-meta {
    display: none;
  }
}
</style>
