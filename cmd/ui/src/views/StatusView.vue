<script setup>
import { onMounted, onUnmounted, ref } from 'vue'

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

const fields = [
  { key: 'sn', label: '设备 SN' },
  { key: 'schoolCode', label: '学校代码' },
  { key: 'connected', label: 'MQTT 连接' },
  { key: 'workerAlive', label: '业务进程' },
  { key: 'agentVersion', label: 'Agent 版本' },
  { key: 'protocolVersion', label: '协议版本' },
]
</script>

<template>
  <div>
    <h3>状态仪表</h3>
    <el-alert
      v-if="error"
      :title="'核心进程未连接:' + error"
      type="warning"
      :closable="false"
      show-icon
    />
    <el-descriptions v-else-if="status" :column="1" border>
      <el-descriptions-item v-for="f in fields" :key="f.key" :label="f.label">
        <template v-if="f.key === 'connected'">
          <el-tag :type="status.connected ? 'success' : 'danger'">
            {{ status.connected ? '已连接' : '断开' }}
          </el-tag>
        </template>
        <template v-else-if="f.key === 'workerAlive'">
          <el-tag :type="status.workerAlive ? 'success' : 'danger'">
            {{ status.workerAlive ? '运行中' : '未运行' }}
          </el-tag>
        </template>
        <template v-else>{{ status[f.key] }}</template>
      </el-descriptions-item>
    </el-descriptions>
    <el-skeleton v-else :rows="5" animated />
  </div>
</template>
