#!/usr/bin/env node
// gen-icon-main.js — OneClassMesh 壹课管家 应用图标生成器
//
// 纯 Node + Electron,无图像库依赖:
//   隐藏 Electron 窗口渲染内联 SVG → webContents.capturePage() 抓图
//   → nativeImage.resize 下采样 → toPNG / 手工打包多尺寸 .ico
//
// 用法(在 cmd/ui 目录下): npx electron build/gen-icon-main.js
//
// 产物:
//   build/icon.ico                  多尺寸 Windows 图标(16/24/32/48/64/128/256)
//   build/icons/{N}x{N}.png         16/24/32/48/64/128/256/512 PNG(electron-builder Linux 约定)
//   build/icons/icon.png            512×512 主图标
//   electron/assets/tray-32.png     托盘图标 32×32(electron/main.js 引用)
//   electron/assets/tray-32@2x.png  托盘图标 64×64(高分屏下 Electron 自动加载 @2x)

'use strict'

const { app, BrowserWindow, nativeImage } = require('electron')
const path = require('path')
const fs = require('fs')

// ---- 必须在 app.whenReady 之前调用,保证软件渲染下抓图行为一致 ----
app.disableHardwareAcceleration()

const UI_ROOT = path.join(__dirname, '..') // cmd/ui
const ICONS_DIR = path.join(UI_ROOT, 'build', 'icons')
const ICO_PATH = path.join(UI_ROOT, 'build', 'icon.ico')
const TRAY_DIR = path.join(UI_ROOT, 'electron', 'assets')

const BASE_SIZE = 512
const PNG_SIZES = [512, 256, 128, 64, 48, 32, 24, 16]
// ICO 帧尺寸(256→16):256 采用 PNG 压缩帧,其余采用 32bpp BMP 帧,兼容性最好
const ICO_SIZES = [256, 128, 64, 48, 32, 24, 16]
const TRAY_SIZES = [32, 64] // tray-32.png 与 tray-32@2x.png

// ---- 图标设计:圆角方块磁贴 + 对角渐变 + 白色盾牌 + 蓝色对勾 ----
const SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512">
<defs>
  <linearGradient id="tile" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="#3da4ff"/>
    <stop offset="1" stop-color="#2196f3"/>
  </linearGradient>
</defs>
<rect x="24" y="24" width="464" height="464" rx="104" fill="url(#tile)"/>
<path d="M176,128 H336 C336,246 302,332 256,392 C210,332 176,246 176,128 Z" fill="#ffffff"/>
<path d="M204,260 L236,292 L308,208" fill="none" stroke="#2196f3" stroke-width="26" stroke-linecap="round" stroke-linejoin="round"/>
</svg>`

const HTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<style>
  html, body { margin: 0; padding: 0; width: 512px; height: 512px; background: transparent; overflow: hidden; }
  svg { display: block; }
</style>
</head>
<body>${SVG}</body>
</html>`

const delay = (ms) => new Promise((r) => setTimeout(r, ms))
const resizeTo = (img, size) => img.resize({ width: size, height: size, quality: 'best' })

// ---- 抓图 ----

function hasInk(img) {
  // 位图中是否存在任何非透明像素(防止抓到空白帧)
  const bmp = img.toBitmap()
  for (let i = 3; i < bmp.length; i += 4) {
    if (bmp[i] !== 0) return true
  }
  return false
}

async function captureInk(win) {
  const wc = win.webContents
  for (let attempt = 1; attempt <= 3; attempt++) {
    const img = await wc.capturePage(undefined, { stayHidden: true })
    if (img && !img.isEmpty() && hasInk(img)) return img
    await delay(200 * attempt)
  }
  // 兜底:短暂显示(窗口置于屏幕外)再抓一次
  win.setPosition(-2000, 0)
  win.show()
  await delay(300)
  const img = await wc.capturePage(undefined, {})
  win.hide()
  if (img && !img.isEmpty() && hasInk(img)) return img
  throw new Error('capturePage 未取到有效图像(空白帧)')
}

