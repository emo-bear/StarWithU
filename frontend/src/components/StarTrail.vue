<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { prettyDate } from '../api'

// 「星途」：一条从左上远方舒展到右下跟前的路。
//
// 两件事决定了它像不像一条"路"：
// 1. 中心线用羊角螺线（Euler spiral / clothoid）积分得到。这种曲线的曲率随弧长
//    单调增大，正是公路与铁路上接直线和圆曲线的"缓和曲线"，所以它在远端几乎是
//    直的（半径无穷大，从远处渗出来），越靠近观者弯得越厉害 —— 这既是"由远及近"，
//    也天然是一道好看的弯，而不需要手摆控制点去猜。
// 2. 路面不用任何填色/描边去"画形状"。凡是填出来的色块都有边，而任何一条边都会
//    让这条路读成管道或带子。这里只沿中心线撒柔光团：每一团自带 0→1→0 的径向
//    剖面，叠起来横截面就是一条高斯，所以整条光带找不到可以指认的边界，只有明暗。
//    半宽仍按透视律正比于弧长（见 thickAt），远端自然收细、隐没。
const props = defineProps({
  events: { type: Array, default: () => [] },
  active: { type: Boolean, default: true },
})
const emit = defineEmits(['open'])

const wrapRef = ref(null)
const canvasRef = ref(null)
const tip = ref({ show: false, x: 0, y: 0, date: '', title: '' })

// 羊角螺线的参数（全部在 1060×720 的参考画面里推过一遍）
// 几个参数都能用 URL 覆盖，方便一眼比对：?th0=12&turn=56&pow=1.1
const Q = new URLSearchParams(location.search)
const qn = (k, d) => (Q.has(k) && !Number.isNaN(Number(Q.get(k))) ? Number(Q.get(k)) : d)
const PATH = {
  th0: qn('th0', 6), // 远端切向角（度，相对屏幕 +x，正值朝下）
  turn: qn('turn', 68), // 全程一共转多少度
  pow: qn('pow', 1.0), // 曲率增速：1 是纯羊角螺线，越大越把弯收到跟前
  p0: [0.012, 0.05], // 远端起点（归一化）
  exitY: 1.05, // 校准时让它从底边外面穿出去
  n: 720,
}

const COUNT = 3600

// 主题色：粉为主 + 暖金 + 藕紫 + 一点青
const PALETTE = [
  { rgb: [255, 146, 190], w: 0.56 },
  { rgb: [255, 198, 142], w: 0.19 },
  { rgb: [206, 166, 255], w: 0.13 },
  { rgb: [127, 227, 242], w: 0.07 },
  { rgb: [255, 232, 242], w: 0.05 },
]
const STAR_COLORS = [
  [255, 244, 250],
  [255, 214, 172],
  [196, 240, 255],
]

// 近端半宽占画面宽度的比例。远端由透视律自动收到 ~0。
const NEAR_HW = qn('hw', 0.165)

// 横截面剖面（0 = 左缘，0.5 = 中轴，1 = 右缘）：一条两端严格归零的钟形曲线。
// 因为两端是 0，所以带子在任何位置上都没有"边"，只有明暗。
const CROSS = [
  [0.0, 0.0],
  [0.1, 0.2],
  [0.26, 0.6],
  [0.5, 1.0],
  [0.74, 0.6],
  [0.9, 0.2],
  [1.0, 0.0],
]

// 三层：外层雾 → 主体 → 芯。K 是半宽倍数（2.7 倍半宽之外就完全没了），
// peak 是这条剖面在中轴上的不透明度。三层加起来横截面才是一条高斯。
const BAND = [
  { K: 3.0, peak: 0.05, rgb: [232, 148, 204] },
  { K: 1.6, peak: 0.095, rgb: [255, 156, 200] },
  { K: 1.0, peak: 0.15, rgb: [255, 170, 208] },
  { K: 0.55, peak: 0.2, rgb: [255, 200, 228] },
  { K: 0.22, peak: 0.28, rgb: [255, 236, 248] },
]
// 横截面切成这么多细带。带与带之间只差一点点透明度，所以它们的接缝看不见；
// 而每条细带自己是沿整条路一次画完的连续多边形，沿路方向没有任何拼接。
const LANES = 78
const LANE_SAMPLES = 200

