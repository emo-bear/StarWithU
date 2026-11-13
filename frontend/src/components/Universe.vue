<script setup>
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as THREE from 'three'
import { prettyDate } from '../api'

const props = defineProps({
  events: { type: Array, default: () => [] },
  active: { type: Boolean, default: false },
  // 切到「星途」时暂停渲染，别让两个 60fps 循环一起跑
  paused: { type: Boolean, default: false },
})
const emit = defineEmits(['open', 'core'])

const wrapRef = ref(null)
const tooltip = ref({ show: false, x: 0, y: 0, date: '', title: '' })

let scene
let camera
let renderer
let tiltGroup
let spinGroup
let points
let geo
let mat
let stars
let coreSpikes
let coreGlow
let coreHalo
let frameGlow
let dustLayers = []
let raf = 0
let W = 1
let H = 1

const FOV = 45
const BASE_CAMZ = 3.05
const TILT_X = 0.56 // 固定俯角：只允许绕盘面中心左右转，不允许改倾角
const TILT_Z = -0.2 // 盘面在屏幕里的滚转，模仿参考图的构图
const SPIRAL_K = 2.9 // 对数螺旋的卷绕强度
const SPIRAL_R0 = 0.055
const SIZE_K = 330 // 点尺寸系数：px ≈ size * 330 / dist

// 盘面相对"四角半径"的倍数。>1 时盘面比窗口更大，
// 旋臂一直铺到四边四角之外，转动时也看不到边界。
const DISK_OVER = 1.36
// 星系保持铺满画面（亮盘边缘刚好在画面上边外侧），
// 四角交给光晕 + 沿臂排列的装饰星点去收，不缩星系。
// 解析场平面再放大一圈，几何边界彻底离开画面。
// 右上角那条视线几乎和盘面平行，打到盘面上的半径能到 33 个盘单位，
// 5 倍只到 21.5，平面边缘正好横切在右上角（ndc y≈0.88）——那块空白就是它。
const FIELD_OVER = 9.5

let diskR = 3 // 星系本身的尺度（旋臂/尘埃的分布按它归一）
let coreR = 1.5 // 核心的视觉尺度：盘面放大时核心不跟着变大
let planeR = 4 // 解析场平面半径，比 diskR 更大
let visR = 0.62 // 画面四角在盘面坐标系里对应的半径（diskR 归一）
let eventR = 1.5 // 事件星分布半径：保证都在可视范围内
let armOffset = 0 // 事件旋臂的相位补偿（半径不同则相位不同）
let zoomK = 1 // 滚轮缩放带来的视口补偿

let n = 0
let targetPos = null
let startPos = null
let curPos = null
let aSizeArr = null
let aAlphaArr = null
let aColorArr = null
let curSize = null
let tgtSize = null
let baseSize = 0.11

const prevPos = new Map()

let introT = 0
let intro = false
let freezeIntro = -1

let rotY = 0.6
let velY = 0
let dragging = false
let lastPx = 0
let lastPy = 0
let moved = 0
let hoverIdx = -1
let pointerX = 0
let pointerY = 0
let camZ = BASE_CAMZ
let camZT = BASE_CAMZ
// 星系核心在屏幕上的位置：点它进「星途」
let coreSX = 0
let coreSY = 0
const CORE_HIT = 74
const camOff = { x: 0, y: 0 }
const camOffT = { x: 0, y: 0 }

const v3 = new THREE.Vector3()

onMounted(async () => {
  await nextTick()
  const q = new URLSearchParams(location.search)
  if (q.has('rot')) rotY = parseFloat(q.get('rot')) || 0
  init()
  rebuild()
  if (props.active) intro = true
  if (location.search.includes('nointro')) {
    intro = true
    introT = 1
  }
  // 调试：?intro=0.4 冻结在入场动画的某个进度
  if (q.has('intro')) {
    freezeIntro = Math.min(1, Math.max(0, parseFloat(q.get('intro')) || 0))
    intro = false
    introT = freezeIntro
  }
  window.addEventListener('resize', onResize)
  raf = requestAnimationFrame(tick)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', onResize)
  if (renderer) renderer.dispose()
  disposePoints(points)
  if (stars) for (const p of stars.children) disposePoints(p)
  for (const p of dustLayers) disposePoints(p)
})

watch(
  () => props.active,
  (v) => {
    if (v) intro = true
  },
)

watch(
  () => props.events,
  () => {
    if (renderer) rebuild()
  },
  { deep: false },
)

function disposePoints(p) {
  if (!p) return
  p.geometry.dispose()
  p.material.dispose()
}

// 柔光圆点：星屑、雾、辉光都用它
function dotTexture() {
  const s = 64
  const c = document.createElement('canvas')
  c.width = c.height = s
  const g = c.getContext('2d')
  const grd = g.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2)
  grd.addColorStop(0, 'rgba(255,255,255,1)')
  grd.addColorStop(0.34, 'rgba(255,255,255,0.5)')
  grd.addColorStop(0.7, 'rgba(255,255,255,0.1)')
  grd.addColorStop(1, 'rgba(255,255,255,0)')
  g.fillStyle = grd
  g.fillRect(0, 0, s, s)
  return new THREE.CanvasTexture(c)
}

