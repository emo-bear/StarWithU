<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { api, today } from '../api'
import CoverFlow from './CoverFlow.vue'
import DatePicker from './DatePicker.vue'

const props = defineProps({
  event: { type: Object, default: null },
})
const emit = defineEmits(['close', 'saved'])
// 截图调试时跳过入场动画
const noAnim = location.search.includes('nointro')

const isEdit = computed(() => !!(props.event && props.event.id))
const form = reactive({
  id: props.event?.id || 0,
  title: props.event?.title || '',
  date: props.event?.date || today(),
  description: props.event?.description || '',
  images: [...(props.event?.images || [])],
})

const previews = ref({}) // name -> dataUrl
const busy = ref(false)
const picking = ref(false)
const error = ref('')

// 轮播当前张 + 舞台高度（按窗口高度给一档合适的比例）
// ?ci=n 仅用于截图调试时指定初始张
const active = ref(Number(new URLSearchParams(location.search).get('ci') || 0))
const flowH = ref(Math.round(Math.min(226, Math.max(150, window.innerHeight * 0.3))))

const flowItems = computed(() =>
  form.images.map((name) => ({ key: name, src: previews.value[name] || '' })),
)

onMounted(async () => {
  for (const name of form.images) loadPreview(name)
  window.addEventListener('keydown', onKey)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

// 图片数量变化后把当前张拉回合法范围
watch(
  () => form.images.length,
  (len) => {
    if (!len) active.value = 0
    else if (active.value > len - 1) active.value = len - 1
  },
)

// Esc 关闭，Ctrl/⌘ + Enter 保存
function onKey(e) {
  if (e.key === 'Escape') {
    emit('close')
  } else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    save()
  }
}

async function loadPreview(name) {
  if (previews.value[name]) return
  try {
    const url = await api.imageData(name)
    previews.value = { ...previews.value, [name]: url }
  } catch {
    /* 忽略 */
  }
}

async function pick() {
  if (picking.value) return
  picking.value = true
  error.value = ''
  try {
    const names = await api.pickImages()
    const first = form.images.length
    for (const name of names || []) {
      if (!form.images.includes(name)) form.images.push(name)
      loadPreview(name)
    }
    if (form.images.length > first) active.value = first
  } catch (e) {
    error.value = '选择图片失败：' + (e?.message || e)
  } finally {
    picking.value = false
  }
}

function removeImage(name) {
  const i = form.images.indexOf(name)
  if (i < 0) return
  form.images = form.images.filter((x) => x !== name)
  if (i < active.value) active.value -= 1
  else if (i === active.value) active.value = Math.min(active.value, form.images.length - 1)
}

async function save() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const saved = await api.save({
      id: form.id,
      title: form.title,
      date: form.date,
      description: form.description,
      images: form.images,
      x: props.event?.x || 0,
      y: props.event?.y || 0,
      z: props.event?.z || 0,
    })
    emit('saved', saved)
  } catch (e) {
    error.value = '保存失败：' + (e?.message || e)
    busy.value = false
  }
}
</script>

