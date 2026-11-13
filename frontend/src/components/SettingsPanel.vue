<script setup>
import { ref, computed } from 'vue'
import { api, today } from '../api'
import DatePicker from './DatePicker.vue'

const props = defineProps({
  settings: { type: Object, default: () => ({}) },
})
const emit = defineEmits(['close', 'changed', 'refresh'])

const tab = ref(new URLSearchParams(location.search).get('tab') === 'storage' ? 'storage' : 'general')
const busy = ref(false)
const msg = ref('')
const loveStart = ref(props.settings.loveStart || today())
const saving = ref(false)

const spanDays = computed(() => {
  const t = new Date(loveStart.value + 'T00:00:00')
  if (Number.isNaN(t.getTime())) return 0
  const now = new Date()
  const d0 = new Date(t.getFullYear(), t.getMonth(), t.getDate())
  const d1 = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const n = Math.round((d1 - d0) / 86400000) + 1
  return n > 0 ? n : 1
})

async function saveLoveStart() {
  if (saving.value) return
  saving.value = true
  msg.value = ''
  try {
    await api.setLoveStart(loveStart.value)
    emit('refresh')
  } catch (e) {
    msg.value = '保存失败：' + (e?.message || e)
  } finally {
    saving.value = false
  }
}

async function openDir() {
  try {
    await api.openDataDir()
  } catch (e) {
    msg.value = '打开失败：' + (e?.message || e)
  }
}

async function changeDir() {
  if (busy.value) return
  busy.value = true
  msg.value = ''
  try {
    const dir = await api.chooseDataDir()
    if (dir && dir !== props.settings.dataDir) {
      emit('changed', dir)
    }
  } catch (e) {
    msg.value = '更换失败：' + (e?.message || e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="mask" @click.self="$emit('close')">
    <div class="panel">
      <aside class="side">
        <div class="ttl">设置</div>
        <button class="nav" :class="{ on: tab === 'general' }" @click="tab = 'general'">常规</button>
        <button class="nav" :class="{ on: tab === 'storage' }" @click="tab = 'storage'">存储</button>
      </aside>

      <section class="main">
        <button class="x" @click="$emit('close')">
          <svg width="13" height="13" viewBox="0 0 14 14">
            <path d="M3 3l8 8M11 3l-8 8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
          </svg>
        </button>

        <div v-if="tab === 'general'" class="page">
          <h2>常规</h2>

          <div class="stats">
            <div class="stat">
              <b>{{ spanDays }}</b>
              <span>天相伴</span>
            </div>
            <div class="stat">
              <b>{{ settings.eventCount ?? 0 }}</b>
              <span>条瞬间</span>
            </div>
            <div class="stat">
              <b>{{ settings.imageCount ?? 0 }}</b>
              <span>张照片</span>
            </div>
          </div>

          <div class="field">
            <label class="label" for="love">相爱时间</label>
            <DatePicker id="love" v-model="loveStart" variant="field" @change="saveLoveStart" />
            <p class="note">从这一天开始算「相伴」的天数，默认是今天，以后随时可以改。</p>
          </div>

          <p class="err" v-if="msg">{{ msg }}</p>
        </div>

        <div v-else class="page">
          <h2>存储</h2>

          <div class="field">
            <div class="label">数据目录</div>
            <div class="path" :title="settings.dataDir">{{ settings.dataDir }}</div>
            <div class="acts">
              <button class="btn" @click="openDir">打开目录</button>
              <button class="btn" :disabled="busy" @click="changeDir">
                {{ busy ? '搬迁中…' : '更换目录' }}
              </button>
            </div>
            <p class="note">
              数据就是一份 SQLite 数据库 + images 图片文件夹。换目录时旧数据会整包复制过去；想备份或换电脑，把整个文件夹拷走即可。
            </p>
          </div>

          <p class="err" v-if="msg">{{ msg }}</p>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.mask {
  position: absolute;
  inset: 0;
  z-index: 90;
  background: rgba(2, 3, 8, 0.66);
  backdrop-filter: blur(6px);
  display: grid;
  place-items: center;
  animation: fade 0.25s ease;
}

@keyframes fade {
  from {
    opacity: 0;
  }
}

.panel {
  width: 620px;
  max-width: calc(100vw - 40px);
  height: 400px;
  max-height: calc(100vh - 40px);
  display: flex;
  border-radius: 16px;
  overflow: hidden;
  background: linear-gradient(160deg, rgba(20, 24, 40, 0.98), rgba(9, 11, 20, 0.99));
  border: 1px solid rgba(255, 255, 255, 0.09);
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.7), 0 0 40px rgba(255, 158, 196, 0.05);
  animation: pop 0.28s cubic-bezier(0.2, 0.8, 0.3, 1);
}

@keyframes pop {
  from {
    opacity: 0;
    transform: scale(0.97) translateY(8px);
  }
}

.side {
  flex: none;
  width: 132px;
  padding: 22px 12px;
  background: rgba(255, 255, 255, 0.025);
  border-right: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.side .ttl {
  font-size: 11px;
  letter-spacing: 0.22em;
  color: rgba(233, 238, 247, 0.3);
  padding: 0 10px 12px;
}

.nav {
  text-align: left;
  padding: 9px 10px;
  border-radius: 8px;
  font-size: 13px;
  letter-spacing: 0.08em;
  color: rgba(233, 238, 247, 0.5);
  transition: all 0.2s;
}

.nav:hover {
  color: rgba(233, 238, 247, 0.85);
  background: rgba(255, 255, 255, 0.04);
}

.nav.on {
  color: var(--pink);
  background: rgba(255, 158, 196, 0.1);
}

.main {
  position: relative;
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 26px 28px;
}

.x {
  position: absolute;
  top: 18px;
  right: 18px;
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: rgba(233, 238, 247, 0.45);
}

.x:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.07);
}

h2 {
  margin: 0 0 20px;
  font-size: 14px;
  font-weight: 500;
  letter-spacing: 0.2em;
  color: rgba(233, 238, 247, 0.85);
}

.stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 22px;
}

.stat {
  text-align: center;
  padding: 12px 4px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.stat b {
  display: block;
  font-size: 19px;
  font-weight: 500;
  color: var(--pink);
}

.stat span {
  font-size: 10.5px;
  letter-spacing: 0.12em;
  color: rgba(233, 238, 247, 0.34);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.label {
  font-size: 11px;
  letter-spacing: 0.2em;
  color: rgba(233, 238, 247, 0.34);
}

.path {
  font-size: 12px;
  color: rgba(233, 238, 247, 0.72);
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 9px;
  padding: 10px 12px;
  word-break: break-all;
  user-select: text;
  font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
  line-height: 1.5;
}

.acts {
  display: flex;
  gap: 8px;
}

.acts .btn {
  flex: 1;
  padding: 8px 0;
  font-size: 12px;
}

.acts .btn:disabled {
  opacity: 0.5;
}

.note {
  margin: 2px 0 0;
  font-size: 11.5px;
  line-height: 1.75;
  color: rgba(233, 238, 247, 0.28);
}

.err {
  margin: 12px 0 0;
  font-size: 11.5px;
  color: #ff8296;
}
</style>