let W = 1
let H = 1
let dpr = 1
let ctx = null
let raf = 0
let last = 0
let time = 0
let lut = [] // {x,y,nx,ny,tx,ty} 像素坐标
let bandLanes = [] // {path, grad} 横截面细带（沿路一次画完，所以没有拼缝）
let bandCv = null // 光带先画到半分辨率的离屏画布，再放大贴回来
let bandG = null
let bandScale = 0.5
let dust = []
let spray = []
let stars = [] // 事件星屏幕坐标
let bgStars = []
let bgClouds = []
let hoverIdx = -1
let sprites = []
let starSprites = []
let ready = false

onMounted(async () => {
  buildDust()
  buildBackdrop()
  buildSprites()
  resize()
  window.addEventListener('resize', resize)
  window.addEventListener('mousemove', onMove)
  wrapRef.value?.addEventListener('mouseleave', onLeave)
  raf = requestAnimationFrame(tick)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', resize)
  window.removeEventListener('mousemove', onMove)
  wrapRef.value?.removeEventListener('mouseleave', onLeave)
})

/* ---------------- 曲线 ---------------- */

// 切向角：θ(u) = θ0 + Δ·u^pow。曲率 = dθ/ds ∝ u^(pow-1)，
// pow = 1 就是严格的羊角螺线（曲率线性增长），因此远端曲率趋近 0。
function headingAt(u) {
  return (PATH.th0 + PATH.turn * Math.pow(u, PATH.pow)) * (Math.PI / 180)
}

function buildPath() {
  const n = PATH.n
  const x0 = PATH.p0[0] * W
  const y0 = PATH.p0[1] * H
  const walk = (L) => {
    const pts = new Array(n)
    let x = x0
    let y = y0
    const ds = L / (n - 1)
    for (let i = 0; i < n; i++) {
      pts[i] = [x, y]
      const a = headingAt(i / (n - 1))
      x += Math.cos(a) * ds
      y += Math.sin(a) * ds
    }
    return pts
  }
  // 长度决定了画面里能看到多少路：二分到"刚好从底边外穿出去"
  let lo = 120
  let hi = 9000
  for (let k = 0; k < 40; k++) {
    const mid = (lo + hi) / 2
    if (walk(mid)[n - 1][1] < H * PATH.exitY) lo = mid
    else hi = mid
  }
  const pts = walk(lo)

  lut = new Array(n)
  for (let i = 0; i < n; i++) {
    const a = pts[Math.max(0, i - 1)]
    const b = pts[Math.min(n - 1, i + 1)]
    let dx = b[0] - a[0]
    let dy = b[1] - a[1]
    const len = Math.hypot(dx, dy) || 1
    dx /= len
    dy /= len
    lut[i] = { x: pts[i][0], y: pts[i][1], tx: dx, ty: dy, nx: -dy, ny: dx }
  }
}

const clamp01 = (v) => Math.min(1, Math.max(0, v))

// 路面半宽：等宽的路投到屏幕上，宽度正比于它离消失点的距离，也就是正比于 u。
// 这是整条路"由远及近"的根，不能换成固定关键帧。
function thickAt(t) {
  const near = Math.min(W * NEAR_HW, H * 0.32)
  return near * (0.03 + 0.97 * clamp01(t))
}

function sample(t) {
  const g = Math.min(lut.length - 1.001, Math.max(0, t) * (lut.length - 1))
  const i = Math.floor(g)
  const f = g - i
  const a = lut[i]
  const b = lut[Math.min(lut.length - 1, i + 1)]
  return {
    x: a.x + (b.x - a.x) * f,
    y: a.y + (b.y - a.y) * f,
    nx: a.nx + (b.nx - a.nx) * f,
    ny: a.ny + (b.ny - a.ny) * f,
    tx: a.tx + (b.tx - a.tx) * f,
    ty: a.ty + (b.ty - a.ty) * f,
  }
}

