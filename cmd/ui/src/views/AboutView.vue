<script setup>
import { onMounted, ref } from 'vue'
import { Connection, Document, InfoFilled, Monitor } from '@element-plus/icons-vue'

const info = ref({ name: '', version: '', protocol: '', configPath: '' })

onMounted(async () => {
  info.value = await window.ocm.getAppInfo()
})
</script>

<template>
  <div class="about-page">
    <section class="about-heading">
      <span class="eyebrow">产品与组件</span>
      <h1>关于课佑</h1>
      <p>一套部署在校园内网中的终端守护与互联能力。</p>
    </section>

    <section class="identity-band">
      <div class="identity-mark" aria-hidden="true">
        <svg viewBox="0 0 48 48" fill="none">
          <path d="M24 5.5 39 11v10.8c0 9.7-5.8 16.8-15 21.2-9.2-4.4-15-11.5-15-21.2V11l15-5.5Z" fill="currentColor" opacity=".16" />
          <path d="M24 8.5 36 13v8.7c0 7.9-4.4 13.8-12 17.6-7.6-3.8-12-9.7-12-17.6V13l12-4.5Z" stroke="currentColor" stroke-width="2.2" />
          <path d="M16.5 24.5h6.1l4.4-6 4.5 5.8h.9" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
          <circle cx="16.5" cy="24.5" r="2.1" fill="currentColor" /><circle cx="27" cy="18.5" r="2.1" fill="currentColor" /><circle cx="32.4" cy="24.3" r="2.1" fill="currentColor" />
        </svg>
      </div>
      <div class="identity-copy"><span class="section-label">ClassAegis</span><h2>{{ info.name || 'ClassAegisService' }}</h2><p>校园内网一体化终端代理</p></div>
      <div class="identity-version"><span>当前版本</span><strong>{{ info.version || '0.1.0' }}</strong></div>
    </section>

    <section class="about-layout">
      <div class="about-panel version-panel">
        <div class="panel-heading"><div><span class="section-label">构建信息</span><h2>当前构建</h2></div><div class="panel-icon"><el-icon><InfoFilled /></el-icon></div></div>
        <dl class="info-list">
          <div><dt>应用版本</dt><dd>{{ info.version || '—' }}</dd></div>
          <div><dt>协议版本</dt><dd>{{ info.protocol ? `v${info.protocol}` : '—' }}</dd></div>
          <div><dt>授权协议</dt><dd>Apache License 2.0</dd></div>
        </dl>
      </div>

      <div class="about-panel boundary-panel">
        <div class="panel-heading"><div><span class="section-label">本机架构</span><h2>三个运行层</h2></div><div class="panel-icon"><el-icon><Connection /></el-icon></div></div>
        <div class="boundary-list">
          <div class="boundary-row"><span class="boundary-node teal"><el-icon><Connection /></el-icon></span><span><strong>ClassAegisCore</strong><small>MQTT 连接、验签与回执</small></span></div>
          <div class="boundary-row"><span class="boundary-node amber"><el-icon><Monitor /></el-icon></span><span><strong>ClassAegisAbility</strong><small>执行本地业务能力</small></span></div>
          <div class="boundary-row"><span class="boundary-node slate"><el-icon><Document /></el-icon></span><span><strong>ClassAegisService</strong><small>仅访问 127.0.0.1 环回接口</small></span></div>
        </div>
      </div>
    </section>

    <section class="config-panel"><div><span class="section-label">运行配置</span><h2>配置文件路径</h2></div><code>{{ info.configPath || '未找到配置文件' }}</code></section>
    <p class="about-note">课佑把连接、执行和显示分开管理。你的数据留在校园内网，桌面界面只负责呈现本机状态。</p>
  </div>
</template>

