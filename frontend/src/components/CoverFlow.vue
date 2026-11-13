<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'

// 3D coverflow 轮播：中间一张正对镜头，左右两侧沿 Y 轴内旋、后退、压暗，
// 一直排到容器外被裁掉，形成"环绕"纵深（参考 iTunes CoverFlow）
const props = defineProps({
  // [{ key, src }]
  items: { type: Array, default: () => [] },
  index: { type: Number, default: 0 },
  height: { type: Number, default: 200 },
})
const emit = defineEmits(['update:index', 'remove', 'add'])

const root = ref(null)
const width = ref(640)
let ro = null

onMounted(() => {
  if (!root.value) return
  const measure = () => {
    width.value = root.value?.clientWidth || 640
  }
  measure()
  if (window.ResizeObserver) {
    ro = new ResizeObserver(measure)
    ro.observe(root.value)
  } else {
    window.addEventListener('resize', measure)
  }
})

onBeforeUnmount(() => {
  if (ro) ro.disconnect()
})

const cardW = computed(() => Math.round(Math.min(Math.max(props.height * 1.95, 240), 460)))
const stageH = computed(() => props.height + 30)

// 沿一条虚拟圆弧摆位：每远离一张，横向步进更大、转得更狠、缩得更多、退得更深。
// 这些数字调出来的观感就是参考图那种"环绕一圈"的纵深感（x 单位 = 卡宽）
const ARC = [
  null,
  { x: 0.66, ry: 46, s: 0.86, z: -70, o: 0.94, dim: 0.28 },
  { x: 1.26, ry: 54, s: 0.68, z: -150, o: 0.8, dim: 0.44 },
  { x: 1.8, ry: 60, s: 0.54, z: -230, o: 0.56, dim: 0.56 },
  { x: 2.28, ry: 64, s: 0.43, z: -310, o: 0.3, dim: 0.64 },
]

const n = computed(() => props.items.length)

function go(i) {
  const next = Math.min(Math.max(i, 0), n.value - 1)
  if (next !== props.index) emit('update:index', next)
}

function slideStyle(i) {
  const off = i - props.index
  const a = Math.abs(off)
  if (a === 0) {
    return {
      '--dim': 0,
      opacity: 1,
      zIndex: 60,
      transform: 'translate(-50%, -50%) translateZ(0)',
      pointerEvents: 'auto',
    }
  }
  const c = ARC[a] || ARC[ARC.length - 1]
  const extra = Math.max(0, a - (ARC.length - 1))
  const dir = Math.sign(off)
  const dx = (c.x + extra * 0.6) * cardW.value * dir
  return {
    '--dim': c.dim,
    opacity: Math.max(0.18, c.o - extra * 0.3),
    zIndex: 60 - a,
    transform:
      `translate(-50%, -50%) translateX(${dx}px) translateZ(${c.z}px) ` +
      `rotateY(${-dir * c.ry}deg) scale(${Math.max(0.32, c.s - extra * 0.1)})`,
    pointerEvents: a > 4 ? 'none' : 'auto',
  }
}

// —— 拖拽 / 滚轮切换 ——
const drag = { on: false, x: 0, moved: false }

function onDown(e) {
  if (e.button) return
  drag.on = true
  drag.moved = false
  drag.x = e.clientX
}

function onMove(e) {
  if (!drag.on) return
  if (Math.abs(e.clientX - drag.x) > 8) drag.moved = true
}

function onUp(e) {
  if (!drag.on) return
  drag.on = false
  const dx = e.clientX - drag.x
  if (Math.abs(dx) > 38) go(props.index + (dx > 0 ? -1 : 1))
}

let lock = 0
function onWheel(e) {
  if (!n.value || n.value < 2) return
  const d = Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : 0
  if (!d) return
  e.preventDefault()
  const now = Date.now()
  if (now < lock) return
  lock = now + 260
  go(props.index + (d > 0 ? 1 : -1))
}

function tap(i) {
  if (drag.moved) return
  if (i === props.index) return
  go(i)
}

function removeAt(i) {
  emit('remove', props.items[i].key)
}
</script>

<template>
  <div ref="root" class="cf">
    <div
      class="stage"
      :style="{ height: stageH + 'px' }"
      @pointerdown="onDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
      @pointerleave="onUp"
      @wheel="onWheel"
    >
      <div class="track">
        <div
          v-for="(it, i) in items"
          :key="it.key"
          class="slide"
          :class="{ on: i === index }"
          :style="{ ...slideStyle(i), width: cardW + 'px', height: height + 'px' }"
          @click="tap(i)"
        >
          <img v-if="it.src" :src="it.src" alt="" draggable="false" />
          <div v-else class="ph"><span></span></div>
          <div class="veil"></div>
          <div class="glow"></div>
          <button v-if="i === index" class="x" title="移除这张" @click.stop="removeAt(i)">
            <svg width="9" height="9" viewBox="0 0 10 10">
              <path d="M2 2l6 6M8 2L2 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
            </svg>
          </button>
        </div>

        <button v-if="!items.length" class="empty" @click="$emit('add')">
          <span class="big">+</span>
          <span class="txt">添加照片</span>
          <span class="sub">支持多选，左右拖动切换</span>
        </button>
      </div>
    </div>

    <div class="ctrl">
      <div class="dots" v-if="items.length > 1">
        <i v-for="(_, i) in items" :key="i" :class="{ on: i === index }" @click="go(i)"></i>
      </div>
      <button class="addpill" v-if="items.length" @click="$emit('add')">
        <span class="p">+</span> 添加照片
      </button>
    </div>
  </div>
