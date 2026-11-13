<script setup>
import { onMounted, onBeforeUnmount, ref } from 'vue'
import splashUrl from '../assets/splash.png'

const emit = defineEmits(['done'])
const canvasRef = ref(null)
const uiOut = ref(false)
const uiIn = ref(false)

let ctx = null
let raf = 0
let W = 0
let H = 0
let t0 = 0
let finished = false
let buckets = []
let cx = 0
let cy = 0

const APPEAR = 700
const SCATTER = 1800
const FINISH = 3520
const easeOut = (x) => 1 - Math.pow(1 - x, 3)

onMounted(async () => {
  resize()
  await build()
  t0 = performance.now()
  raf = requestAnimationFrame(loop)
  window.addEventListener('resize', onResize)
  setTimeout(() => (uiIn.value = true), 300)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', onResize)
  buckets = []
})

function onResize() {
  resize()
}

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  W = window.innerWidth
  H = window.innerHeight
  canvas.width = Math.floor(W * dpr)
  canvas.height = Math.floor(H * dpr)
  canvas.style.width = W + 'px'
  canvas.style.height = H + 'px'
  ctx = canvas.getContext('2d')
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  cx = W / 2
  cy = H / 2
}

// 把图片拆成一团细粒子：先找出有效内容范围（裁掉大片黑边），
// 再按亮度概率撒点，亮的地方密、暗的地方稀。
async function build() {
  const img = new Image()
  img.src = splashUrl
  try {
    await img.decode()
  } catch {
    return
  }

  const scanW = 220
  const scanH = Math.max(1, Math.round((scanW * img.height) / img.width))

  const off = document.createElement('canvas')
  off.width = scanW
  off.height = scanH
  const octx = off.getContext('2d', { willReadFrequently: true })
  octx.drawImage(img, 0, 0, scanW, scanH)
  const data = octx.getImageData(0, 0, scanW, scanH).data

  const lumAt = (x, y) => {
    const i = (y * scanW + x) * 4
    return {
      r: data[i],
      g: data[i + 1],
      b: data[i + 2],
      a: data[i + 3],
      l: 0.299 * data[i] + 0.587 * data[i + 1] + 0.114 * data[i + 2],
    }
  }

  // 有效内容包围盒：按行/列统计亮像素，躲开零散背景噪点
  const LUM = 30
  const rowCnt = new Array(scanH).fill(0)
  const colCnt = new Array(scanW).fill(0)
  for (let y = 0; y < scanH; y++) {
    for (let x = 0; x < scanW; x++) {
      const c = lumAt(x, y)
      if (c.a < 40 || c.l < LUM) continue
      rowCnt[y]++
      colCnt[x]++
    }
  }
  let minY = -1
  let maxY = -1
  let minX = -1
  let maxX = -1
  for (let y = 0; y < scanH; y++) {
    if (rowCnt[y] >= 2) {
      if (minY < 0) minY = y
      maxY = y
    }
  }
  for (let x = 0; x < scanW; x++) {
    if (colCnt[x] >= 2) {
      if (minX < 0) minX = x
      maxX = x
    }
  }
  if (minX < 0 || minY < 0 || maxX <= minX || maxY <= minY) {
    return
  }
  const bw = maxX - minX + 1
  const bh = maxY - minY + 1

  // 铺满画面中部的尺寸
  let dh = Math.min(H * 0.72, 680)
  let scale = dh / bh
  let dw = bw * scale
  const maxW = Math.min(W * 0.74, 760)
  if (dw > maxW) {
    scale = maxW / bw
    dw = maxW
    dh = bh * scale
  }
  const originX = (W - dw) / 2
  const originY = (H - dh) / 2 - H * 0.02

  // 收集粒子（先按图片坐标记，最后按亮度质心做水平对齐）
  const raw = []
  const total = 20000
  let tries = 0
  const maxTries = total * 8

  while (raw.length < total && tries < maxTries) {
    tries++
    const u = Math.random()
    const v = Math.random()
    const sx = minX + Math.min(bw - 1, Math.floor(u * bw))
    const sy = minY + Math.min(bh - 1, Math.floor(v * bh))
    const c = lumAt(sx, sy)
    if (c.a < 40) continue

    // 概率与亮度相关：暗处稀疏，亮处密集
    let prob = c.l / 205
    // 边界处淡出，避免硬切边
    const edge = Math.min(
      (sx - minX) / 13,
      (maxX - sx) / 13,
      (sy - minY) / 13,
      (maxY - sy) / 13,
      1,
    )
    prob *= Math.max(0, Math.min(1, edge)) * 0.85 + 0.15
    if (Math.random() > prob) continue

    raw.push({
      sx: sx + Math.random(),
      sy: sy + Math.random(),
      lum: c.l,
      r: c.r,
      g: c.g,
      b: c.b,
    })
  }

  // 按亮度加权质心，把视觉主体对齐到画面中轴
  let wx = 0
  let wsum = 0
  for (const p of raw) {
    wx += p.sx * p.lum
    wsum += p.lum
  }
  const centroidX = wsum > 0 ? wx / wsum : (minX + maxX) / 2
  const shift = (minX + bw / 2 - centroidX) * scale

  const map = new Map()
  for (const p of raw) {
    const px = originX + (p.sx - minX) * scale + shift
    const py = originY + (p.sy - minY) * scale

    const boost = 1.5
    const rr = Math.min(255, Math.round(p.r * boost))
    const gg = Math.min(255, Math.round(p.g * boost))
    const bb = Math.min(255, Math.round(p.b * boost))
    const key = ((rr >> 4) << 8) | ((gg >> 4) << 4) | (bb >> 4)

    let bucket = map.get(key)
    if (!bucket) {
      bucket = { color: `rgb(${rr},${gg},${bb})`, items: [] }
      map.set(key, bucket)
    }

    const dx = px - cx
    const dy = py - cy
    const ang = Math.atan2(dy, dx) + (Math.random() - 0.5) * 0.9

    bucket.items.push({
      ox: Math.round(px),
      oy: Math.round(py),
      nx: Math.cos(ang),
      ny: Math.sin(ang),
      sp: 0.5 + Math.random() * 1.05,
      s: Math.random() < 0.55 ? 1 : 2,
    })
  }

  buckets = Array.from(map.values())
}

