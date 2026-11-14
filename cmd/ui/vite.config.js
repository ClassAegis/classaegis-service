// SPDX-License-Identifier: AGPL-3.0-only
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: './',
  server: {
    port: 1420,
    strictPort: true,
  },
  build: {
    outDir: process.env.VITE_OUT_DIR || 'dist-tauri',
  },
})
