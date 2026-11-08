<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, FolderOpened, CopyDocument, Document } from '@element-plus/icons-vue'

// 数据源(main 进程直读 %ProgramData%\oneclassmesh\logs,经 preload 桥):
//   listLogs() -> { ok, files:[{ name, backup, size, mtimeMs }] }  文件序已排好:主文件 → 备份(新→旧)
//   readLog(name) -> { ok, text, size, mtimeMs } | { ok:false, error }
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
  const arr = text.value.split(/\r?\n/)
  if (arr.length && arr[arr.length - 1] === '') arr.pop()
  return arr
})
const totalPages = computed(() =>
  lines.value.length ? Math.ceil(lines.value.length / PAGE_SIZE) : 0
)
const pagedLines = computed(() => {
  const start = (pageNo.value - 1) * PAGE_SIZE
  return lines.value.slice(start, start + PAGE_SIZE)
})

// 分组展示:core / worker / launcher / 其他,组内保持 main 进程排好的顺序
const groups = computed(() => {
  const order = []
  const map = new Map()
  const keyOf = (name) => {
    if (name.startsWith('core.log')) return 'core · 核心进程'
    if (name.startsWith('worker.log')) return 'worker · 业务进程'
    if (name.startsWith('launcher.log')) return 'launcher · 守护进程'
    return '其他'
  }
  for (const f of files.value) {
    const key = keyOf(f.name)
    if (!map.has(key)) {
      const g = { key, files: [] }
      map.set(key, g)
      order.push(g)
    }
    map.get(key).files.push(f)
  }
  return order
})

const cMeta = computed(() => {
  if (readError.value) return ''
  const parts = []
  if (lines.value.length) parts.push(lines.value.length + ' 行')
  if (size.value) parts.push(fmtSize(size.value))
  if (mtime.value) parts.push('更新于 ' + fmtTime(mtime.value))
  return parts.join(' · ')
})