<style scoped>
.about-page { max-width: 1000px; margin: 0 auto; }
.about-heading { margin-bottom: 25px; }
.eyebrow, .section-label { color: var(--accent); font-size: 10px; font-weight: 800; text-transform: uppercase; }
.about-heading h1 { margin: 7px 0 6px; font-size: 29px; line-height: 1.15; }
.about-heading p { margin: 0; color: var(--muted); font-size: 13px; }
.identity-band { display: flex; align-items: center; gap: 15px; min-height: 104px; margin-bottom: 15px; padding: 19px 22px; border: 1px solid var(--line); border-left: 4px solid var(--accent); background: var(--surface); box-shadow: var(--shadow); }
.identity-mark { display: grid; place-items: center; width: 58px; height: 58px; flex: 0 0 58px; border-radius: 12px; background: var(--accent-soft); color: var(--accent-deep); }
.identity-mark svg { width: 43px; height: 43px; }
.identity-copy { display: grid; gap: 4px; }
.identity-copy h2 { margin: 0; font-size: 19px; }
.identity-copy p { margin: 0; color: var(--muted); font-size: 11px; }
.identity-version { display: grid; gap: 4px; min-width: 92px; margin-left: auto; padding-left: 22px; border-left: 1px solid var(--line); text-align: right; }
.identity-version span { color: var(--muted); font-size: 10px; }
.identity-version strong { color: var(--ink-soft); font-family: Consolas, 'Cascadia Mono', monospace; font-size: 12px; }
.about-layout { display: grid; grid-template-columns: .85fr 1.15fr; gap: 15px; }
.about-panel, .config-panel { border: 1px solid var(--line); background: var(--surface); box-shadow: var(--shadow); }
.about-panel { min-width: 0; padding: 18px 19px 10px; }
.panel-heading { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 13px; }
.panel-heading h2, .config-panel h2 { margin: 5px 0 0; color: var(--ink); font-size: 15px; }
.panel-icon { display: grid; place-items: center; width: 29px; height: 29px; border-radius: 6px; background: var(--accent-soft); color: var(--accent-deep); }
.info-list { margin: 0; border-top: 1px solid var(--line); }
.info-list > div { display: flex; align-items: center; justify-content: space-between; min-height: 42px; border-bottom: 1px solid var(--line); }
.info-list dt { color: var(--muted); font-size: 11px; }
.info-list dd { margin: 0; color: var(--ink-soft); font-size: 11px; font-weight: 700; }
.boundary-list { display: grid; border-top: 1px solid var(--line); }
.boundary-row { display: flex; align-items: center; gap: 10px; min-height: 46px; border-bottom: 1px solid var(--line); }
.boundary-row > span:last-child { display: grid; gap: 3px; }
.boundary-row strong { color: var(--ink-soft); font-family: Consolas, 'Cascadia Mono', monospace; font-size: 11px; }
.boundary-row small { color: var(--muted); font-size: 10px; }
.boundary-node { display: grid; place-items: center; width: 27px; height: 27px; flex: 0 0 27px; border-radius: 6px; }
.boundary-node.teal { background: var(--accent-soft); color: var(--accent-deep); }
.boundary-node.amber { background: var(--amber-soft); color: var(--amber); }
.boundary-node.slate { background: #edf1f0; color: #607980; }
.config-panel { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-top: 15px; padding: 16px 19px; }
.config-panel code { max-width: 65%; overflow: hidden; color: var(--muted); font-family: Consolas, 'Cascadia Mono', monospace; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.about-note { margin: 17px 0 0; color: var(--muted); font-size: 11px; line-height: 1.7; }
@media (max-width: 720px) { .about-layout { grid-template-columns: 1fr; } }
@media (max-width: 520px) { .identity-band { align-items: flex-start; flex-wrap: wrap; } .identity-version { width: 100%; margin-left: 0; padding: 11px 0 0; border-top: 1px solid var(--line); border-left: 0; text-align: left; } .config-panel { align-items: flex-start; flex-direction: column; gap: 10px; } .config-panel code { max-width: 100%; } }
</style>