// 远端要淡出去：路是从很远的地方渗出来的，不该有一刀切的开头
function apron(t) {
  const a = clamp01((t - 0.015) / 0.3)
  return a * a * (3 - 2 * a)
}

/* ---------------- 横截面 ---------------- */

function crossAt(u) {
  if (u <= 0) return 1
  if (u >= 1) return 0
  for (let i = 1; i < CROSS.length; i++) {
    if (u <= CROSS[i][0]) {
      const [x0, y0] = CROSS[i - 1]
      const [x1, y1] = CROSS[i]
      return y0 + ((y1 - y0) * (u - x0)) / (x1 - x0 || 1)
    }
  }
  return 0
}

// 某一条细带（离中轴 xK 倍半宽处）的合成不透明度与合成颜色：
// 五层剖面在这一点上相加，颜色按权重混。这样只要一个 profile 数组，
// 就能一次把整条高斯画成若干条等宽的带，而不用一层层去叠。
function laneStyle(x) {
  const u = Math.abs(x)
  let a = 0
  let r = 0
  let gg = 0
  let b = 0
  for (const L of BAND) {
    // CROSS 是"整条横截面"的剖面（0.5 才是中轴），这里要把"离中轴的距离"折回去
    const w = L.peak * crossAt(0.5 + 0.5 * (u / L.K))
    if (w <= 0) continue
    a += w
    r += w * L.rgb[0]
    gg += w * L.rgb[1]
    b += w * L.rgb[2]
  }
  if (a <= 0) return null
  return { a, rgb: [Math.round(r / a), Math.round(gg / a), Math.round(b / a)] }
}

// 把光带预先切成 LANES 条：每条都是沿整条路的一整块多边形，
// 填色用一条沿路方向的渐变（远端 0 → 近端最亮），所以沿路方向不会有接缝。
function buildBand() {
  bandLanes = []
  const K = BAND[0].K
  for (let k = 0; k < LANES; k++) {
    const lo = -K + (2 * K * k) / LANES
    const hi = -K + (2 * K * (k + 1)) / LANES
    const st = laneStyle((lo + hi) / 2)
    if (!st) continue

    const path = new Path2D()
    for (let i = 0; i < LANE_SAMPLES; i++) {
      const t = i / (LANE_SAMPLES - 1)
      const p = sample(t)
      const hw = thickAt(t) * lo
      const x = p.x + p.nx * hw
      const y = p.y + p.ny * hw
      if (i === 0) path.moveTo(x, y)
      else path.lineTo(x, y)
    }
    for (let i = LANE_SAMPLES - 1; i >= 0; i--) {
      const t = i / (LANE_SAMPLES - 1)
      const p = sample(t)
      const hw = thickAt(t) * hi
      path.lineTo(p.x + p.nx * hw, p.y + p.ny * hw)
    }
    path.closePath()

    const head = sample(0)
    const tail = sample(1)
    const grad = ctx.createLinearGradient(head.x, head.y, tail.x, tail.y)
    const [r, g, b] = st.rgb
    for (let i = 0; i <= 24; i++) {
      const t = i / 24
      const fade = apron(t) * (0.08 + 0.92 * Math.pow(t, 1.5))
      grad.addColorStop(t, `rgba(${r},${g},${b},${(st.a * fade).toFixed(4)})`)
    }
    bandLanes.push({ path, grad })
  }

  // 一次性烘焙到半分辨率离屏画布
  const bg = bandG
  bg.setTransform(1, 0, 0, 1, 0, 0)
  bg.clearRect(0, 0, bandCv.width, bandCv.height)
  bg.globalCompositeOperation = 'lighter'
  bg.globalAlpha = 1
  bg.setTransform(bandScale, 0, 0, bandScale, 0, 0)
  for (const lane of bandLanes) {
    bg.fillStyle = lane.grad
    bg.fill(lane.path)
  }
  bg.setTransform(1, 0, 0, 1, 0, 0)
}

/* ---------------- 伪随机（同一事件每帧位置稳定） ---------------- */

function hash(i, salt) {
  const x = Math.sin(i * 12.9898 + salt * 78.233) * 43758.5453
  return x - Math.floor(x)
}

