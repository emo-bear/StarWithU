<script setup>
import { ref, onMounted, computed } from 'vue'
import TitleBar from './components/TitleBar.vue'
import Splash from './components/Splash.vue'
import Universe from './components/Universe.vue'
import StarTrail from './components/StarTrail.vue'
import DetailView from './components/DetailView.vue'
import EditorView from './components/EditorView.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import { api, waitForBridge } from './api'
import { mockEvents, mockSettings } from './mock'

const events = ref([])
const settings = ref({})
// ?nointro 用于调试时跳过开屏
const skipIntro = location.search.includes('nointro')
const showSplash = ref(!skipIntro)
const live = ref(skipIntro)
const hudOn = ref(skipIntro)

const view = ref('universe')
// 主页两种形态：星云（星系） / 星途（星尾尘）
const mode = ref(new URLSearchParams(location.search).get('mode') === 'trail' ? 'trail' : 'nebula')
const current = ref(null)
const editorOpen = ref(false)
const editorSource = ref(null)
const settingsOpen = ref(false)
const error = ref('')

const spanText = computed(() => {
  const s = settings.value
  if (!s || !s.spanDays) return '从今天开始'
  return `相伴 ${s.spanDays} 天 · ${s.eventCount || 0} 个瞬间`
})

onMounted(async () => {
  // 开屏开始散开时就让背后的星系开始聚拢，衔接更顺
  if (!skipIntro) setTimeout(() => (live.value = true), 2400)
  // ?demo 直接用演示数据（浏览器里调样式时不必等后端超时）
  if (location.search.includes('demo')) {
    const list = mockEvents()
    events.value = list
    settings.value = mockSettings(list)
    applyDebugView()
    return
  }
  try {
    await waitForBridge(2500)
    await reload()
  } catch (e) {
    // 浏览器里调样式时退化成演示数据
    const list = mockEvents()
    events.value = list
    settings.value = mockSettings(list)
    applyDebugView()
  }
})

async function reload() {
  const [list, st] = await Promise.all([api.list(), api.settings()])
  events.value = list || []
  settings.value = st || {}
  applyDebugView()
}

// 调试用：?view=detail / ?view=editor 直接打开对应页面
function applyDebugView() {
  const v = new URLSearchParams(location.search).get('view')
  if (v === 'detail' && events.value.length) {
    current.value = events.value[0]
    view.value = 'detail'
  } else if (v === 'editor') {
    // ?view=editor&edit=1 用第 1 条事件打开编辑态（带图片），方便调样式
    const q = new URLSearchParams(location.search)
    const idx = Number(q.get('edit') || 0)
    if (idx > 0 && events.value[idx - 1]) {
      editorSource.value = events.value[idx - 1]
    }
    editorOpen.value = true
  } else if (v === 'settings') {
    settingsOpen.value = true
  }
}

function onSplashDone() {
  showSplash.value = false
  live.value = true
  // 星系已经聚拢了一会儿，HUD 稍等就淡入
  setTimeout(() => (hudOn.value = true), 900)
}

function openEvent(e) {
  current.value = e
  view.value = 'detail'
}

function backToUniverse() {
  view.value = 'universe'
}

function newEvent() {
  editorSource.value = null
  editorOpen.value = true
}

function editEvent(e) {
  editorSource.value = e
  editorOpen.value = true
}

async function onSaved(ev) {
  const wasEdit = !!editorSource.value
  editorOpen.value = false
  editorSource.value = null
  await reload()
  if (wasEdit && view.value === 'detail') {
    current.value = events.value.find((x) => x.id === ev.id) || ev
  }
}

async function onDelete(ev) {
  await api.remove(ev.id)
  await reload()
  current.value = null
  view.value = 'universe'
}

async function onDirChanged() {
  await reload()
  settingsOpen.value = false
}

// 只刷新数据，不关闭设置面板（改相爱时间时用）
async function onSettingsRefresh() {
  try {
    settings.value = (await api.settings()) || {}
  } catch {
    /* 忽略 */
  }
}
</script>

