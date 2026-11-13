<script setup>
import { ref, watch, onMounted, onBeforeUnmount, computed } from 'vue'
import { api, prettyDate } from '../api'
import { renderMarkdown } from '../markdown'

const props = defineProps({
  event: { type: Object, required: true },
})
const emit = defineEmits(['back', 'edit', 'delete'])
// 截图调试时跳过入场动画
const noAnim = location.search.includes('nointro')

const images = ref([])
const loading = ref(true)
const idx = ref(0)
const asking = ref(false)
const paused = ref(false)

const hasImages = computed(() => images.value.length > 0)
const multi = computed(() => images.value.length > 1)
const bodyHtml = computed(() => renderMarkdown(props.event.description || ''))

let timer = 0
const AUTO_MS = 3400

function stopAuto() {
  if (timer) clearInterval(timer)
  timer = 0
}

function startAuto() {
  stopAuto()
  if (!multi.value) return
  timer = setInterval(() => {
    if (paused.value || document.hidden) return
    idx.value = (idx.value + 1) % images.value.length
  }, AUTO_MS)
}

async function load() {
  loading.value = true
  idx.value = 0
  const names = props.event.images || []
  const out = []
  for (const name of names) {
    try {
      const url = await api.imageData(name)
      if (url) out.push(url)
    } catch {
      /* 图片丢失就跳过 */
    }
  }
  images.value = out
  loading.value = false
  startAuto()
}

watch(() => props.event.id, load)
watch(multi, startAuto)

onMounted(() => {
  load()
  window.addEventListener('keydown', onKey)
})

onBeforeUnmount(() => {
  stopAuto()
  window.removeEventListener('keydown', onKey)
})

function onKey(e) {
  if (e.key === 'Escape') emit('back')
  if (!multi.value) return
  if (e.key === 'ArrowLeft') prev()
  if (e.key === 'ArrowRight') next()
}

function prev() {
  if (!multi.value) return
  idx.value = (idx.value - 1 + images.value.length) % images.value.length
  startAuto()
}

function next() {
  if (!multi.value) return
  idx.value = (idx.value + 1) % images.value.length
  startAuto()
}

function askDelete() {
  if (!asking.value) {
    asking.value = true
    setTimeout(() => (asking.value = false), 3500)
    return
  }
  emit('delete', props.event)
}
</script>

<template>
  <div class="detail" :class="{ noanim: noAnim }">
    <div class="bar">
      <button class="back" @click="$emit('back')">
        <svg width="16" height="16" viewBox="0 0 16 16">
          <path d="M10 3L5 8l5 5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span>返回</span>
      </button>
      <div class="acts">
        <button @click="$emit('edit', event)">编辑</button>
        <button class="del" :class="{ arming: asking }" @click="askDelete">
          {{ asking ? '确认删除' : '删除' }}
        </button>
      </div>
    </div>

    <!-- 标题与日期：固定在导航栏下方，不随正文滚动 -->
    <header class="head">
      <h1>{{ event.title }}</h1>
      <div class="date">{{ prettyDate(event.date) }}</div>
    </header>

    <!-- 轮播：固定区域，自动播放，底部小圆点定位 -->
    <div
      class="media"
      v-if="loading || hasImages"
      @mouseenter="paused = true"
      @mouseleave="paused = false"
    >
      <div class="frame">
        <div class="loading" v-if="loading"><span></span></div>
        <Transition v-else name="swap" mode="out-in">
          <img :key="idx" :src="images[idx]" alt="" draggable="false" />
        </Transition>
      </div>
      <div class="dots" v-if="multi">
        <i v-for="(_, i) in images" :key="i" :class="{ on: i === idx }" @click="idx = i"></i>
      </div>
    </div>

    <!-- 只有正文滚动 -->
    <div class="body">
      <div class="md" v-if="event.description" v-html="bodyHtml"></div>
      <div class="blank" v-else>这一天还没有写下什么。</div>
    </div>
  </div>
</template>

<style scoped>
.detail.noanim,
.detail.noanim .head,
.detail.noanim .body {
  animation: none !important;
}

.detail {
  position: absolute;
  inset: 0;
  z-index: 60;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background:
    radial-gradient(ellipse 60% 45% at 50% 0%, rgba(255, 158, 196, 0.07), transparent 70%),
    radial-gradient(ellipse 70% 50% at 80% 100%, rgba(127, 227, 242, 0.05), transparent 70%),
    #05060d;
  animation: pagein 0.4s cubic-bezier(0.2, 0.7, 0.3, 1);
}

@keyframes pagein {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
}

.bar {
  flex: none;
  height: 44px;
  padding: 40px 26px 0;
  box-sizing: content-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.back {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: rgba(233, 238, 247, 0.55);
  transition: color 0.2s;
}

.back:hover {
  color: #fff;
}

.acts {
  display: flex;
  gap: 6px;
}

.acts button {
  font-size: 12px;
  padding: 6px 15px;
  border-radius: 999px;
  border: 1px solid var(--line);
  color: rgba(233, 238, 247, 0.55);
  transition: all 0.2s;
}

.acts button:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.2);
}

.acts button.del:hover,
.acts button.del.arming {
  color: #ff8296;
  border-color: rgba(255, 110, 130, 0.5);
}

