<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'

// 自绘日期选择器：原生 <input type="date"> 的弹层是 Windows 系统样式，和主题完全不搭，
// 这里用 Teleport + fixed 定位做一个暗色玻璃面板，选中日、今天、周末都有独立配色
const props = defineProps({
  modelValue: { type: String, default: '' },
  // ghost：编辑器里那行居中的粉色小字；field：设置页里像输入框的那种
  variant: { type: String, default: 'ghost' },
  placeholder: { type: String, default: '选择日期' },
})
const emit = defineEmits(['update:modelValue', 'change'])

const WEEK = ['一', '二', '三', '四', '五', '六', '日']
const PANEL_W = 274
const PANEL_H = 322

const open = ref(false)
const anchor = ref(null)
const panel = ref(null)
const pos = ref({ left: 0, top: 0 })
const cur = ref({ y: 0, m: 0 })

const pad = (n) => String(n).padStart(2, '0')

function parse(s) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s || '')
  return m ? { y: +m[1], m: +m[2] - 1, d: +m[3] } : null
}
function ymd(y, m, d) {
  return `${y}-${pad(m + 1)}-${pad(d)}`
}

const picked = computed(() => parse(props.modelValue))
const label = computed(() => {
  const p = picked.value
  return p ? `${p.y} 年 ${p.m + 1} 月 ${p.d} 日` : props.placeholder
})

const todayStr = computed(() => {
  const t = new Date()
  return ymd(t.getFullYear(), t.getMonth(), t.getDate())
})

// 周一开头的 6×7 网格
const cells = computed(() => {
  const { y, m } = cur.value
  const first = new Date(y, m, 1)
  const lead = (first.getDay() + 6) % 7
  const out = []
  for (let i = 0; i < 42; i++) {
    const d = new Date(y, m, 1 - lead + i)
    out.push({ y: d.getFullYear(), m: d.getMonth(), d: d.getDate(), out: d.getMonth() !== m })
  }
  return out
})

function isSel(c) {
  const p = picked.value
  return !!p && p.y === c.y && p.m === c.m && p.d === c.d
}
function isToday(c) {
  const t = parse(todayStr.value)
  return !!t && t.y === c.y && t.m === c.m && t.d === c.d
}

function place() {
  const el = anchor.value
  if (!el) return
  const r = el.getBoundingClientRect()
  let left = r.left + r.width / 2 - PANEL_W / 2
  left = Math.max(10, Math.min(left, window.innerWidth - PANEL_W - 10))
  let top = r.bottom + 10
  if (top + PANEL_H > window.innerHeight - 10) top = Math.max(10, r.top - PANEL_H - 10)
  pos.value = { left: Math.round(left), top: Math.round(top) }
}

function show() {
  const t = new Date()
  const p = picked.value
  cur.value = p ? { y: p.y, m: p.m } : { y: t.getFullYear(), m: t.getMonth() }
  place()
  open.value = true
  nextTick(place)
  // 捕获阶段监听：面板开着时 Esc 只关面板，不要连带关掉整个编辑页
  window.addEventListener('pointerdown', onDocDown, true)
  window.addEventListener('keydown', onDocKey, true)
  window.addEventListener('scroll', close, true)
  window.addEventListener('resize', place)
}

function close() {
  if (!open.value) return
  open.value = false
  window.removeEventListener('pointerdown', onDocDown, true)
  window.removeEventListener('keydown', onDocKey, true)
  window.removeEventListener('scroll', close, true)
  window.removeEventListener('resize', place)
}

function toggle() {
  open.value ? close() : show()
}

function onDocDown(e) {
  if (panel.value?.contains(e.target) || anchor.value?.contains(e.target)) return
  close()
}

function onDocKey(e) {
  if (e.key !== 'Escape') return
  e.preventDefault()
  e.stopPropagation()
  close()
}

function pick(c) {
  const v = ymd(c.y, c.m, c.d)
  emit('update:modelValue', v)
  emit('change', v)
  close()
}

function pickToday() {
  const t = parse(todayStr.value)
  pick({ y: t.y, m: t.m, d: t.d })
}

function shiftMonth(d) {
  const { y, m } = cur.value
  const n = new Date(y, m + d, 1)
  cur.value = { y: n.getFullYear(), m: n.getMonth() }
}
function shiftYear(d) {
  cur.value = { y: cur.value.y + d, m: cur.value.m }
}

// 截图调试：?dpop 直接把面板打开
onMounted(() => {
  if (location.search.includes('dpop')) nextTick(show)
})

onBeforeUnmount(close)
</script>

