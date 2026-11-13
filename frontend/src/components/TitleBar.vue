<script setup>
import { win } from '../api'

const maximise = () => win.toggleMaximise()
</script>

<template>
  <div class="titlebar" style="--wails-draggable: drag" @dblclick.self="maximise">
    <div class="winbtns" style="--wails-draggable: no-drag">
      <button title="最小化" @click="win.minimise()">
        <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2 5h6" stroke="currentColor" stroke-width="1.2" /></svg>
      </button>
      <button title="最大化" @click="maximise">
        <svg width="10" height="10" viewBox="0 0 10 10"><rect x="2" y="2" width="6" height="6" fill="none" stroke="currentColor" stroke-width="1.1" /></svg>
      </button>
      <button class="close" title="关闭" @click="win.quit()">
        <svg width="10" height="10" viewBox="0 0 10 10"><path d="M2.2 2.2l5.6 5.6M7.8 2.2L2.2 7.8" stroke="currentColor" stroke-width="1.2" /></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.titlebar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0 6px;
  z-index: 120;
  background: linear-gradient(180deg, rgba(5, 6, 13, 0.85), rgba(5, 6, 13, 0));
}

.winbtns {
  display: flex;
  gap: 2px;
}

.winbtns button {
  width: 34px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  color: rgba(233, 238, 247, 0.5);
  transition: background 0.18s, color 0.18s;
}

.winbtns button:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.winbtns button.close:hover {
  background: rgba(232, 62, 90, 0.85);
  color: #fff;
}
</style>