// 抓图结果可能带 devicePixelRatio(如 1.5x 缩放屏上是 768 物理 px)。
// 统一重建为 scaleFactor=1 的原生像素图,保证后续所有尺寸精确可控。
function toScaleOne(img) {
  const bmp = img.toBitmap()
  const px = Math.round(Math.sqrt(bmp.length / 4))
  if (!Number.isInteger(px) || px <= 0) {
    throw new Error('位图尺寸异常,长度=' + bmp.length)
  }
  return nativeImage.createFromBitmap(bmp, { width: px, height: px, scaleFactor: 1 })
}

// ---- 校验 ----

function assertPngSize(buf, expected) {
  const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])
  if (buf.length < 33 || !buf.subarray(0, 8).equals(sig)) {
    throw new Error('生成的 PNG 签名无效')
  }
  const w = buf.readUInt32BE(16)
  const h = buf.readUInt32BE(20)
  if (w !== expected || h !== expected) {
    throw new Error(`PNG 尺寸 ${w}x${h} != 期望 ${expected}x${expected}`)
  }
}

// ---- ICO 打包 ----

// 32bpp BMP 帧(BITMAPINFOHEADER + XOR 自下而上 BGRA + AND 掩码)。
// toBitmap 返回预乘 alpha 的 BGRA;Windows 图标帧要求直通 alpha,先做反预乘。
function bmpFrameFromBitmap(bgra, size) {
  const w = size
  const h = size
  const px = Buffer.from(bgra) // 拷贝,避免修改原图像
  for (let i = 0; i < px.length; i += 4) {
    const a = px[i + 3]
    if (a > 0 && a < 255) {
      const f = 255 / a
      px[i] = Math.min(255, Math.round(px[i] * f))
      px[i + 1] = Math.min(255, Math.round(px[i + 1] * f))
      px[i + 2] = Math.min(255, Math.round(px[i + 2] * f))
    }
  }
  const andStride = Math.ceil(w / 32) * 4 // 1bpp,按 32 位对齐
  const andSize = andStride * h
  const xorSize = w * h * 4
  const buf = Buffer.alloc(40 + xorSize + andSize)
  buf.writeUInt32LE(40, 0) // biSize
  buf.writeInt32LE(w, 4) // biWidth
  buf.writeInt32LE(h * 2, 8) // biHeight = XOR + AND 两段
  buf.writeUInt16LE(1, 12) // biPlanes
  buf.writeUInt16LE(32, 14) // biBitCount
  buf.writeUInt32LE(0, 16) // biCompression = BI_RGB
  buf.writeUInt32LE(xorSize + andSize, 20) // biSizeImage
  for (let y = 0; y < h; y++) {
    const src = (h - 1 - y) * w * 4 // DIB 自下而上
    px.copy(buf, 40 + y * w * 4, src, src + w * 4) // BGRA 无需换序
  }
  // AND 掩码保持全 0:32bpp 帧的透明度由 alpha 通道决定
  return buf
}

function buildIco(frames) {
  // frames: [{ size, data }] — data 为 PNG(256)或 32bpp BMP(其余)
  const header = Buffer.alloc(6)
  header.writeUInt16LE(0, 0) // reserved
  header.writeUInt16LE(1, 2) // type = icon
  header.writeUInt16LE(frames.length, 4)
  const entries = []
  let offset = 6 + 16 * frames.length
  for (const f of frames) {
    const e = Buffer.alloc(16)
    const s = f.size >= 256 ? 0 : f.size // 0 表示 256
    e[0] = s
    e[1] = s
    e[2] = 0 // 调色板色数
    e[3] = 0 // reserved
    e.writeUInt16LE(1, 4) // planes
    e.writeUInt16LE(32, 6) // bit count
    e.writeUInt32LE(f.data.length, 8) // bytes in resource
    e.writeUInt32LE(offset, 12) // image offset
    offset += f.data.length
    entries.push(e)
  }
  return Buffer.concat([header, ...entries, ...frames.map((f) => f.data)])
}

