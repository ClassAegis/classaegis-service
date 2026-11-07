<script setup>
import { ref } from 'vue'
import StatusView from './views/StatusView.vue'
import AboutView from './views/AboutView.vue'

const page = ref('status')
</script>

<template>
  <el-container style="height: 100vh">
    <el-aside width="200px" class="aside">
      <div class="brand">
        <div class="logo">壹课管家</div>
        <div class="sub">OneClassMesh Agent</div>
      </div>
      <el-menu :default-active="page" @select="(k) => (page = k)">
        <el-menu-item index="status">状态仪表</el-menu-item>
        <el-menu-item index="about">关于</el-menu-item>
      </el-menu>
    </el-aside>
    <el-main class="main">
      <StatusView v-if="page === 'status'" />
      <AboutView v-else />
    </el-main>
  </el-container>
</template>

<style>
body {
  margin: 0;
  font-family: 'Microsoft YaHei', sans-serif;
}
.aside {
  border-right: 1px solid #e4e7ed;
}
.brand {
  padding: 18px 16px;
}
.logo {
  font-size: 18px;
  font-weight: 600;
}
.sub {
  font-size: 12px;
  color: #909399;
}
.main {
  background: #f6f8fa;
}
</style>
