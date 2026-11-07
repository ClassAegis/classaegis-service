<script setup>
import { onMounted, ref } from 'vue'

const info = ref({ name: '', version: '', protocol: '', configPath: '' })

onMounted(async () => {
  info.value = await window.ocm.getAppInfo()
})
</script>

<template>
  <div>
    <h3>关于</h3>
    <el-descriptions :column="1" border>
      <el-descriptions-item label="产品">{{ info.name }}</el-descriptions-item>
      <el-descriptions-item label="版本">{{ info.version }}</el-descriptions-item>
      <el-descriptions-item label="协议版本">{{ info.protocol }}</el-descriptions-item>
    </el-descriptions>
  </div>
</template>