/* ---------------- 粒子 ---------------- */

function randn() {
  let u = 0
  let v = 0
  while (u === 0) u = Math.random()
  while (v === 0) v = Math.random()
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v)
}

// 只返回下标：以前这里返回色值再用 findIndex + pickColor() 反查，
// 而 pickColor() 每次调用都会重新随机，导致经常查不到（-1），drawImage(undefined) 直接抛错
function pickColorIdx() {
  let r = Math.random()
  for (let i = 0; i < PALETTE.length; i++) {
    if (r < PALETTE[i].w) return i
    r -= PALETTE[i].w
  }
  return 0
}

function buildDust() {
  dust = []
  for (let i = 0; i < COUNT; i++) {
    const r = Math.random()
    // 三层：路上的密尘 / 带外的稀薄尘 / 零星飞溅
    const layer = r < 0.6 ? 0 : r < 0.9 ? 1 : 2
    const sigma = layer === 0 ? 0.55 : layer === 1 ? 1.15 : 2.6
    dust.push({
      t: Math.random(),
      sp: 0.03 + Math.random() * 0.06,
      off: randn() * sigma,
      layer,
      size: layer === 0 ? 3.0 + Math.random() * 3.2 : layer === 1 ? 2.2 + Math.random() * 2.4 : 1.4 + Math.random() * 1.8,
      alpha: layer === 0 ? 0.30 + Math.random() * 0.42 : layer === 1 ? 0.12 + Math.random() * 0.22 : 0.05 + Math.random() * 0.13,
      ph: Math.random() * 6.283,
      wob: 0.5 + Math.random() * 1.4,
      rgbIdx: pickColorIdx(),
    })
  }
  dust.sort((a, b) => a.rgbIdx - b.rgbIdx)
  buildSpray()
}

// 向外散发的光点：从路面上"抖"出去，越远越淡。
// 用无状态的相位代替真实粒子，省掉每帧积分，也不会在暂停时丢位置。
function buildSpray() {
  spray = []
  const n = 1500
  for (let i = 0; i < n; i++) {
    spray.push({
      t: 0.14 + Math.pow(Math.random(), 0.75) * 0.86,
      seed: Math.random(),
      sp: 0.1 + Math.random() * 0.26,
      side: Math.random() < 0.5 ? -1 : 1,
      reach: 0.4 + Math.random() * 3.4,
      jit: randn() * 0.55,
      size: 1.5 + Math.random() * 3.4,
      a: 0.1 + Math.random() * 0.4,
      rgbIdx: pickColorIdx(),
    })
  }
  spray.sort((a, b) => a.rgbIdx - b.rgbIdx)
}