// 核心星芒：一束竖直长芒 + 一束横向短芒 + 一层柔核。
// 参考图里核心不是一团糊光，而是有明确的十字衍射芒。
function spikeTexture() {
  const S = 512
  const c = document.createElement('canvas')
  c.width = c.height = S
  const g = c.getContext('2d')
  const img = g.createImageData(S, S)
  const d = img.data
  const half = S / 2
  const gauss = (v, w) => Math.exp(-(v * v) / (2 * w * w))

  // 逐像素算：横向高斯决定光柱宽度，纵向指数决定长度。
  // 用 canvas 的径向渐变画不出这种"宽度恒定、末端渐隐"的光柱，
  // 只能自己铺像素。
  for (let y = 0; y < S; y++) {
    const ny = (y - half) / half
    for (let x = 0; x < S; x++) {
      const nx = (x - half) / half

      // 纹理边界要归零，否则 sprite 会在边缘被硬切一刀
      const fadeY = Math.pow(Math.max(0, 1 - ny * ny), 0.4)
      const fadeX = Math.pow(Math.max(0, 1 - nx * nx), 0.4)

      // 竖直长芒：向上比向下更长
      const v = gauss(nx, 0.05) * (ny < 0 ? Math.exp(ny * 2.5) : Math.exp(-ny * 3.4)) * fadeY
      // 横向短芒
      const h = gauss(ny, 0.036) * Math.exp(-Math.abs(nx) * 3.6) * fadeX
      // 柔核
      const cr = Math.sqrt(nx * nx + ny * ny * 1.3)
      const core = Math.exp(-(cr * cr) / (2 * 0.027 * 0.027))

      let a = v * 0.78 + h * 0.34 + core * 0.34
      if (a > 1) a = 1

      const i = (y * S + x) * 4
      d[i] = 255
      d[i + 1] = 247 - 26 * (1 - a)
      d[i + 2] = 251 - 12 * (1 - a)
      d[i + 3] = a * 255
    }
  }
  g.putImageData(img, 0, 0)
  const tex = new THREE.CanvasTexture(c)
  tex.minFilter = THREE.LinearFilter
  return tex
}

const smooth = (x) => {
  const t = Math.min(1, Math.max(0, x))
  return t * t * (3 - 2 * t)
}

// 光晕亮度：和片元着色器里的 haze 同一套公式（去掉噪声，取期望值）。
// 装饰星点靠它决定该不该亮——光晕淡的地方星点也淡，
// 这样星点看起来是"长在星云上"的，而不是撒在虚空里。
function hazeAt(r) {
  return (0.095 / Math.sqrt(1 + 0.055 * r * r)) * smooth((r - 0.3) / 0.7)
}

function randn() {
  let u = 0
  let v = 0
  while (u === 0) u = Math.random()
  while (v === 0) v = Math.random()
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v)
}

// 视角相关的尺度：盘面半径随视口变化，始终溢出四边
function fitMetrics() {
  const halfH = Math.tan(((FOV / 2) * Math.PI) / 180) * BASE_CAMZ
  const halfW = halfH * (W / H)
  const corner = Math.sqrt(halfW * halfW + (halfH * halfH) / (Math.sin(TILT_X) * Math.sin(TILT_X)))
  diskR = corner * DISK_OVER
  // 核心跟着盘面按比例走（0.437 ≈ 原来 1.42 倍时的观感），
  // 否则盘面一拉远，核心在画面里就显得过大
  coreR = diskR * 0.437
  planeR = diskR * FIELD_OVER
  visR = 1 / DISK_OVER // 画面四角在盘面坐标里的半径
  eventR = Math.min(halfW * 0.82, halfH * 1.5)
  armOffset = SPIRAL_K * Math.log(eventR / diskR)
}

// 旋臂中心线上的云团
function makeClumps(count) {
  const list = []
  // 云团一直撒到亮盘的外缘（r≈1.06），别让最外圈只剩一层薄雾没有颗粒
  const rMax = visR * 1.33
  for (let i = 0; i < count; i++) {
    const arm = i % 2
    const r = 0.08 + (rMax - 0.08) * Math.pow(Math.random(), 0.76)
    const phi = arm * Math.PI + SPIRAL_K * Math.log(r / SPIRAL_R0) + randn() * 0.05
    list.push([r, phi, 0.01 + Math.random() * 0.024])
  }
  return list
}

function armPhi(r, arm) {
  return arm * Math.PI + SPIRAL_K * Math.log(Math.max(0.02, r) / SPIRAL_R0)
}

// 盘面配色：核心暖白 → 淡粉主体 → 外缘藕紫，臂间掺一点青、少量暖金
function diskColor(r, out) {
  if (r < 0.06) {
    out.setRGB(1.0, 0.96, 0.96)
    return
  }
  const t = Math.random()
  if (r < 0.17) {
    if (t < 0.66) out.setRGB(1.0, 0.83, 0.9)
    else if (t < 0.9) out.setRGB(1.0, 0.93, 0.93)
    else out.setRGB(0.7, 0.94, 0.97)
    return
  }
  if (r < 0.42) {
    if (t < 0.56) out.setRGB(1.0, 0.72, 0.84)
    else if (t < 0.76) out.setRGB(1.0, 0.86, 0.78)
    else if (t < 0.9) out.setRGB(0.9, 0.72, 0.98)
    else out.setRGB(0.64, 0.93, 0.98)
    return
  }
  if (t < 0.46) out.setRGB(0.97, 0.62, 0.8)
  else if (t < 0.7) out.setRGB(0.9, 0.68, 0.9)
  else if (t < 0.87) out.setRGB(1.0, 0.78, 0.6)
  else out.setRGB(0.6, 0.92, 0.97)
}