</template>

<style scoped>
.cf {
  width: 100%;
  user-select: none;
}

.stage {
  position: relative;
  width: 100%;
  overflow: hidden;
  perspective: 950px;
  perspective-origin: 50% 46%;
  touch-action: pan-y;
  cursor: grab;
}

.stage:active {
  cursor: grabbing;
}

/* 两侧极轻的边缘压暗：卡片是直接被窗口边缘裁掉的，这里只做一点收口 */
.stage::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 90;
  background: linear-gradient(
    90deg,
    rgba(5, 6, 13, 0.55) 0%,
    rgba(5, 6, 13, 0) 7%,
    rgba(5, 6, 13, 0) 93%,
    rgba(5, 6, 13, 0.55) 100%
  );
}

.track {
  position: absolute;
  inset: 0;
  transform-style: preserve-3d;
}

.slide {
  position: absolute;
  left: 50%;
  top: 50%;
  border-radius: 14px;
  overflow: hidden;
  transform-style: preserve-3d;
  backface-visibility: hidden;
  transition: transform 0.52s cubic-bezier(0.22, 0.72, 0.22, 1), opacity 0.42s ease,
    box-shadow 0.4s ease;
  box-shadow: 0 22px 54px rgba(0, 0, 0, 0.62), 0 0 0 1px rgba(255, 255, 255, 0.07);
  background: #0a0c16;
}

.slide:not(.on) {
  cursor: pointer;
}

.slide.on {
  box-shadow: 0 26px 64px rgba(0, 0, 0, 0.7), 0 0 0 1px rgba(255, 255, 255, 0.13),
    0 0 46px rgba(255, 158, 196, 0.14);
}

.slide img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.ph {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  background: linear-gradient(120deg, rgba(255, 255, 255, 0.05), rgba(255, 255, 255, 0.015));
}

.ph span {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1.4px solid rgba(255, 158, 196, 0.22);
  border-top-color: rgba(255, 158, 196, 0.8);
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* 侧边卡片的压暗遮罩 */
.veil {
  position: absolute;
  inset: 0;
  background: rgba(5, 6, 13, var(--dim, 0));
  pointer-events: none;
  transition: background 0.42s ease;
}

/* 中间那张在下方透一层粉色反光，和星系配色呼应 */
.glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(
    ellipse 70% 40% at 50% 108%,
    rgba(255, 158, 196, 0.16),
    transparent 70%
  );
  opacity: 0;
  transition: opacity 0.42s ease;
}

.slide.on .glow {
  opacity: 1;
}

.slide .x {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: rgba(8, 10, 18, 0.72);
  color: rgba(255, 255, 255, 0.8);
  opacity: 0;
  transition: all 0.2s;
}

.slide.on:hover .x {
  opacity: 1;
}

.slide .x:hover {
  background: rgba(255, 110, 130, 0.85);
  color: #fff;
}

.empty {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: min(420px, 62%);
  height: calc(100% - 28px);
  border-radius: 14px;
  border: 1px dashed rgba(255, 255, 255, 0.16);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  color: rgba(233, 238, 247, 0.42);
  transition: all 0.24s;
}

.empty .big {
  font-size: 22px;
  font-weight: 200;
  line-height: 1;
}

.empty .txt {
  font-size: 12px;
  letter-spacing: 0.12em;
}

.empty .sub {
  font-size: 10px;
  letter-spacing: 0.06em;
  color: rgba(233, 238, 247, 0.22);
}

.empty:hover {
  border-color: rgba(255, 158, 196, 0.45);
  color: var(--pink);
  background: rgba(255, 158, 196, 0.035);
}

.ctrl {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  height: 26px;
}

.dots {
  display: flex;
  align-items: center;
  gap: 7px;
}

.dots i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.24);
  cursor: pointer;
  transition: all 0.25s;
}

.dots i:hover {
  background: rgba(255, 255, 255, 0.5);
}

.dots i.on {
  background: var(--pink);
  box-shadow: 0 0 8px rgba(255, 158, 196, 0.8);
  width: 14px;
  border-radius: 3px;
}

.addpill {
  font-size: 11px;
  letter-spacing: 0.1em;
  padding: 5px 13px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: rgba(233, 238, 247, 0.45);
  transition: all 0.22s;
  white-space: nowrap;
}

.addpill .p {
  color: rgba(255, 158, 196, 0.85);
  margin-right: 2px;
}

.addpill:hover {
  color: #fff;
  border-color: rgba(255, 158, 196, 0.4);
  background: rgba(255, 158, 196, 0.06);
}
</style>