// 背景：远处的星点与几团极淡的星云，把留白填掉，但不抢主体
function buildBackdrop() {
  bgStars = []
  for (let i = 0; i < 1250; i++) {
    const t = Math.random()
    bgStars.push({
      x: Math.random(),
      y: Math.random(),
      r: 0.7 + Math.pow(Math.random(), 2.2) * 1.9,
      a: 0.07 + Math.pow(Math.random(), 1.7) * 0.3,
      ph: Math.random() * 6.283,
      c: t < 0.62 ? 0 : t < 0.82 ? 1 : t < 0.94 ? 2 : 3,
    })
  }
  bgClouds = []
  for (let i = 0; i < 34; i++) {
    const x = hash(i, 7)
    const y = hash(i, 11) * 0.62 + (hash(i, 13) < 0.5 ? 0.02 : 0.72)
    bgClouds.push({
      x,
      y: clamp01(y + (hash(i, 17) - 0.5) * 0.06),
      rx: 0.11 + hash(i, 23) * 0.19,
      ry: 0.08 + hash(i, 29) * 0.14,
      c: hash(i, 31) < 0.62 ? 0 : hash(i, 37) < 0.7 ? 2 : 3,
      a: 0.024 + hash(i, 41) * 0.038,
    })
  }
  // 手工补几团：路把画面斜分成两半，左下和右上这两片最容易空
  bgClouds.push(
    { x: 0.24, y: 0.62, rx: 0.3, ry: 0.2, c: 0, a: 0.042 },
    { x: 0.10, y: 0.84, rx: 0.26, ry: 0.2, c: 2, a: 0.036 },
    { x: 0.36, y: 0.92, rx: 0.3, ry: 0.16, c: 0, a: 0.034 },
    { x: 0.07, y: 0.38, rx: 0.2, ry: 0.18, c: 2, a: 0.028 },
    { x: 0.74, y: 0.12, rx: 0.28, ry: 0.16, c: 0, a: 0.038 },
    { x: 0.92, y: 0.28, rx: 0.22, ry: 0.2, c: 2, a: 0.032 },
    { x: 0.55, y: 0.05, rx: 0.26, ry: 0.12, c: 3, a: 0.026 },
    { x: 0.16, y: 0.20, rx: 0.22, ry: 0.14, c: 0, a: 0.03 },
    { x: 0.05, y: 0.58, rx: 0.24, ry: 0.22, c: 0, a: 0.034 },
    { x: 0.19, y: 0.44, rx: 0.22, ry: 0.18, c: 2, a: 0.03 },
    { x: 0.09, y: 0.97, rx: 0.26, ry: 0.16, c: 0, a: 0.032 },
    { x: 0.30, y: 0.76, rx: 0.24, ry: 0.18, c: 2, a: 0.026 },
    { x: 0.44, y: 0.96, rx: 0.26, ry: 0.14, c: 0, a: 0.028 },
    { x: 0.17, y: 0.72, rx: 0.28, ry: 0.2, c: 0, a: 0.045 },
    { x: 0.04, y: 0.78, rx: 0.22, ry: 0.18, c: 2, a: 0.04 },
    { x: 0.62, y: 0.20, rx: 0.24, ry: 0.16, c: 0, a: 0.03 },
    { x: 0.14, y: 0.55, rx: 0.3, ry: 0.22, c: 0, a: 0.036 },
    { x: 0.30, y: 0.66, rx: 0.26, ry: 0.18, c: 2, a: 0.03 },
    { x: 0.06, y: 0.66, rx: 0.22, ry: 0.18, c: 0, a: 0.032 },
  )
}

// 内容画完之后统一用 destination-in 叠一层径向遮罩，
// 保证贴图四边 alpha 严格为 0 —— 否则放大后会看到一圈方形边界
function maskEdge(g, s, hold = 0.78) {
  g.globalCompositeOperation = 'destination-in'
  const m = g.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2)
  m.addColorStop(0, 'rgba(0,0,0,1)')
  m.addColorStop(hold, 'rgba(0,0,0,1)')
  m.addColorStop(1, 'rgba(0,0,0,0)')
  g.fillStyle = m
  g.fillRect(0, 0, s, s)
  g.globalCompositeOperation = 'source-over'
}

function makeSprite(rgb) {
  const s = 64
  const c = document.createElement('canvas')
  c.width = c.height = s
  const g = c.getContext('2d')
  const grad = g.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2)
  // 只有最里面一点点泛白，主体就是自己的颜色，整团才不会发灰
  grad.addColorStop(0, `rgba(255,255,255,0.62)`)
  grad.addColorStop(0.14, `rgba(${rgb[0]},${rgb[1]},${rgb[2]},0.9)`)
  grad.addColorStop(0.42, `rgba(${rgb[0]},${rgb[1]},${rgb[2]},0.26)`)
  grad.addColorStop(1, `rgba(${rgb[0]},${rgb[1]},${rgb[2]},0)`)
  g.fillStyle = grad
  g.fillRect(0, 0, s, s)
  maskEdge(g, s, 0.72)
  return c
}