<template>
  <div class="app">
    <TitleBar />

    <div class="stage-layer" :class="{ off: mode === 'trail' }">
      <Universe
        :events="events"
        :active="live"
        :paused="mode === 'trail'"
        @open="openEvent"
        @core="mode = 'trail'"
      />
    </div>

    <Transition name="vfade">
      <StarTrail
        v-if="mode === 'trail'"
        :events="events"
        :active="view === 'universe'"
        @open="openEvent"
      />
    </Transition>

    <div class="hud" :class="{ on: hudOn }" v-show="view === 'universe'">
      <div class="tabs">
        <button :class="{ on: mode === 'nebula' }" @click="mode = 'nebula'">星云</button>
        <button :class="{ on: mode === 'trail' }" @click="mode = 'trail'">星途</button>
      </div>

      <div class="meta">
        <div class="name">Star With U</div>
        <div class="sub">{{ spanText }}</div>
      </div>

      <div class="tools">
        <button class="tool" title="记录新的一天" @click="newEvent">
          <svg width="16" height="16" viewBox="0 0 20 20">
            <path d="M10 4v12M4 10h12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
          </svg>
        </button>
        <button class="tool" title="设置" @click="settingsOpen = true">
          <svg width="17" height="17" viewBox="0 0 20 20">
            <path
              d="M8.6 2.2h2.8l.4 1.9c.4.14.78.34 1.13.58l1.77-.82 2 2-.82 1.77c.24.35.44.73.58 1.13l1.9.4v2.8l-1.9.4c-.14.4-.34.78-.58 1.13l.82 1.77-2 2-1.77-.82c-.35.24-.73.44-1.13.58l-.4 1.9H8.6l-.4-1.9a6.6 6.6 0 01-1.13-.58l-1.77.82-2-2 .82-1.77a6.6 6.6 0 01-.58-1.13l-1.9-.4v-2.8l1.9-.4c.14-.4.34-.78.58-1.13L3.3 5.86l2-2 1.77.82c.35-.24.73-.44 1.13-.58l.4-1.9z"
              fill="none"
              stroke="currentColor"
              stroke-width="1.2"
            />
            <circle cx="10" cy="10" r="2.4" fill="none" stroke="currentColor" stroke-width="1.2" />
          </svg>
        </button>
      </div>

      <div class="empty" v-if="!events.length">
        <div class="et">这里还没有星星</div>
        <div class="es">点右上角的 +，写下第一个瞬间</div>
      </div>

      <div class="err" v-if="error">{{ error }}</div>
    </div>

    <DetailView
      v-if="view === 'detail' && current"
      :event="current"
      @back="backToUniverse"
      @edit="editEvent"
      @delete="onDelete"
    />

    <EditorView v-if="editorOpen" :event="editorSource" @close="editorOpen = false" @saved="onSaved" />

    <SettingsPanel
      v-if="settingsOpen"
      :settings="settings"
      @close="settingsOpen = false"
      @changed="onDirChanged"
      @refresh="onSettingsRefresh"
    />

    <Transition name="fade">
      <Splash v-if="showSplash" @done="onSplashDone" />
    </Transition>
  </div>
</template>