.head {
  flex: none;
  padding: 6px 30px 14px;
  text-align: center;
  animation: rise 0.5s cubic-bezier(0.2, 0.7, 0.3, 1);
}

h1 {
  margin: 0;
  font-size: 25px;
  font-weight: 500;
  letter-spacing: 0.06em;
  line-height: 1.4;
  color: #fff;
  text-shadow: 0 0 30px rgba(255, 158, 196, 0.25);
}

.date {
  margin-top: 7px;
  font-size: 11.5px;
  letter-spacing: 0.26em;
  color: rgba(255, 158, 196, 0.62);
}

.media {
  flex: none;
  width: 100%;
  max-width: 620px;
  margin: 0 auto;
  padding: 0 30px;
  box-sizing: border-box;
  position: relative;
}

.frame {
  position: relative;
  height: min(30vh, 268px);
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  box-shadow: 0 16px 46px rgba(0, 0, 0, 0.5);
}

.frame img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  display: block;
}

.loading {
  display: grid;
  place-items: center;
}

.loading span {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  border: 1.5px solid rgba(255, 158, 196, 0.2);
  border-top-color: rgba(255, 158, 196, 0.85);
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.dots {
  display: flex;
  justify-content: center;
  gap: 7px;
  margin-top: 12px;
  height: 8px;
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

.body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 16px 30px 72px;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 158, 196, 0.22) transparent;
  animation: rise 0.55s 0.05s backwards cubic-bezier(0.2, 0.7, 0.3, 1);
}

@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
}

.body::-webkit-scrollbar {
  width: 6px;
}

.body::-webkit-scrollbar-track {
  background: transparent;
}

.body::-webkit-scrollbar-thumb {
  border-radius: 3px;
  background: rgba(255, 158, 196, 0.18);
}

.body::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 158, 196, 0.35);
}

.md {
  max-width: 640px;
  margin: 0 auto;
  font-size: 14.5px;
  line-height: 2;
  color: rgba(233, 238, 247, 0.78);
  word-break: break-word;
  user-select: text;
}

.blank {
  max-width: 640px;
  margin: 0 auto;
  text-align: center;
  padding-top: 26px;
  font-size: 13px;
  letter-spacing: 0.1em;
  color: rgba(233, 238, 247, 0.28);
}

/* v-html 生成的内容不在 scoped 作用域里，需要 :deep */
.md :deep(h1),
.md :deep(h2),
.md :deep(h3),
.md :deep(h4),
.md :deep(h5),
.md :deep(h6) {
  margin: 26px 0 10px;
  font-weight: 500;
  line-height: 1.5;
  color: rgba(255, 255, 255, 0.94);
  letter-spacing: 0.04em;
}

.md :deep(h1) {
  font-size: 20px;
}
.md :deep(h2) {
  font-size: 17.5px;
}
.md :deep(h3) {
  font-size: 15.5px;
}
.md :deep(h4),
.md :deep(h5),
.md :deep(h6) {
  font-size: 14.5px;
}

.md :deep(h1:first-child),
.md :deep(h2:first-child),
.md :deep(h3:first-child),
.md :deep(p:first-child) {
  margin-top: 0;
}

.md :deep(p) {
  margin: 0 0 14px;
}

.md :deep(strong) {
  color: #fff;
  font-weight: 600;
}

.md :deep(em) {
  color: rgba(255, 158, 196, 0.92);
  font-style: italic;
}

.md :deep(del) {
  color: rgba(233, 238, 247, 0.4);
}

.md :deep(a) {
  color: var(--cyan, #7fe3f2);
  text-decoration: none;
  border-bottom: 1px solid rgba(127, 227, 242, 0.35);
}

.md :deep(a:hover) {
  border-bottom-color: rgba(127, 227, 242, 0.8);
}

.md :deep(ul),
.md :deep(ol) {
  margin: 0 0 14px;
  padding-left: 22px;
}

.md :deep(li) {
  margin: 4px 0;
}

.md :deep(li::marker) {
  color: rgba(255, 158, 196, 0.7);
}

.md :deep(blockquote) {
  margin: 0 0 14px;
  padding: 4px 16px;
  border-left: 2px solid rgba(255, 158, 196, 0.4);
  background: rgba(255, 158, 196, 0.045);
  border-radius: 0 8px 8px 0;
  color: rgba(233, 238, 247, 0.68);
}

.md :deep(blockquote p:last-child) {
  margin-bottom: 0;
}

.md :deep(code) {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 0.88em;
  padding: 1px 6px;
  border-radius: 5px;
  background: rgba(255, 255, 255, 0.07);
  color: rgba(255, 205, 220, 0.95);
}

.md :deep(pre) {
  margin: 0 0 14px;
  padding: 14px 16px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.045);
  border: 1px solid rgba(255, 255, 255, 0.06);
  overflow-x: auto;
}

.md :deep(pre code) {
  padding: 0;
  background: none;
  color: rgba(233, 238, 247, 0.8);
}

.md :deep(hr) {
  margin: 26px auto;
  width: 46px;
  border: none;
  border-top: 1px solid rgba(255, 255, 255, 0.14);
}

.swap-enter-active,
.swap-leave-active {
  transition: opacity 0.32s ease, transform 0.32s ease;
}

.swap-enter-from {
  opacity: 0;
  transform: scale(1.02);
}

.swap-leave-to {
  opacity: 0;
  transform: scale(0.99);
}
</style>