function makeStarSprite(rgb) {
  const s = 128
  const c = document.createElement('canvas')
  c.width = c.height = s
  const g = c.getContext('2d')
  const c0 = `${rgb[0]},${rgb[1]},${rgb[2]}`
  const grad = g.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2)
  grad.addColorStop(0, 'rgba(255,255,255,1)')
  grad.addColorStop(0.1, `rgba(255,255,255,0.92)`)
  grad.addColorStop(0.26, `rgba(${c0},0.5)`)
  grad.addColorStop(0.55, `rgba(${c0},0.1)`)
  grad.addColorStop(1, `rgba(${c0},0)`)
  g.fillStyle = grad
  g.fillRect(0, 0, s, s)
  // 十字光芒
  g.translate(s / 2, s / 2)
  for (const [sx, sy] of [
    [1, 0.045],
    [0.045, 1],
  ]) {
    g.save()
    g.scale(sx, sy)
    const lg = g.createRadialGradient(0, 0, 0, 0, 0, s / 2)
    lg.addColorStop(0, 'rgba(255,255,255,0.9)')
    lg.addColorStop(0.35, `rgba(${c0},0.32)`)
    lg.addColorStop(1, `rgba(${c0},0)`)
    g.fillStyle = lg
    g.beginPath()
    g.arc(0, 0, s / 2, 0, Math.PI * 2)
    g.fill()
    g.restore()
  }
  // 十字是用 translate 到中心画的，这里必须先把变换复位，
  // 否则遮罩的 fillRect 只覆盖右下四分之一，其余全被 destination-in 擦掉
  g.setTransform(1, 0, 0, 1, 0, 0)
  maskEdge(g, s, 0.8)
  return c
}

function buildSprites() {
  sprites = PALETTE.map((p) => makeSprite(p.rgb))
  starSprites = STAR_COLORS.map(makeStarSprite)
}

/* ---------------- 尺寸 / 交互 ---------------- */

function resize() {
  const el = wrapRef.value
  const cv = canvasRef.value
  if (!el || !cv) return
  W = el.clientWidth || window.innerWidth
  H = el.clientHeight || window.innerHeight
  dpr = Math.min(2, window.devicePixelRatio || 1)
  cv.width = Math.round(W * dpr)
  cv.height = Math.round(H * dpr)
  cv.style.width = W + 'px'
  cv.style.height = H + 'px'
  ctx = cv.getContext('2d')
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  buildPath()
  // 光带单独画在一张半分辨率的离屏画布上：放大贴回来的时候，
  // 细带之间的亚像素接缝会被重采样抹平，顺带让整条带子更柔。
  bandScale = 0.5
  bandCv = document.createElement('canvas')
  bandCv.width = Math.max(1, Math.round(W * bandScale))
  bandCv.height = Math.max(1, Math.round(H * bandScale))
  bandG = bandCv.getContext('2d')
  buildBand()
  ready = true
}

// 事件星在路上的基准位置：按时间先后等距，避开两端
function starT(i, n) {
  if (n <= 1) return 0.5
  return 0.17 + (0.77 * i) / (n - 1)
}

function onMove(e) {
  if (!stars.length) return
  let best = -1
  let bd = 26 * 26
  for (let i = 0; i < stars.length; i++) {
    const d = (stars[i].x - e.clientX) ** 2 + (stars[i].y - e.clientY) ** 2
    if (d < bd) {
      bd = d
      best = i
    }
  }
  if (best === hoverIdx) {
    if (best >= 0) tip.value = { ...tip.value, x: stars[best].x, y: stars[best].y }
    return
  }
  hoverIdx = best
  const cv = canvasRef.value
  if (cv) cv.style.cursor = best >= 0 ? 'pointer' : 'default'
  if (best >= 0 && props.events[best]) {
    const ev = props.events[best]
    tip.value = {
      show: true,
      x: stars[best].x,
      y: stars[best].y,
      date: prettyDate(ev.date),
      title: ev.title,
    }
  } else {
    tip.value.show = false
  }
}

function onLeave() {
  hoverIdx = -1
  tip.value.show = false
}

function onClick(e) {
  if (hoverIdx >= 0 && props.events[hoverIdx]) {
    emit('open', props.events[hoverIdx])
    return
  }
  if (!stars.length) return
  let best = -1
  let bd = 30 * 30
  for (let i = 0; i < stars.length; i++) {
    const d = (stars[i].x - e.clientX) ** 2 + (stars[i].y - e.clientY) ** 2
    if (d < bd) {
      bd = d
      best = i
    }
  }
  if (best >= 0 && props.events[best]) emit('open', props.events[best])
}