<style scoped>
.app {
  position: fixed;
  inset: 0;
  background:
    radial-gradient(ellipse 70% 50% at 22% 18%, rgba(90, 74, 138, 0.16), transparent 65%),
    radial-gradient(ellipse 60% 45% at 82% 78%, rgba(40, 96, 120, 0.14), transparent 65%),
    radial-gradient(ellipse at 50% 50%, #0a0d1c 0%, #05060d 60%, #020308 100%);
  overflow: hidden;
}

.hud {
  position: absolute;
  inset: 0;
  z-index: 50;
  pointer-events: none;
  opacity: 0;
  transition: opacity 1.2s ease 0.2s;
}

.hud.on {
  opacity: 1;
}

.hud > * {
  pointer-events: auto;
}

/* 居中的两个主视图切换：星云 / 星途 */
.tabs {
  position: absolute;
  top: 50px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 38px;
}

/* 只有文字，没有底；选中说明靠下面对一根淡粉横杠 */
.tabs button {
  position: relative;
  padding: 3px 1px 9px;
  font-size: 11.5px;
  letter-spacing: 0.22em;
  text-indent: 0.22em;
  color: rgba(233, 238, 247, 0.42);
  transition: color 0.3s ease, text-shadow 0.3s ease;
}

.tabs button::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 0;
  width: 0;
  height: 1.5px;
  border-radius: 2px;
  transform: translateX(-50%);
  opacity: 0;
  background: linear-gradient(
    90deg,
    rgba(255, 158, 196, 0) 0%,
    rgba(255, 158, 196, 0.9) 28%,
    rgba(255, 214, 232, 1) 50%,
    rgba(255, 158, 196, 0.9) 72%,
    rgba(255, 158, 196, 0) 100%
  );
  box-shadow: 0 0 9px rgba(255, 158, 196, 0.65);
  transition: width 0.34s cubic-bezier(0.3, 0.8, 0.3, 1), opacity 0.34s ease;
}

.tabs button:hover {
  color: rgba(255, 255, 255, 0.82);
}

.tabs button.on {
  color: #fff;
  text-shadow: 0 0 16px rgba(255, 158, 196, 0.42);
}

.tabs button.on::after {
  width: 108%;
  opacity: 1;
}

/* 星系视图切换时整体退场，别硬切 */
.stage-layer {
  position: absolute;
  inset: 0;
  transition: opacity 0.5s ease, transform 0.7s cubic-bezier(0.2, 0.7, 0.3, 1);
}

.stage-layer.off {
  opacity: 0;
  transform: scale(1.07);
  pointer-events: none;
}

.vfade-enter-active {
  transition: opacity 0.6s ease 0.12s;
}

.vfade-leave-active {
  transition: opacity 0.3s ease;
}

.vfade-enter-from,
.vfade-leave-to {
  opacity: 0;
}

.meta {
  position: absolute;
  top: 60px;
  left: 36px;
  pointer-events: none !important;
}

.meta .name {
  font-size: 13px;
  letter-spacing: 0.28em;
  color: rgba(233, 238, 247, 0.82);
  text-shadow: 0 1px 10px rgba(4, 5, 12, 0.9), 0 0 26px rgba(255, 158, 196, 0.3);
}

.meta .sub {
  margin-top: 7px;
  font-size: 11px;
  letter-spacing: 0.16em;
  color: rgba(233, 238, 247, 0.42);
  text-shadow: 0 1px 10px rgba(4, 5, 12, 0.9);
}

.tools {
  position: absolute;
  top: 58px;
  right: 32px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.tool {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: rgba(233, 238, 247, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.07);
  background: rgba(255, 255, 255, 0.02);
  transition: all 0.25s;
}

.tool:hover {
  color: var(--pink);
  border-color: rgba(255, 158, 196, 0.35);
  background: rgba(255, 158, 196, 0.06);
}

.tools .tool:first-child:hover svg {
  transform: rotate(90deg);
}

.tool svg {
  transition: transform 0.25s;
}

.empty {
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  transform: translateY(-50%);
  text-align: center;
  pointer-events: none !important;
}

.empty .et {
  font-size: 15px;
  letter-spacing: 0.24em;
  color: rgba(233, 238, 247, 0.5);
}

.empty .es {
  margin-top: 10px;
  font-size: 11.5px;
  letter-spacing: 0.12em;
  color: rgba(233, 238, 247, 0.24);
}

.err {
  position: absolute;
  bottom: 40px;
  left: 0;
  right: 0;
  text-align: center;
  font-size: 11.5px;
  color: #ff8296;
}
</style>