// ---- 主流程 ----

async function main() {
  console.log('[gen-icon] Electron', process.versions.electron, '/ Node', process.versions.node)
  fs.mkdirSync(ICONS_DIR, { recursive: true })
  fs.mkdirSync(TRAY_DIR, { recursive: true })

  const win = new BrowserWindow({
    show: false,
    width: BASE_SIZE,
    height: BASE_SIZE,
    useContentSize: true, // 512 为纯内容区,不受边框影响
    frame: false,
    transparent: true, // 磁贴圆角外保留透明
    backgroundColor: '#00000000',
    resizable: false,
    skipTaskbar: true,
    webPreferences: {
      backgroundThrottling: false, // 隐藏窗口仍持续绘制
    },
  })

  await win.loadURL('data:text/html,' + encodeURIComponent(HTML))
  // 等渲染器栅格化 SVG:双 rAF + 固定缓冲
  await Promise.race([
    win.webContents
      .executeJavaScript(
        'new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)))',
        true
      )
      .catch(() => {}),
    delay(800),
  ])
  await delay(150)

  const captured = await captureInk(win)
  const base = toScaleOne(captured)
  const baseSize = base.getSize()
  console.log(`[gen-icon] capture → ${baseSize.width}x${baseSize.height}`)

  const cornerAlpha = base.toBitmap()[3]
  if (cornerAlpha !== 0) {
    console.warn(`[gen-icon] 警告: 角像素 alpha=${cornerAlpha}(期望 0,透明背景可能未生效)`)
  }

  const written = []

  // PNG 资产(electron-builder Linux 约定 {N}x{N}.png + 主图标 icon.png)
  for (const size of PNG_SIZES) {
    const png = resizeTo(base, size).toPNG()
    assertPngSize(png, size)
    const name = `${size}x${size}.png`
    fs.writeFileSync(path.join(ICONS_DIR, name), png)
    written.push(path.relative(UI_ROOT, path.join(ICONS_DIR, name)))
    if (size === BASE_SIZE) {
      fs.writeFileSync(path.join(ICONS_DIR, 'icon.png'), png)
      written.push(path.relative(UI_ROOT, path.join(ICONS_DIR, 'icon.png')))
    }
  }

  // 多尺寸 Windows ICO
  const frames = []
  for (const size of ICO_SIZES) {
    const resized = resizeTo(base, size)
    if (size >= 256) {
      frames.push({ size, data: resized.toPNG() })
    } else {
      const bmp = resized.toBitmap()
      if (bmp.length !== size * size * 4) {
        throw new Error(`toBitmap 尺寸异常: 期望 ${size * size * 4},实际 ${bmp.length}`)
      }
      frames.push({ size, data: bmpFrameFromBitmap(bmp, size) })
    }
  }
  fs.writeFileSync(ICO_PATH, buildIco(frames))
  written.push(path.relative(UI_ROOT, ICO_PATH))

  // 托盘图标(32 + 64,Electron 高分屏自动加载 @2x)
  for (const size of TRAY_SIZES) {
    const png = resizeTo(base, size).toPNG()
    assertPngSize(png, size)
    const name = size === 32 ? 'tray-32.png' : 'tray-32@2x.png'
    fs.writeFileSync(path.join(TRAY_DIR, name), png)
    written.push(path.relative(UI_ROOT, path.join(TRAY_DIR, name)))
  }

  console.log('[gen-icon] 生成完成:')
  for (const f of written) console.log('  -', f)
}

app.whenReady().then(() => {
  main()
    .then(() => app.exit(0))
    .catch((err) => {
      console.error('[gen-icon] 失败:', err)
      app.exit(1)
    })
})