/* ---------------- 主循环 ---------------- */

function tick(now) {
  raf = requestAnimationFrame(tick)
  const dt = last ? Math.min(0.05, (now - last) / 1000) : 0.016
  last = now
  if (!ready || !props.active) return
  time += dt
  draw()
}

function draw() {
  const g = ctx
  g.clearRect(0, 0, W, H)
  g.globalCompositeOperation = 'lighter'

  // 0. 背景：极淡的星云团 + 星点，把留白填满
  for (const c of bgClouds) {
    const rx = c.rx * W
    const ry = c.ry * H
    const x = c.x * W
    const y = c.y * H
    g.globalAlpha = c.a * (0.85 + 0.15 * Math.sin(time * 0.35 + c.x * 9))
    g.drawImage(sprites[c.c], x - rx, y - ry, rx * 2, ry * 2)
  }
  for (const s of bgStars) {
    const tw = 0.62 + 0.38 * Math.sin(time * 1.1 + s.ph)
    g.globalAlpha = s.a * tw
    const d = s.r * 3.4
    g.drawImage(sprites[s.c === 1 ? 1 : s.c === 2 ? 2 : s.c === 3 ? 3 : 0], s.x * W - d / 2, s.y * H - d / 2, d, d)
  }

  // ================= 光带 =================
  // 不填"形状"，只填"剖面"：整条带预先拆成几十条并排的细带，每条沿路一次画完，
  // 横截面两端都归零。所以既没有沿路的拼缝，也没有可以指认的轮廓线，
  // 只有从明到暗的过渡 —— 边界是被"化"掉的，不是被画出来的。
  // 光带本身不随时间变化，所以只在 resize 时画一次，这里直接贴图。
  g.globalAlpha = 1
  g.drawImage(bandCv, 0, 0, W, H)

  // 流光：顺路面从远处涌向跟前，近端更长更快，这是"由远及近"最直接的证据。
  // 用拉长的柔光而不是实心光条，才不会在带子上割出一条硬边。
  for (let i = 0; i < 14; i++) {
    const s0 = 0.08 + ((time * 0.1 + i / 14) % 0.92)
    const t = Math.pow(s0, 1.45)
    const p = sample(t)
    const hw = thickAt(t)
    const len = hw * 1.5 + 10
    const wid = hw * 0.34 + 3
    g.save()
    g.translate(p.x, p.y)
    g.rotate(Math.atan2(p.ty, p.tx))
    g.globalAlpha = Math.min(1, 0.4 * Math.pow(t, 1.2) * Math.sin(Math.PI * Math.min(1, s0 * 1.04)))
    g.drawImage(sprites[4], -len / 2, -wid / 2, len, wid)
    g.restore()
  }

  // 外散光点：从路面上抖出去的光，越远越淡，散到一定程度就消失
  for (let i = 0; i < spray.length; i++) {
    const d = spray[i]
    const ph = (d.seed + time * d.sp) % 1
    const a = sample(d.t)
    const hw = thickAt(d.t)
    const offN = d.side * hw * (0.35 + d.reach * ph)
    const offT = d.jit * hw * ph * 1.3
    const x = a.x + a.nx * offN + a.tx * offT
    const y = a.y + a.ny * offN + a.ty * offT
    const fade = (1 - ph) * (1 - ph)
    const al = d.a * fade * apron(d.t) * 0.85
    if (al < 0.008) continue
    const px = d.size * (H / 720) * (0.35 + 1.05 * d.t) * (1 - ph * 0.4)
    g.globalAlpha = al
    g.drawImage(sprites[d.rgbIdx] || sprites[0], x - px / 2, y - px / 2, px, px)
  }

  // 尘（带上的颗粒，尺寸也跟着透视走）
  for (let i = 0; i < dust.length; i++) {
    const d = dust[i]
    const t = (d.t + time * d.sp) % 1
    const a = sample(t)
    const hw = thickAt(t)
    const wob = Math.sin(time * 0.9 + d.ph) * hw * 0.1 * d.wob
    const off = d.off * hw + wob
    const x = a.x + a.nx * off
    const y = a.y + a.ny * off
    const edge = apron(t) * Math.min(1, (1 - t) / 0.04)
    const al = d.alpha * edge * (0.62 + 0.38 * Math.sin(time * 2.2 + d.ph)) * 0.8
    if (al <= 0.01) continue
    const px = d.size * (H / 720) * (0.3 + 1.05 * t)
    g.globalAlpha = al
    g.drawImage(sprites[d.rgbIdx] || sprites[0], x - px / 2, y - px / 2, px, px)
  }

  // 事件星：时间顺序占位，但围绕光带散落
  const n = props.events.length
  stars = []
  for (let i = 0; i < n; i++) {
    const base = starT(i, n)
    const t = clamp01(base + (hash(i, 1) - 0.5) * 0.026)
    const p = sample(t)
    const th = thickAt(t)
    const side = hash(i, 2) < 0.5 ? -1 : 1
    const amp = th * (0.2 + 0.75 * hash(i, 3))
    const breathe = Math.sin(time * 1.1 + i * 1.7) * (1.2 + 2 * t)
    const x = p.x + p.nx * (side * amp + breathe)
    const y = p.y + p.ny * (side * amp + breathe)
    stars.push({ x, y })

    const on = i === hoverIdx
    const pulse = 0.88 + 0.12 * Math.sin(time * 2 + i)
    const size = (7 + 24 * t + (on ? 9 : 0)) * pulse * (H / 720) * 1.1
    const spr = starSprites[i % starSprites.length]

    g.globalAlpha = on ? 0.32 : 0.2
    g.drawImage(spr, x - size * 1.8, y - size * 1.8, size * 3.6, size * 3.6)
    g.globalAlpha = on ? 0.8 : 0.58
    g.drawImage(spr, x - size * 0.85, y - size * 0.85, size * 1.7, size * 1.7)
    g.globalAlpha = 0.95
    g.drawImage(spr, x - size * 0.31, y - size * 0.31, size * 0.62, size * 0.62)

    if (on) {
      g.globalCompositeOperation = 'source-over'
      g.globalAlpha = 0.5
      g.strokeStyle = 'rgba(255,220,236,0.9)'
      g.lineWidth = 1
      g.beginPath()
      g.arc(x, y, size * 0.66, 0, Math.PI * 2)
      g.stroke()
      g.globalCompositeOperation = 'lighter'
    }
  }

  g.globalAlpha = 1
  g.globalCompositeOperation = 'source-over'
}
</script>