async function loadList() {
  listLoading.value = true
  try {
    const res = await window.ocm.listLogs()
    files.value = Array.isArray(res?.files) ? res.files : []
    if (!files.value.some((f) => f.name === selected.value)) {
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
    const res = await window.ocm.readLog(selected.value)
    if (res?.ok) {
      text.value = res.text || ''
      size.value = res.size ?? 0
      mtime.value = res.mtimeMs ?? 0
      pageNo.value = toTail ? totalPages.value || 1 : Math.min(pageNo.value, totalPages.value || 1)
    } else {
      text.value = ''
      size.value = 0
      mtime.value = 0
      readError.value = res?.error || '读取失败'
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
  await readFile(true) // 切换文件默认看最新(末尾)
}

async function refreshAll() {
  const wasTail = pageNo.value >= totalPages.value
  await loadList()
  if (selected.value) await readFile(wasTail) // 本来就在末页则跟随到最新
}

async function openDir() {
  const res = await window.ocm.openLogs()
  if (res && res.ok === false && res.error) {
    ElMessage.error('打开日志目录失败:' + res.error)
  }
}

async function copyText(t) {
  try {
    await navigator.clipboard.writeText(t)
    return true
  } catch (_) {
    try {
      const ta = document.createElement('textarea')
      ta.value = t
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      ta.remove()
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
  else ElMessage.error('复制失败,请手动选择复制')
}

// 翻页后回到页首;在末页(最新)时贴底
watch(pageNo, async () => {
  await nextTick()
  const el = bodyEl.value
  if (!el) return
  el.scrollTop = pageNo.value >= totalPages.value ? el.scrollHeight : 0
})

onMounted(async () => {
  await loadList()
  if (selected.value) await readFile(true)
})

function fmtSize(n) {
  if (n == null) return '—'
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1024 / 1024).toFixed(2) + ' MB'
}

function fmtTime(ms) {
  if (!ms) return '—'
  const diff = Date.now() - ms
  if (diff < 60e3) return '刚刚'
  if (diff < 3600e3) return Math.floor(diff / 60e3) + ' 分钟前'
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + ' 小时前'
  const d = new Date(ms)
  const p = (x) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
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
  <div class="log-page">
    <div class="head">
      <div class="head-text">
        <h1 class="title">运行日志</h1>
        <div class="sub">
          核心与业务进程的落盘日志(%ProgramData%\oneclassmesh\logs),仅本机读取;排障时可复制全文发给支持人员。
        </div>
      </div>
      <div class="head-actions">
        <button class="btn-ghost" @click="refreshAll">
          <el-icon :size="14"><Refresh /></el-icon>刷新
        </button>
        <button class="btn-ghost" @click="openDir">
          <el-icon :size="14"><FolderOpened /></el-icon>打开日志目录
        </button>
      </div>
    </div>

    <div class="board" v-if="files.length || listLoading">
      <aside class="files" v-loading="listLoading && !files.length">
        <template v-if="groups.length">
          <div v-for="g in groups" :key="g.key" class="fgroup">
            <div class="gtitle">{{ g.key }}</div>
            <div
              v-for="f in g.files"
              :key="f.name"
              class="fitem"
              :class="{ active: f.name === selected, backup: f.backup }"
              :title="new Date(f.mtimeMs).toLocaleString()"
              @click="select(f.name)"
            >
              <div class="fname">
                <span v-if="f.backup" class="bmark">└</span>{{ f.name
                }}<span v-if="f.backup" class="bbadge">备份</span>
              </div>
              <div class="fmeta">{{ fmtSize(f.size) }} · {{ fmtTime(f.mtimeMs) }}</div>
            </div>
          </div>
        </template>
        <div v-else class="panel-hint">正在读取日志列表…</div>
      </aside>

      <section class="console" v-loading="readLoading">
        <div class="c-head">
          <el-icon :size="15" class="c-icon"><Document /></el-icon>
          <div class="c-file">{{ selected || '未选择文件' }}</div>
          <div class="c-meta">{{ cMeta }}</div>
          <button class="mini" :disabled="!text" @click="copyAll">
            <el-icon :size="12"><CopyDocument /></el-icon>复制全文
          </button>
        </div>
        <div class="c-body" ref="bodyEl">
          <div v-if="readError" class="c-error">读取失败:{{ readError }}</div>
          <div v-else-if="!pagedLines.length" class="c-empty">该文件暂无内容</div>
          <template v-else>
            <div
              v-for="(ln, i) in pagedLines"
              :key="pageNo + '-' + i"
              class="ln"
              :class="levelOf(ln)"
            >{{ ln || ' ' }}</div>
          </template>
        </div>
        <div class="c-foot" v-if="totalPages > 0">
          <button class="mini" :disabled="pageNo <= 1" @click="pageNo = 1">« 首页</button>
          <button class="mini" :disabled="pageNo <= 1" @click="pageNo--">‹ 上一页</button>
          <span class="pinfo">第 {{ pageNo }} / {{ totalPages }} 页 · 每页 {{ PAGE_SIZE }} 行</span>
          <button class="mini" :disabled="pageNo >= totalPages" @click="pageNo++">下一页 ›</button>
          <button class="mini" :disabled="pageNo >= totalPages" @click="pageNo = totalPages">
            最新 »
          </button>
        </div>
      </section>
    </div>

    <div class="empty" v-else>
      <div class="e-icon">📜</div>
      <div class="e-title">还没有日志文件</div>
      <div class="e-sub">核心进程首次运行后,会在日志目录生成 core.log / worker.log 等文件。</div>
      <button class="btn-ghost" @click="openDir">
        <el-icon :size="14"><FolderOpened /></el-icon>打开日志目录
      </button>
    </div>
  </div>
</template>

<style scoped>
.log-page {
  position: relative;
  z-index: 1;
  height: calc(100vh - 96px);
  min-height: 420px;
  display: flex;
  flex-direction: column;
}
.head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex: none;
}
.title {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 700;
  color: var(--text-strong);
  letter-spacing: 0.5px;
}
.sub {
  font-size: 12.5px;
  color: var(--text-mid);
}
.head-actions {
  display: flex;
  gap: 10px;
  flex: none;
  padding-bottom: 2px;
}
.btn-ghost {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 9px 16px;
  font-size: 13px;
  color: var(--text-mid);
  background: rgba(255, 255, 255, 0.75);
  border: 1px solid #cfe4f8;
  border-radius: 20px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-ghost:hover {
  color: var(--primary-deep);
  border-color: var(--primary);
  background: #fff;
}

.board {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 14px;
  margin-top: 16px;
}

/* 左侧文件列表 */
.files {
  width: 224px;
  flex: none;
  overflow-y: auto;
  background: var(--card);
  border-radius: 16px;
  box-shadow: 0 6px 22px rgba(44, 49, 68, 0.08);
  padding: 10px;
  box-sizing: border-box;
}
.fgroup + .fgroup {
  margin-top: 10px;
}
.gtitle {
  font-size: 11px;
  color: var(--muted);
  letter-spacing: 0.5px;
  padding: 4px 8px 6px;
  user-select: none;
}
.fitem {
  padding: 7px 10px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.12s ease;
}
.fitem:hover {
  background: #f2f9ff;
}
.fitem.active {
  background: rgba(61, 164, 255, 0.12);
  box-shadow: inset 3px 0 0 var(--primary);
}
.fname {
  font-family: Consolas, ui-monospace, 'Courier New', monospace;
  font-size: 12.5px;
  color: var(--text-strong);
  display: flex;
  align-items: center;
  gap: 5px;
  word-break: break-all;
}
.fitem.backup .fname {
  color: var(--text-mid);
  font-size: 12px;
}
.bmark {
  color: var(--muted);
}
.bbadge {
  flex: none;
  font-size: 10px;
  font-family: 'Microsoft YaHei UI', system-ui, sans-serif;
  color: var(--muted);
  background: #f0f4fa;
  border-radius: 6px;
  padding: 1px 5px;
}
.fmeta {
  font-size: 11px;
  color: var(--muted);
  margin-top: 2px;
}
.panel-hint {
  font-size: 12px;
  color: var(--muted);
  text-align: center;
  padding: 24px 8px;
}

/* 右侧深色控制台 */
.console {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: linear-gradient(165deg, #2b3143, #232936);
  border-radius: 16px;
  box-shadow: 0 10px 28px rgba(44, 49, 68, 0.25);
  overflow: hidden;
}
.c-head {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}
.c-icon {
  color: rgba(255, 255, 255, 0.5);
  flex: none;
}
.c-file {
  font-family: Consolas, ui-monospace, 'Courier New', monospace;
  font-size: 13px;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.c-meta {
  flex: 1;
  min-width: 0;
  font-size: 11.5px;
  color: rgba(255, 255, 255, 0.45);
  text-align: right;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.mini {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.08);
  border: none;
  border-radius: 8px;
  padding: 5px 11px;
  cursor: pointer;
  transition: background 0.12s ease;
}
.mini:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.16);
}
.mini:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.c-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 14px;
  font-family: Consolas, ui-monospace, 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-all;
}
.ln {
  color: #c6d2e2;
}
.ln.err {
  color: #ff8585;
}
.ln.warn {
  color: #ffc66d;
}
.c-error {
  color: #ff8585;
  padding: 24px 4px;
  text-align: center;
}
.c-empty {
  color: rgba(255, 255, 255, 0.4);
  padding: 24px 4px;
  text-align: center;
}
.c-foot {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
.pinfo {
  font-size: 11.5px;
  color: rgba(255, 255, 255, 0.5);
  min-width: 150px;
  text-align: center;
}

/* 空状态 */
.empty {
  flex: 1;
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 9px;
  background: var(--card);
  border-radius: 16px;
  box-shadow: 0 6px 22px rgba(44, 49, 68, 0.08);
}
.e-icon {
  width: 54px;
  height: 54px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 22px;
  background: #f2f9ff;
  box-shadow: 0 4px 14px rgba(44, 49, 68, 0.1);
}
.e-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-strong);
}
.e-sub {
  font-size: 12.5px;
  color: var(--muted);
  margin-bottom: 6px;
}

/* 细滚动条 */
.files::-webkit-scrollbar,
.c-body::-webkit-scrollbar {
  width: 8px;
}
.files::-webkit-scrollbar-thumb {
  background: #d7e4f4;
  border-radius: 4px;
}
.c-body::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.18);
  border-radius: 4px;
}
</style>
