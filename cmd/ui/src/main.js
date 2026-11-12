import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import './appearance'
import './theme.css'
import { installDesktopBridge } from './bridge'

installDesktopBridge()
createApp(App).use(ElementPlus).mount('#app')