<template>
  <button
    ref="anchor"
    type="button"
    class="dp"
    :class="[variant, { open }]"
    @click="toggle"
  >
    <svg class="ico" width="13" height="13" viewBox="0 0 16 16" aria-hidden="true">
      <rect x="2.2" y="3.4" width="11.6" height="10.4" rx="2.2" fill="none" stroke="currentColor" stroke-width="1.1" />
      <path d="M2.4 6.6h11.2" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" />
      <path d="M5.4 2.2v2.4M10.6 2.2v2.4" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" />
    </svg>
    <span class="val">{{ label }}</span>
    <svg class="car" width="9" height="9" viewBox="0 0 10 10" aria-hidden="true">
      <path d="M2 4l3 3 3-3" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </button>

  <Teleport to="body">
    <Transition name="dp">
      <div
        v-if="open"
        ref="panel"
        class="dp-panel"
        :style="{ left: pos.left + 'px', top: pos.top + 'px' }"
      >
        <div class="head">
          <button class="nav" title="上一年" @click="shiftYear(-1)">
            <svg width="11" height="11" viewBox="0 0 12 12"><path d="M7.5 2L3.5 6l4 4M11 2L7 6l4 4" fill="none" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </button>
          <button class="nav" title="上个月" @click="shiftMonth(-1)">
            <svg width="11" height="11" viewBox="0 0 12 12"><path d="M7.5 2L3.5 6l4 4" fill="none" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </button>
          <div class="ttl">{{ cur.y }} 年 {{ cur.m + 1 }} 月</div>
          <button class="nav" title="下个月" @click="shiftMonth(1)">
            <svg width="11" height="11" viewBox="0 0 12 12"><path d="M4.5 2l4 4-4 4" fill="none" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </button>
          <button class="nav" title="下一年" @click="shiftYear(1)">
            <svg width="11" height="11" viewBox="0 0 12 12"><path d="M4.5 2l4 4-4 4M1 2l4 4-4 4" fill="none" stroke="currentColor" stroke-width="1.1" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </button>
        </div>

        <div class="week">
          <i v-for="(w, i) in WEEK" :key="w" :class="{ we: i >= 5 }">{{ w }}</i>
        </div>

        <div class="grid">
          <button
            v-for="(c, i) in cells"
            :key="i"
            type="button"
            class="day"
            :class="{ out: c.out, sel: isSel(c), today: isToday(c), we: i % 7 >= 5 }"
            @click="pick(c)"
          >
            {{ c.d }}
          </button>
        </div>

        <div class="foot">
          <span class="now">{{ picked ? '已选 ' + label : '还没有选择' }}</span>
          <button class="todaybtn" @click="pickToday">今天</button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* ---------- 触发器 ---------- */
.dp {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  transition: color 0.2s, border-color 0.2s, background 0.2s;
}

.dp .ico,
.dp .car {
  flex: none;
  opacity: 0.72;
}

.dp .car {
  transition: transform 0.24s;
}

.dp.open .car {
  transform: rotate(180deg);
}

/* ghost：编辑器里那行居中的粉色日期 */
.dp.ghost {
  padding: 4px 8px;
  border-radius: 8px;
  font-size: 12.5px;
  letter-spacing: 0.14em;
  color: rgba(255, 158, 196, 0.88);
  background: transparent;
}

.dp.ghost:hover {
  color: var(--pink);
  background: rgba(255, 158, 196, 0.07);
}

.dp.ghost.open {
  color: #fff;
  background: rgba(255, 158, 196, 0.12);
}

/* field：设置页里当输入框用 */
.dp.field {
  width: 100%;
  max-width: 240px;
  padding: 10px 13px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  font-size: 13.5px;
  letter-spacing: 0.05em;
  color: var(--ink);
}

.dp.field:hover {
  border-color: rgba(255, 255, 255, 0.14);
}

.dp.field.open {
  border-color: var(--pink-soft);
  background: rgba(255, 255, 255, 0.06);
}

.dp.field .val {
  color: rgba(255, 158, 196, 0.9);
}

/* ---------- 面板 ---------- */
.dp-panel {
  position: fixed;
  z-index: 400;
  width: 274px;
  padding: 12px 12px 10px;
  border-radius: 16px;
  background: linear-gradient(165deg, rgba(22, 26, 44, 0.97), rgba(9, 11, 20, 0.985));
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 26px 60px rgba(0, 0, 0, 0.68), 0 0 34px rgba(255, 158, 196, 0.07),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(14px);
  user-select: none;
  overflow: hidden;
}

.head {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px 2px 10px;
}

.head .ttl {
  flex: 1;
  text-align: center;
  font-size: 12.5px;
  letter-spacing: 0.12em;
  color: rgba(233, 238, 247, 0.9);
}

.nav {
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  color: rgba(233, 238, 247, 0.42);
  transition: all 0.18s;
}

.nav:hover {
  color: var(--pink);
  background: rgba(255, 158, 196, 0.1);
}

.week {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  padding-bottom: 4px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.055);
  margin-bottom: 4px;
}

.week i.we {
  color: rgba(255, 158, 196, 0.42);
}

.week i {
  text-align: center;
  font-style: normal;
  font-size: 10.5px;
  letter-spacing: 0.06em;
  color: rgba(233, 238, 247, 0.3);
}

.grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px 0;
}

.day {
  justify-self: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
  color: rgba(233, 238, 247, 0.82);
  transition: background 0.16s, color 0.16s, box-shadow 0.2s;
}

.day.we {
  color: rgba(255, 190, 212, 0.82);
}

.day.out {
  color: rgba(233, 238, 247, 0.16);
}

.day:hover {
  background: rgba(255, 158, 196, 0.13);
  color: #fff;
}

.day.today {
  color: var(--pink);
  box-shadow: inset 0 0 0 1px rgba(255, 158, 196, 0.45);
}

.day.sel {
  background: linear-gradient(135deg, #ffb3d1, #ff7fb0);
  color: #1a0810;
  font-weight: 600;
  box-shadow: 0 3px 14px rgba(255, 126, 176, 0.42);
}

.day.sel:hover {
  color: #1a0810;
}

.foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.055);
}

.foot .now {
  font-size: 10.5px;
  letter-spacing: 0.04em;
  color: rgba(233, 238, 247, 0.32);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.todaybtn {
  flex: none;
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 11px;
  letter-spacing: 0.08em;
  color: rgba(255, 158, 196, 0.9);
  border: 1px solid rgba(255, 158, 196, 0.28);
  transition: all 0.2s;
}

.todaybtn:hover {
  color: #1a0810;
  background: var(--pink);
  border-color: var(--pink);
}

/* 出入场 */
.dp-enter-active,
.dp-leave-active {
  transition: opacity 0.18s ease, transform 0.18s cubic-bezier(0.2, 0.8, 0.3, 1);
}

.dp-enter-from,
.dp-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.97);
}
</style>