function loop(now) {
  const t = now - t0
  ctx.clearRect(0, 0, W, H)
  ctx.globalCompositeOperation = 'lighter'

  const appear = easeOut(Math.min(1, t / APPEAR))
  const e = Math.max(0, t - SCATTER)
  const alpha =
    t < APPEAR
      ? appear
      : e > 0
        ? Math.max(0, 1 - Math.pow(Math.min(1, e / 1450), 1.2))
        : 1

  if (alpha > 0.004) {
    ctx.globalAlpha = alpha
    const growth = 0.68 + 0.32 * appear
    const drift = e * e * 0.00048

    for (let bi = 0; bi < buckets.length; bi++) {
      const bucket = buckets[bi]
      ctx.fillStyle = bucket.color
      const items = bucket.items
      for (let i = 0; i < items.length; i++) {
        const p = items[i]
        let x
        let y
        if (e > 0) {
          const d = drift * p.sp
          x = p.ox + p.nx * d
          y = p.oy + p.ny * d - e * 0.024
        } else {
          x = cx + (p.ox - cx) * growth
          y = cy + (p.oy - cy) * growth
        }
        ctx.fillRect(x | 0, y | 0, p.s, p.s)
      }
    }
  }

  ctx.globalAlpha = 1
  ctx.globalCompositeOperation = 'source-over'

  if (t > SCATTER - 40 && !uiOut.value) uiOut.value = true

  if (!finished && t > FINISH) finish()
  raf = requestAnimationFrame(loop)
}

function finish() {
  if (finished) return
  finished = true
  cancelAnimationFrame(raf)
  emit('done')
}

function onTap() {
  if (finished) return
  const elapsed = performance.now() - t0
  if (elapsed < SCATTER) {
    t0 -= SCATTER - elapsed - 30
    return
  }
  finish()
}
</script>

<template>
  <div class="splash" @click="onTap">
    <canvas ref="canvasRef"></canvas>
    <div class="wordmark" :class="{ in: uiIn, out: uiOut }">
      <div class="name">STAR WITH U</div>
      <div class="sub">把我们的每一天，都挂到星星上</div>
    </div>
  </div>
</template>

<style scoped>
.splash {
  position: fixed;
  inset: 0;
  z-index: 100;
  background: radial-gradient(ellipse at 50% 52%, #090c18 0%, #05060d 58%, #030409 100%);
  cursor: pointer;
}

canvas {
  position: absolute;
  inset: 0;
  display: block;
}

.wordmark {
  position: absolute;
  bottom: 7%;
  left: 0;
  right: 0;
  text-align: center;
  pointer-events: none;
  opacity: 0;
  transform: translateY(8px);
  transition: opacity 1s ease, transform 1s ease;
}

.wordmark.in {
  opacity: 1;
  transform: translateY(0);
}

.wordmark.out {
  opacity: 0;
  transform: translateY(-10px);
  transition-duration: 0.6s;
}

.name {
  font-size: 14px;
  letter-spacing: 0.34em;
  color: rgba(233, 238, 247, 0.8);
  text-indent: 0.34em;
  text-shadow: 0 0 26px rgba(255, 158, 196, 0.35);
}

.sub {
  margin-top: 10px;
  font-size: 11px;
  letter-spacing: 0.24em;
  color: rgba(233, 238, 247, 0.28);
}
</style>