<template>
  <div ref="wrapRef" class="trail" @click="onClick" @mousemove="onMove">
    <canvas ref="canvasRef" class="cv"></canvas>

    <div class="tip" v-if="tip.show" :style="{ left: tip.x + 'px', top: tip.y - 34 + 'px' }">
      <div class="d">{{ tip.date }}</div>
      <div class="t">{{ tip.title }}</div>
    </div>
  </div>
</template>

<style scoped>
.trail {
  position: absolute;
  inset: 0;
  z-index: 5;
  background:
    radial-gradient(ellipse 64% 52% at 82% 84%, rgba(255, 158, 196, 0.055), transparent 74%),
    radial-gradient(ellipse 44% 38% at 14% 12%, rgba(127, 227, 242, 0.03), transparent 72%);
}

.cv {
  display: block;
  width: 100%;
  height: 100%;
}

.tip {
  position: absolute;
  transform: translate(-50%, -100%);
  padding: 7px 12px 8px;
  border-radius: 10px;
  background: rgba(10, 12, 22, 0.86);
  border: 1px solid rgba(255, 255, 255, 0.09);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.55), 0 0 24px rgba(255, 158, 196, 0.1);
  backdrop-filter: blur(8px);
  pointer-events: none;
  white-space: nowrap;
  text-align: center;
}

.tip .d {
  font-size: 10.5px;
  letter-spacing: 0.16em;
  color: rgba(255, 158, 196, 0.85);
}

.tip .t {
  margin-top: 3px;
  font-size: 12.5px;
  letter-spacing: 0.06em;
  color: rgba(240, 244, 252, 0.92);
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