// 所有点共用的着色器：近点距离钳制，避免透视把前景点放大成虚化泡泡
function pointMaterial(opts = {}) {
  return new THREE.ShaderMaterial({
    uniforms: {
      uPixelRatio: { value: renderer.getPixelRatio() },
      uTime: { value: 0 },
      uDistMin: { value: opts.distMin ?? 2.0 },
      uTwinkle: { value: opts.twinkle ?? 0 },
      uGlow: { value: opts.glow ?? 1 },
      uIntro: { value: 1 },
      uOpacity: { value: 0 },
    },
    vertexShader: `
      attribute float aSize;
      attribute float aAlpha;
      attribute vec3 aColor;
      attribute vec3 aFrom;
      attribute float aDelay;
      uniform float uPixelRatio;
      uniform float uIntro;
      uniform float uTime;
      uniform float uDistMin;
      uniform float uTwinkle;
      uniform float uGlow;
      uniform float uOpacity;
      varying vec3 vColor;
      varying float vAlpha;
      void main() {
        vColor = aColor;
        // 入场：从散乱点螺旋落入盘面，逐点错开形成"聚拢"的层次
        float t = clamp((uIntro - aDelay * 0.26) / 0.74, 0.0, 1.0);
        t = t * t * (3.0 - 2.0 * t);
        vec3 p = mix(aFrom, position, t);
        float ang = (1.0 - t) * 2.4;
        float ca = cos(ang);
        float sa = sin(ang);
        p.xz = mat2(ca, -sa, sa, ca) * p.xz;

        vec4 mv = modelViewMatrix * vec4(p, 1.0);
        float dist = max(uDistMin, -mv.z);
        float tw = 1.0 - uTwinkle * (0.5 - 0.5 * sin(uTime * 1.8 + p.x * 23.0 + p.z * 17.0));
        // 摊开时点更稀，用亮度补偿，整体明度别掉太多
        float gain = mix(1.75, 1.0, t);
        vAlpha = aAlpha * uOpacity * tw * gain * smoothstep(0.0, 0.05, uIntro);
        gl_PointSize = aSize * uPixelRatio * (${SIZE_K.toFixed(1)} / dist);
        gl_Position = projectionMatrix * mv;
      }
    `,
    fragmentShader: `
      varying vec3 vColor;
      varying float vAlpha;
      uniform float uGlow;
      void main() {
        float d = length(gl_PointCoord - vec2(0.5));
        if (d > 0.5) discard;
        float core = smoothstep(0.15, 0.0, d);
        float glow = pow(smoothstep(0.5, 0.05, d), 2.0) * 0.8;
        vec3 c = mix(vColor, vec3(1.0), core * 0.26);
        float a = (core * 0.82 + glow * uGlow) * vAlpha;
        if (a < 0.004) discard;
        gl_FragColor = vec4(c * (1.0 + 0.7 * core), min(1.0, a));
      }
    `,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
}

// count 个点：sample 返回世界坐标，size/alpha 支持区间随机
function makePoints(count, sample, opts = {}) {
  const pos = new Float32Array(count * 3)
  const from = new Float32Array(count * 3)
  const delay = new Float32Array(count)
  const col = new Float32Array(count * 3)
  const size = new Float32Array(count)
  const alpha = new Float32Array(count)
  const c = new THREE.Color()
  const sMin = opts.size?.[0] ?? opts.size ?? 0.03
  const sMax = opts.size?.[1] ?? opts.size ?? 0.03
  const aMin = opts.alpha?.[0] ?? opts.alpha ?? 1
  const aMax = opts.alpha?.[1] ?? opts.alpha ?? 1

  const scatter = opts.scatter !== false

  for (let i = 0; i < count; i++) {
    const p = sample()
    pos[i * 3] = p[0]
    pos[i * 3 + 1] = p[1]
    pos[i * 3 + 2] = p[2]
    if (scatter) {
      // 起点：把一个"膨胀且打散"的星系摊开，聚拢时收紧、绕回旋臂
      // 越靠外的点飞得越远，所以核心先成形、旋臂后落位
      const len = Math.hypot(p[0], p[1], p[2])
      const r = len * (1.12 + (0.58 * len) / diskR) * (0.96 + Math.random() * 0.08)
      const th = Math.random() * Math.PI * 2
      const u = Math.random() * 2 - 1
      const sr = Math.sqrt(Math.max(0, 1 - u * u))
      from[i * 3] = r * sr * Math.cos(th) * 0.34 + p[0] * 0.66
      from[i * 3 + 1] = r * u * 0.4 + p[1] * 0.55
      from[i * 3 + 2] = r * sr * Math.sin(th) * 0.34 + p[2] * 0.66
      delay[i] = Math.min(1, len / diskR) * 0.5 + Math.random() * 0.3
    } else {
      from[i * 3] = p[0]
      from[i * 3 + 1] = p[1]
      from[i * 3 + 2] = p[2]
      delay[i] = 0
    }
    opts.color(c, p)
    col[i * 3] = c.r
    col[i * 3 + 1] = c.g
    col[i * 3 + 2] = c.b
    size[i] = sMin + Math.random() * (sMax - sMin)
    alpha[i] = aMin + Math.random() * (aMax - aMin)
  }

  const g = new THREE.BufferGeometry()
  g.setAttribute('position', new THREE.BufferAttribute(pos, 3))
  g.setAttribute('aColor', new THREE.BufferAttribute(col, 3))
  g.setAttribute('aFrom', new THREE.BufferAttribute(from, 3))
  g.setAttribute('aDelay', new THREE.BufferAttribute(delay, 1))
  g.setAttribute('aSize', new THREE.BufferAttribute(size, 1))
  g.setAttribute('aAlpha', new THREE.BufferAttribute(alpha, 1))
  const m = pointMaterial(opts)
  return new THREE.Points(g, m)
}

function init() {
  const el = wrapRef.value
  W = el.clientWidth || window.innerWidth
  H = el.clientHeight || window.innerHeight
  fitMetrics()

  scene = new THREE.Scene()

  camera = new THREE.PerspectiveCamera(FOV, W / H, 0.1, 300)
  camera.position.set(0, 0, camZ)

  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  renderer.setSize(W, H)
  renderer.setClearColor(0x000000, 0)
  el.appendChild(renderer.domElement)

  buildStarField()

  // 倾斜层：先绕 X 抬起视角，再整体滚转
  tiltGroup = new THREE.Group()
  tiltGroup.rotation.z = TILT_Z
  scene.add(tiltGroup)

  // 自转层：绕盘面自身的法线转（真正的“星系自转”）
  spinGroup = new THREE.Group()
  tiltGroup.add(spinGroup)

  // 星系整体的暖雾，让四角不至于死黑
  frameGlow = new THREE.Sprite(
    new THREE.SpriteMaterial({
      map: dotTexture(),
      color: 0x7a2c50,
      transparent: true,
      opacity: 0.04,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  frameGlow.scale.set(diskR * 12, diskR * 8.5, 1)
  frameGlow.material.rotation = TILT_Z
  scene.add(frameGlow)

  // 大范围暖橙弥散，参考图里盘面整体笼着一层橙光
  coreHalo = new THREE.Sprite(
    new THREE.SpriteMaterial({
      map: dotTexture(),
      color: 0xffaecb,
      transparent: true,
      opacity: 0.1,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  coreHalo.material.rotation = TILT_Z
  scene.add(coreHalo)

  // 核心附近的高光，压在星芒下面
  coreGlow = new THREE.Sprite(
    new THREE.SpriteMaterial({
      map: dotTexture(),
      color: 0xfff2f7,
      transparent: true,
      opacity: 0.16,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  coreGlow.material.rotation = TILT_Z
  scene.add(coreGlow)

  // 十字星芒：最上层
  coreSpikes = new THREE.Sprite(
    new THREE.SpriteMaterial({
      map: spikeTexture(),
      color: 0xfff5fa,
      transparent: true,
      opacity: 0.6,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    }),
  )
  coreSpikes.material.rotation = 0.07
  scene.add(coreSpikes)

  buildGalaxy()
  bindPointer()
}

function buildStarField() {
  const mk = (count, size, bright) =>
    makePoints(
      count,
      () => {
        const r = 9 + Math.random() * 26
        const th = Math.random() * Math.PI * 2
        const ph = Math.acos(2 * Math.random() - 1)
        return [
          r * Math.sin(ph) * Math.cos(th),
          r * Math.sin(ph) * Math.sin(th),
          r * Math.cos(ph),
        ]
      },
      {
        size,
        distMin: 6,
        scatter: false,
        alpha: bright ? [0.4, 0.8] : [0.14, 0.46],
        // 背景是冷蓝黑，跟暖色的星系形成对比
        color: (c) => {
          const t = Math.random()
          if (t < 0.66) c.setRGB(0.84, 0.9, 1.0)
          else if (t < 0.84) c.setRGB(1.0, 0.88, 0.94)
          else c.setRGB(0.68, 0.95, 1.0)
        },
      },
    )
  stars = new THREE.Group()
  stars.add(mk(1500, [0.05, 0.085], false))
  stars.add(mk(120, [0.1, 0.15], true))
  scene.add(stars)
}

// 解析星系场：指数盘 + 对数旋臂 + 暗尘埃带 + 核球，
// 用一个平面算出一整层连续辉光（点云只负责颗粒感）
function buildField() {
  const g = new THREE.PlaneGeometry(2, 2)
  const m = new THREE.ShaderMaterial({
    uniforms: {
      uOpacity: { value: 0 },
      uK: { value: SPIRAL_K },
      uR0: { value: SPIRAL_R0 },
      uRad: { value: planeR / diskR },
      uCore: { value: diskR / coreR },
    },
    vertexShader: `
      varying vec2 vP;
      varying vec3 vN;
      varying vec3 vV;
      void main() {
        vP = position.xy;
        // 盘面法线与视线（视图空间）：算掠射角用
        vN = normalize(normalMatrix * vec3(0.0, 0.0, 1.0));
        vec4 mv = modelViewMatrix * vec4(position, 1.0);
        vV = mv.xyz;
        gl_Position = projectionMatrix * mv;
      }
    `,
    fragmentShader: `
      precision highp float;
      varying vec2 vP;
      varying vec3 vN;
      varying vec3 vV;
      uniform float uOpacity;
      uniform float uK;
      uniform float uR0;
      uniform float uRad;
      uniform float uCore;

      float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
      float noise(vec2 p) {
        vec2 i = floor(p);
        vec2 f = fract(p);
        f = f * f * (3.0 - 2.0 * f);
        float a = hash(i);
        float b = hash(i + vec2(1.0, 0.0));
        float c = hash(i + vec2(0.0, 1.0));
        float d = hash(i + vec2(1.0, 1.0));
        return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
      }
      float fbm(vec2 p) {
        float s = 0.0;
        float a = 0.5;
        for (int i = 0; i < 4; i++) {
          s += a * noise(p);
          p *= 2.03;
          a *= 0.5;
        }
        return s;
      }

      void main() {
        // q 是归一化到 diskR 的盘面坐标：1.0 就是星系外缘，与平面几何尺寸解耦
        vec2 q = vP * uRad;
        float r = length(q);
        float phi = atan(q.y, q.x);

        float disk = exp(-r / 0.5);
        float outer = exp(-r / 0.72) * 0.075 * smoothstep(0.26, 0.7, r);
        // 核心不跟着盘面一起放大，保持一颗小而亮的心
        float rc = r * uCore;
        float bulge = exp(-rc * rc / 0.0062);
        // 核球外的一圈亮环
        float ring = exp(-pow((rc - 0.085) / 0.04, 2.0)) * 0.5;

        // 旋臂相位：两条对数螺线
        float sph = phi - uK * log(max(r, 0.04) / uR0);
        float arms = pow(0.5 + 0.5 * cos(2.0 * sph), 4.2);
        arms *= smoothstep(0.06, 0.32, r);
        arms *= 0.55 + 0.85 * fbm(q * 15.0 + 7.0);

        // 暗尘埃带：相位略偏移、比旋臂窄，用噪声拉成丝
        float dust = pow(0.5 + 0.5 * cos(2.0 * (sph + 1.15)), 7.0);
        dust *= smoothstep(0.08, 0.3, r) * smoothstep(1.05, 0.55, r);
        dust *= 0.4 + 0.9 * fbm(q * 6.4 + 11.0);
        dust = clamp(dust * 1.0, 0.0, 0.9);

        float cloud = 0.6 + 0.56 * fbm(q * 9.0);

        // 淡粉为主体：核心暖白 → 内盘淡粉 → 中盘玫粉 → 外缘藕紫
        vec3 cCore = vec3(1.0, 0.96, 0.96);
        vec3 cPink = vec3(1.0, 0.8, 0.88);
        vec3 cRose = vec3(0.97, 0.63, 0.8);
        vec3 cMauve = vec3(0.88, 0.58, 0.82);
        vec3 col = mix(cCore, cPink, smoothstep(0.02, 0.13, rc));
        col = mix(col, cRose, smoothstep(0.1, 0.46, r));
        col = mix(col, cMauve, smoothstep(0.5, 0.98, r));

        // 中盘留一条暖金的带，粉里透一点原来的暖调
        float warm = smoothstep(0.2, 0.4, r) * smoothstep(0.68, 0.42, r);
        col = mix(col, vec3(1.0, 0.82, 0.62), warm * 0.46);

        // 臂间掺一点青，别让整块盘只剩一种粉
        float cyan = smoothstep(0.5, 0.86, noise(q * 7.1 + 23.0)) * smoothstep(0.12, 0.44, r);
        col = mix(col, vec3(0.54, 0.93, 0.97), cyan * 0.62);

        float b = bulge * 0.52 + ring * 0.2 + (disk * 0.2 + arms * disk * 0.74 + outer) * cloud;
        b *= smoothstep(1.03, 0.62, r); // 长长的外缘渐隐，融进深空
        b *= 1.0 - dust;                // 暗尘埃带：把盘面切出丝状暗纹
        b *= 1.32;

        // 星系光晕：指数盘在 r>1 就归零，但画面上边对应盘面半径 4.6、右上角 33，
        // 那些地方永远是黑的。换成 1/sqrt(1+r²) 的慢衰减气体，一直铺到边框之外，
        // 让整个星系连同它的晕都在画面里，没有一块死黑。
        float haze = 0.095 / sqrt(1.0 + 0.055 * r * r);
        haze *= smoothstep(0.30, 1.0, r);
        haze *= 0.35 + 1.1 * fbm(q * 1.55 + 41.0);
        // 掠射补偿：贴着地平线看盘面时，视线穿过的盘面厚度是正面的十几倍，
        // 那一带本该更亮（也是真实星系照片里远岸最亮的原因）。
        // 没有这一项，右上角这种接近地平线的地方永远是黑的。
        float cosv = abs(dot(normalize(vN), normalize(-vV)));
        haze *= clamp(0.52 / max(0.05, cosv), 1.0, 2.8);
        // 光晕偏藕紫，局部透青，比纯粉更像真实星云的稀薄气体
        vec3 hazeCol = mix(vec3(0.9, 0.58, 0.83), vec3(0.5, 0.84, 0.96),
                           smoothstep(0.42, 0.8, noise(q * 2.1 + 9.0)));

        gl_FragColor = vec4((col * b + hazeCol * haze) * uOpacity, 1.0);
      }
    `,
    transparent: true,
    depthWrite: false,
    depthTest: false,
    blending: THREE.AdditiveBlending,
  })
  const mesh = new THREE.Mesh(g, m)
  mesh.rotation.x = -Math.PI / 2
  mesh.scale.setScalar(planeR)
  mesh.renderOrder = -1
  spinGroup.add(mesh)
  return mesh
}

// 星系本体：解析场 + 柔光雾 + 密集星屑 + 旋臂云团 + 核球 + 外晕
function buildGalaxy() {
  for (const p of dustLayers) {
    if (p.parent) p.parent.remove(p)
    disposePoints(p)
  }
  dustLayers = [buildField()]

  const clumps = makeClumps(560)

  // 柔光雾：大而暗的点，把旋臂泡在暖雾里
  const glow = makePoints(
    6000,
    () => {
      const r = 0.04 + visR * 1.33 * Math.pow(Math.random(), 0.5)
      const th = Math.random() * Math.PI * 2
      return [r * Math.cos(th) * diskR, randn() * 0.035 * diskR, r * Math.sin(th) * diskR]
    },
    {
      size: [0.07, 0.145],
      alpha: [0.022, 0.055],
      distMin: 2.6,
      color: (c, p) => {
        const r = Math.hypot(p[0], p[2]) / diskR
        const w = Math.min(1, Math.max(0, (r - 0.1) / 0.34))
        c.setRGB(1.0 - 0.08 * w, 0.9 - 0.24 * w, 0.87 - 0.06 * w)
      },
    },
  )

  // 密集小星屑：参考图最抢眼的就是这一层，颗粒感全靠它
  const sparks = makePoints(
    40000,
    () => {
      let r
      let phi
      const t = Math.random()
      if (t < 0.15) {
        r = 0.04 + visR * 1.33 * Math.pow(Math.random(), 0.55)
        phi = Math.random() * Math.PI * 2
      } else if (t < 0.62) {
        const c = clumps[(Math.random() * clumps.length) | 0]
        r = c[0] * (1 + randn() * c[2] * 1.5)
        phi = c[1] + (randn() * c[2] * 3.2) / Math.max(0.14, c[0])
      } else {
        r = 0.04 + visR * 1.33 * Math.pow(Math.random(), 0.62)
        phi = armPhi(r, Math.random() < 0.5 ? 0 : 1) + randn() * (0.08 + 0.15 * r)
      }
      const y = randn() * 0.016 * (1 - 0.45 * r)
      return [r * Math.cos(phi) * diskR, y * diskR, r * Math.sin(phi) * diskR]
    },
    {
      size: [0.012, 0.024],
      alpha: [0.24, 0.72],
      distMin: 2.4,
      color: (c, p) => {
        const r = Math.hypot(p[0], p[2]) / diskR
        diskColor(r, c)
        // 外缘不要一刀切黑：慢慢淡到底，跟 haze 一起收边
        const f = Math.max(0.1, 1 - smooth((r - 0.58) / 0.52))
        c.multiplyScalar(0.25 + 0.75 * f)
      },
    },
  )

  // 旋臂云团：稍大一点的点，负责把旋臂"堆"出厚度
  const armFuzz = makePoints(
    6800,
    () => {
      const c = clumps[(Math.random() * clumps.length) | 0]
      const r = c[0] * (1 + randn() * c[2] * 1.6)
      const phi = c[1] + (randn() * c[2] * 3.4) / Math.max(0.16, c[0])
      return [r * Math.cos(phi) * diskR, randn() * 0.02 * diskR, r * Math.sin(phi) * diskR]
    },
    {
      size: [0.042, 0.085],
      alpha: [0.016, 0.042],
      distMin: 2.6,
      color: (c, p) => {
        const r = Math.hypot(p[0], p[2]) / diskR
        if (r < 0.2) c.setRGB(1.0, 0.87, 0.87)
        else if (r < 0.45) c.setRGB(1.0, 0.74, 0.84)
        else c.setRGB(0.92, 0.64, 0.88)
      },
    },
  )

  // 核球
  const bulge = makePoints(
    4200,
    () => {
      const r = Math.pow(Math.random(), 2.4) * 0.16
      const th = Math.random() * Math.PI * 2
      const u = randn() * 0.5
      const s = Math.sqrt(Math.max(0, 1 - u * u))
      return [r * s * Math.cos(th) * diskR, r * u * 0.42 * diskR, r * s * Math.sin(th) * diskR]
    },
    {
      size: [0.028, 0.052],
      alpha: [0.06, 0.22],
      distMin: 2.6,
      color: (c, p) => {
        const r = Math.hypot(p[0], p[1], p[2]) / coreR
        c.setRGB(1.0, 0.95 - 0.06 * r, 0.91 - 0.06 * r)
      },
    },
  )

  // 外晕：稀薄的暖粉气体，把盘面边缘接到光晕里。
  // 强度同样跟 haze 走，保证它和解析场的晕是一整块，不出现两层边界
  const halo = makePoints(
    2800,
    () => {
      const r = visR * 0.5 + Math.pow(Math.random(), 0.9) * visR * 1.2
      const th = Math.random() * Math.PI * 2
      const u = Math.random() * 2 - 1
      const s = Math.sqrt(Math.max(0, 1 - u * u))
      return [r * s * Math.cos(th) * diskR, r * u * 0.45 * diskR, r * s * Math.sin(th) * diskR]
    },
    {
      size: [0.03, 0.055],
      alpha: [0.05, 0.16],
      distMin: 2.6,
      color: (c, p) => {
        const r = Math.hypot(p[0], p[2]) / diskR
        const f = Math.max(0.22, Math.min(1, hazeAt(r) / 0.078))
        c.setRGB(0.92 * f, 0.58 * f, 0.8 * f)
      },
    },
  )

  // 装饰星点：沿旋臂相位排，不是随机撒。半径一直拉到亮盘之外（7.5 个盘单位），
  // 越外越稀——画面上边和右上角那片没有亮臂的空档，由"旋臂余波"点状接力填上，
  // 图案和盘内保持连续，所以看起来是星光的延伸，不是撒了一把碎点
  const rim = makePoints(
    36000,
    () => {
      const r = 0.85 + 8.2 * Math.pow(Math.random(), 1.9)
      const arm = Math.random() < 0.5 ? 0 : 1
      // 大部分点贴着臂线，少部分散在臂间，避免排成一条死线
      const phi =
        Math.random() < 0.78
          ? armPhi(r, arm) + randn() * (0.05 + 0.085 * r)
          : Math.random() * Math.PI * 2
      const y = randn() * 0.022 * (0.7 + 0.22 * r) * diskR
      return [r * Math.cos(phi) * diskR, y, r * Math.sin(phi) * diskR]
    },
    {
      size: [0.011, 0.026],
      alpha: [0.08, 0.24],
      distMin: 2.6,
      color: (c, p) => {
        const r = Math.hypot(p[0], p[2]) / diskR
        const t = Math.random()
        if (t < 0.5) c.setRGB(0.98, 0.76, 0.88)
        else if (t < 0.8) c.setRGB(0.86, 0.7, 0.97)
        else c.setRGB(0.62, 0.9, 0.98)
        // 亮度跟光晕一起收，但留个下限：外圈那层接力要看得见
        c.multiplyScalar(Math.max(0.42, Math.min(1, hazeAt(r) / 0.09)))
      },
    },
  )

  dustLayers.push(glow, armFuzz, sparks, bulge, halo, rim)
  for (const p of dustLayers) if (!p.parent) spinGroup.add(p)
}

function rebuild() {
  const list = props.events || []
  n = list.length
  if (!n) {
    if (points) {
      spinGroup.remove(points)
      disposePoints(points)
      points = null
    }
    return
  }

  baseSize = n <= 16 ? 0.21 : n <= 40 ? 0.17 : n <= 100 ? 0.14 : 0.11

  targetPos = new Float32Array(n * 3)
  startPos = new Float32Array(n * 3)
  curPos = new Float32Array(n * 3)
  aColorArr = new Float32Array(n * 3)
  aSizeArr = new Float32Array(n)
  aAlphaArr = new Float32Array(n)
  curSize = new Float32Array(n)
  tgtSize = new Float32Array(n)

  // 事件盘的半径比尘埃盘小，要让它的旋臂和尘埃盘对上，得补一个相位
  const ca = Math.cos(armOffset)
  const sa = Math.sin(armOffset)
  const c = new THREE.Color()
  for (let i = 0; i < n; i++) {
    const e = list[i]
    let x = e.x
    let y = e.y
    let z = e.z
    if (!x && !y && !z) {
      const r = 0.2 + 0.72 * Math.pow(Math.random(), 0.68)
      const phi = armPhi(r, Math.random() < 0.5 ? 0 : 1) + randn() * 0.09
      x = r * Math.cos(phi)
      y = randn() * 0.012
      z = r * Math.sin(phi)
    }
    const rx = x * ca + z * sa
    const rz = -x * sa + z * ca
    targetPos[i * 3] = rx * eventR
    targetPos[i * 3 + 1] = y * eventR
    targetPos[i * 3 + 2] = rz * eventR

    const prev = prevPos.get(e.id)
    if (prev) {
      startPos[i * 3] = prev[0]
      startPos[i * 3 + 1] = prev[1]
      startPos[i * 3 + 2] = prev[2]
    } else {
      const j = 0.06
      startPos[i * 3] = (Math.random() - 0.5) * j
      startPos[i * 3 + 1] = (Math.random() - 0.5) * j
      startPos[i * 3 + 2] = (Math.random() - 0.5) * j
    }
    curPos[i * 3] = startPos[i * 3]
    curPos[i * 3 + 1] = startPos[i * 3 + 1]
    curPos[i * 3 + 2] = startPos[i * 3 + 2]

    // 事件星：粉／青／暖白交替，在暖金色盘面上很跳
    const t = (i * 0.6180339887) % 1
    if (t < 0.4) c.setRGB(0.62, 0.98, 1.0)
    else if (t < 0.72) c.setRGB(1.0, 0.86, 0.52)
    else c.setRGB(1.0, 0.99, 0.99)
    aColorArr[i * 3] = c.r
    aColorArr[i * 3 + 1] = c.g
    aColorArr[i * 3 + 2] = c.b

    curSize[i] = baseSize
    tgtSize[i] = baseSize
    aSizeArr[i] = baseSize
    aAlphaArr[i] = 0
  }

  prevPos.clear()
  for (let i = 0; i < n; i++) {
    prevPos.set(list[i].id, [targetPos[i * 3], targetPos[i * 3 + 1], targetPos[i * 3 + 2]])
  }

  if (points) {
    spinGroup.remove(points)
    disposePoints(points)
  }
  geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(curPos, 3))
  geo.setAttribute('aSize', new THREE.BufferAttribute(aSizeArr, 1))
  geo.setAttribute('aAlpha', new THREE.BufferAttribute(aAlphaArr, 1))
  geo.setAttribute('aColor', new THREE.BufferAttribute(aColorArr, 3))
  mat = pointMaterial({ twinkle: 0.55, distMin: 2.2, glow: 1.9 })
  points = new THREE.Points(geo, mat)
  spinGroup.add(points)

  // 不要重置 introT：保存后重建几何时不该重播整段聚拢动画
  if (location.search.includes('nointro')) introT = 1
  hoverIdx = -1
}

let resizeTimer = 0
function onResize() {
  const el = wrapRef.value
  if (!el || !renderer) return
  W = el.clientWidth
  H = el.clientHeight
  camera.aspect = W / H
  camera.updateProjectionMatrix()
  renderer.setSize(W, H)
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    fitMetrics()
    frameGlow.scale.set(diskR * 12, diskR * 8.5, 1)
    buildGalaxy()
    rebuild()
  }, 220)
}

function bindPointer() {
  const dom = renderer.domElement

  dom.addEventListener('pointerdown', (e) => {
    dragging = true
    moved = 0
    lastPx = e.clientX
    lastPy = e.clientY
    const rect = dom.getBoundingClientRect()
    pointerX = e.clientX - rect.left
    pointerY = e.clientY - rect.top
    dom.setPointerCapture(e.pointerId)
  })

  dom.addEventListener('pointermove', (e) => {
    const rect = dom.getBoundingClientRect()
    pointerX = e.clientX - rect.left
    pointerY = e.clientY - rect.top

    if (dragging) {
      const dx = e.clientX - lastPx
      const dy = e.clientY - lastPy
      lastPx = e.clientX
      lastPy = e.clientY
      // 倾角固定，竖直方向的拖动只计入位移量，不改视角
      moved += Math.abs(dx) + Math.abs(dy)
      rotY += dx * 0.0055
      velY = dx * 0.0015
      return
    }

    // 悬停只更新高亮，不改变自转速度
    setHover(pick(pointerX, pointerY))
  })

  const end = (e) => {
    if (!dragging) return
    dragging = false
    if (moved < 6) {
      const idx = pick(pointerX, pointerY, true)
      if (idx >= 0) {
        const ev = props.events[idx]
        if (ev) emit('open', ev)
      } else if (nearCore(pointerX, pointerY)) {
        // 点中间那团光 → 星尾尘
        emit('core')
      }
    }
    try {
      dom.releasePointerCapture(e.pointerId)
    } catch {
      /* noop */
    }
  }
  dom.addEventListener('pointerup', end)
  dom.addEventListener('pointercancel', end)

  dom.addEventListener('pointerleave', () => {
    setHover(-1)
  })

  dom.addEventListener(
    'wheel',
    (e) => {
      e.preventDefault()
      camZT = Math.min(5.4, Math.max(1.7, camZT + e.deltaY * 0.0012))
    },
    { passive: false },
  )
}

function nearCore(mx, my) {
  if (introT < 0.85) return false
  return (mx - coreSX) ** 2 + (my - coreSY) ** 2 < CORE_HIT * CORE_HIT
}

function setHover(idx) {
  const el = renderer.domElement
  if (idx === hoverIdx) {
    if (idx < 0) el.style.cursor = nearCore(pointerX, pointerY) ? 'pointer' : 'grab'
    return
  }
  hoverIdx = idx
  if (idx >= 0) el.style.cursor = 'pointer'
  else el.style.cursor = nearCore(pointerX, pointerY) ? 'pointer' : 'grab'
  if (idx >= 0 && props.events[idx]) {
    const e = props.events[idx]
    tooltip.value = {
      show: true,
      x: pointerX,
      y: pointerY,
      date: prettyDate(e.date),
      title: e.title,
    }
  } else {
    tooltip.value.show = false
  }
}

function pick(mx, my, force = false) {
  if (!points || !n) return -1
  if (!force && (introT < 0.85 || camZ > 5.2)) return -1
  points.updateMatrixWorld()
  let best = -1
  let bestD = 23 * 23
  for (let i = 0; i < n; i++) {
    v3.set(curPos[i * 3], curPos[i * 3 + 1], curPos[i * 3 + 2])
    v3.applyMatrix4(points.matrixWorld)
    v3.project(camera)
    if (v3.z > 1.2) continue
    const sx = (v3.x * 0.5 + 0.5) * W
    const sy = (-v3.y * 0.5 + 0.5) * H
    const d = (sx - mx) * (sx - mx) + (sy - my) * (sy - my)
    if (d < bestD) {
      bestD = d
      best = i
    }
  }
  return best
}

let last = 0
function tick(now) {
  raf = requestAnimationFrame(tick)
  const dt = last ? Math.min(0.05, (now - last) / 1000) : 0.016
  last = now
  // 切到「星途」时不再跑渲染，回来时 time 不会跳
  if (props.paused) return
  const k = dt * 60

  if (freezeIntro >= 0) introT = freezeIntro
  else if (intro) introT = Math.min(1, introT + dt / 2.2)

  // 尘埃先聚拢，事件星等尘埃落定后再从核心散出去
  const evEase = smooth((introT - 0.55) / 0.45)
  // 解析辉光场随聚拢一起亮起来
  const fieldT = smooth(introT / 0.55)

  if (points) {
    for (let i = 0; i < n; i++) {
      const i3 = i * 3
      curPos[i3] = startPos[i3] + (targetPos[i3] - startPos[i3]) * evEase
      curPos[i3 + 1] = startPos[i3 + 1] + (targetPos[i3 + 1] - startPos[i3 + 1]) * evEase
      curPos[i3 + 2] = startPos[i3 + 2] + (targetPos[i3 + 2] - startPos[i3 + 2]) * evEase
      aAlphaArr[i] = evEase

      const isHover = i === hoverIdx
      tgtSize[i] = baseSize * (isHover ? 2.2 : 1)
      curSize[i] += (tgtSize[i] - curSize[i]) * Math.min(1, 0.18 * k)
      aSizeArr[i] = curSize[i]
    }
    geo.attributes.position.needsUpdate = true
    geo.attributes.aSize.needsUpdate = true
    geo.attributes.aAlpha.needsUpdate = true
  }

  // 自转：只要没在拖动就一直缓慢顺时针转（悬停也不会停）。
  // 拖动时让位给鼠标，松手后惯性滑回正常转速。
  if (!dragging) {
    rotY += velY * k
    velY *= Math.pow(0.91, k)
    rotY -= 0.0021 * k
  }

  zoomK = camZ / BASE_CAMZ

  camOffT.x = (pointerX / Math.max(1, W) - 0.5) * 0.42
  camOffT.y = -(pointerY / Math.max(1, H) - 0.5) * 0.42
  camOff.x += (camOffT.x - camOff.x) * Math.min(1, 0.05 * k)
  camOff.y += (camOffT.y - camOff.y) * Math.min(1, 0.05 * k)
  camZ += (camZT - camZ) * Math.min(1, 0.08 * k)
  camera.position.set(camOff.x, camOff.y, camZ)
  camera.lookAt(0, 0, 0)
  // 记录核心的屏幕位置（点它进星途）
  camera.updateMatrixWorld()
  v3.set(0, 0, 0).project(camera)
  coreSX = (v3.x * 0.5 + 0.5) * W
  coreSY = (-v3.y * 0.5 + 0.5) * H

  if (tiltGroup) {
    tiltGroup.rotation.x = TILT_X
    tiltGroup.rotation.z = TILT_Z
  }
  if (spinGroup) {
    spinGroup.rotation.y = rotY
    spinGroup.scale.setScalar(zoomK)
  }

  const breath = 0.5 + 0.5 * Math.sin(now / 2400)
  if (coreSpikes) {
    const s = coreR * 1.46 * zoomK
    coreSpikes.scale.set(s, s * 1.12, 1)
    coreSpikes.material.opacity = (0.56 + 0.06 * breath) * fieldT
  }
  if (coreGlow) {
    const s = coreR * 0.1 * zoomK
    coreGlow.scale.set(s, s * (0.55 + Math.sin(TILT_X) * 0.6), 1)
    coreGlow.material.opacity = (0.05 + 0.012 * breath) * fieldT
  }
  if (coreHalo) {
    const s = coreR * 0.62 * zoomK
    coreHalo.scale.set(s, s * Math.sin(TILT_X) * 0.98, 1)
    coreHalo.material.opacity = (0.058 + 0.01 * Math.sin(now / 3300)) * fieldT
  }
  if (frameGlow) {
    frameGlow.material.opacity = 0.04 * fieldT
  }

  if (stars) {
    stars.rotation.y += 0.0001 * k
    for (const p of stars.children) p.material.uniforms.uOpacity.value = 0.7
  }
  for (const p of dustLayers) {
    if (p.isMesh) p.material.uniforms.uOpacity.value = fieldT
    else {
      p.material.uniforms.uOpacity.value = 1
      p.material.uniforms.uIntro.value = introT
    }
  }
  if (mat) {
    mat.uniforms.uTime.value = now / 1000
    mat.uniforms.uOpacity.value = 1
  }

  // 悬停提示挂在星上：星系继续转，标签跟着星一起走
  if (tooltip.value.show && hoverIdx >= 0 && points && introT > 0.85) {
    v3.set(curPos[hoverIdx * 3], curPos[hoverIdx * 3 + 1], curPos[hoverIdx * 3 + 2])
    v3.applyMatrix4(points.matrixWorld)
    v3.project(camera)
    tooltip.value = {
      ...tooltip.value,
      x: (v3.x * 0.5 + 0.5) * W,
      y: (-v3.y * 0.5 + 0.5) * H,
    }
  }

  renderer.render(scene, camera)
}
</script>

<template>
  <div ref="wrapRef" class="stage">
    <div
      v-if="tooltip.show"
      class="tip"
      :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
    >
      <div class="d">{{ tooltip.date }}</div>
      <div class="t">{{ tooltip.title }}</div>
    </div>
  </div>
</template>

<style scoped>
.stage {
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.stage :deep(canvas) {
  display: block;
  cursor: grab;
}

.tip {
  position: absolute;
  transform: translate(-50%, calc(-100% - 16px));
  pointer-events: none;
  padding: 7px 13px;
  border-radius: 10px;
  background: rgba(14, 10, 16, 0.84);
  border: 1px solid rgba(255, 176, 120, 0.32);
  box-shadow: 0 6px 26px rgba(0, 0, 0, 0.55), 0 0 18px rgba(255, 168, 96, 0.14);
  backdrop-filter: blur(8px);
  white-space: nowrap;
  transition: left 0.06s linear, top 0.06s linear;
  animation: tipin 0.18s ease;
}

.tip::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: -4px;
  width: 7px;
  height: 7px;
  margin-left: -3.5px;
  background: rgba(14, 10, 16, 0.84);
  border-right: 1px solid rgba(255, 176, 120, 0.32);
  border-bottom: 1px solid rgba(255, 176, 120, 0.32);
  transform: rotate(45deg);
}

.tip .d {
  font-size: 12px;
  letter-spacing: 0.12em;
  color: #ffc48a;
}

.tip .t {
  font-size: 11px;
  color: rgba(233, 238, 247, 0.6);
  margin-top: 2px;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

@keyframes tipin {
  from {
    opacity: 0;
    transform: translate(-50%, calc(-100% - 10px));
  }
}
</style>