<template>
  <div class="editor" :class="{ noanim: noAnim }">
    <div class="bar">
      <div class="ttl">{{ isEdit ? '编辑这一天' : '写下这一天' }}</div>
      <button class="close" @click="$emit('close')">
        <svg width="14" height="14" viewBox="0 0 14 14"><path d="M3 3l8 8M11 3l-8 8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" /></svg>
      </button>
    </div>

    <div class="body">
      <!-- 照片：3D coverflow，中间正面、两侧内旋，左右拖动/滚轮切换 -->
      <div class="flow">
        <CoverFlow
          :items="flowItems"
          :index="active"
          :height="flowH"
          @update:index="active = $event"
          @remove="removeImage"
          @add="pick"
        />
      </div>

      <!-- 表单：与图片同一中轴，正文也在图片下方居中 -->
      <div class="form">
        <input class="title" v-model="form.title" maxlength="40" placeholder="给这一天起个名字" />

        <div class="daterow">
          <DatePicker v-model="form.date" />
        </div>

        <textarea
          class="desc"
          v-model="form.description"
          rows="6"
          placeholder="那天发生了什么？（支持 Markdown 排版）"
        ></textarea>
        <div class="mdhint">支持 Markdown：# 标题 · **粗体** · - 列表 · &gt; 引用 · `代码`</div>
      </div>

      <div class="err" v-if="error">{{ error }}</div>

      <div class="footer">
        <button class="btn" @click="$emit('close')">取消</button>
        <button class="btn primary" :disabled="busy" @click="save">
          {{ busy ? '保存中…' : '保存' }}
        </button>
      </div>
      <div class="khint">Esc 取消 · Ctrl + Enter 保存 · 左右拖动切换照片</div>
    </div>
  </div>
</template>

<style scoped>
.editor.noanim {
  animation: none !important;
}

.editor {
  position: absolute;
  inset: 0;
  z-index: 70;
  display: flex;
  flex-direction: column;
  background:
    radial-gradient(ellipse 60% 40% at 50% 0%, rgba(255, 158, 196, 0.055), transparent 70%),
    #05060d;
  animation: pagein 0.35s cubic-bezier(0.2, 0.7, 0.3, 1);
}

@keyframes pagein {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
}

.bar {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 40px 26px 0;
  height: 56px;
  box-sizing: content-box;
}

.ttl {
  font-size: 14px;
  letter-spacing: 0.14em;
  color: rgba(233, 238, 247, 0.7);
}

.close {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: rgba(233, 238, 247, 0.5);
  border: 1px solid var(--line);
  transition: all 0.2s;
}

.close:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.06);
}

.body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 6px 30px 24px;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 158, 196, 0.22) transparent;
}

.body::-webkit-scrollbar {
  width: 6px;
}

.body::-webkit-scrollbar-thumb {
  border-radius: 3px;
  background: rgba(255, 158, 196, 0.18);
}

.flow {
  width: 100%;
}

.form {
  max-width: 540px;
  margin: 4px auto 0;
  padding: 0 30px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.title {
  width: 100%;
  text-align: center;
  font-size: 19px;
  font-weight: 500;
  letter-spacing: 0.05em;
  padding: 8px 0;
  background: transparent;
  border: none;
  border-bottom: 1px solid transparent;
  border-radius: 0;
}

.title:focus {
  background: transparent;
  border-bottom-color: rgba(255, 158, 196, 0.28);
}

.title::placeholder {
  color: rgba(233, 238, 247, 0.2);
}

.daterow {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 1px;
}

.desc {
  width: 100%;
  margin-top: 10px;
  min-height: 128px;
  text-align: center;
  padding: 14px 18px;
  font-size: 14px;
  line-height: 1.95;
  letter-spacing: 0.02em;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.018);
  border: 1px solid rgba(255, 255, 255, 0.055);
}

.desc:focus {
  background: rgba(255, 255, 255, 0.032);
  border-color: rgba(255, 158, 196, 0.32);
}

.mdhint {
  font-size: 10.5px;
  letter-spacing: 0.04em;
  text-align: center;
  color: rgba(233, 238, 247, 0.2);
}

.err {
  max-width: 540px;
  margin: 14px auto 0;
  font-size: 12px;
  color: #ff8296;
  text-align: center;
}

.footer {
  max-width: 540px;
  margin: 14px auto 0;
  display: flex;
  justify-content: center;
  gap: 12px;
}

.footer .btn {
  min-width: 110px;
}

.footer .btn:disabled {
  opacity: 0.55;
}

.khint {
  max-width: 540px;
  margin: 10px auto 0;
  text-align: center;
  font-size: 10.5px;
  letter-spacing: 0.14em;
  color: rgba(233, 238, 247, 0.18);
}
</style>
