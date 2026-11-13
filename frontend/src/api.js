// 与 Go 后端通信的薄封装。Wails 在页面里注入 window.go.main.App。
const bridge = () => window?.go?.main?.App

function call(name, ...args) {
  const api = bridge()
  const fn = api && api[name]
  if (typeof fn !== 'function') {
    return Promise.reject(new Error('后端尚未就绪'))
  }
  return fn(...args)
}

export function waitForBridge(timeout = 8000) {
  return new Promise((resolve, reject) => {
    const started = Date.now()
    const tick = () => {
      if (bridge()) return resolve(true)
      if (Date.now() - started > timeout) return reject(new Error('等待后端超时'))
      setTimeout(tick, 60)
    }
    tick()
  })
}

// 浏览器预览时用生成的占位图代替真实照片
function mockImage(name) {
  const m = String(name).match(/\d+/)
  const i = m ? Number(m[0]) : 1
  const hue = (i * 67) % 360
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="900" height="620">` +
    `<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">` +
    `<stop offset="0" stop-color="hsl(${hue},58%,52%)"/>` +
    `<stop offset="1" stop-color="hsl(${(hue + 70) % 360},50%,22%)"/>` +
    `</linearGradient></defs>` +
    `<rect width="900" height="620" fill="url(#g)"/>` +
    `<circle cx="${300 + i * 40}" cy="250" r="90" fill="rgba(255,255,255,.18)"/>` +
    `<text x="450" y="350" font-size="96" fill="rgba(255,255,255,.55)" text-anchor="middle" font-family="sans-serif">${i}</text>` +
    `</svg>`
  return 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg)
}

export const api = {
  list: () => call('ListEvents'),
  get: (id) => call('GetEvent', id),
  save: (e) => call('SaveEvent', e),
  remove: (id) => call('DeleteEvent', id),
  pickImages: () => call('PickImages'),
  imageData: (name) =>
    bridge() ? call('ImageData', name) : Promise.resolve(mockImage(name)),
  settings: () => call('GetSettings'),
  setLoveStart: (date) => call('SetLoveStart', date),
  chooseDataDir: () => call('ChooseDataDir'),
  openDataDir: () => call('OpenDataDir'),
}

export const win = {
  minimise: () => window.runtime?.WindowMinimise?.(),
  toggleMaximise: () => window.runtime?.WindowToggleMaximise?.(),
  quit: () => window.runtime?.Quit?.(),
}

// 2026-11-12 -> 2026.11.12
export function prettyDate(d) {
  if (!d) return ''
  return d.replace(/-/g, '.')
}

export function today() {
  const t = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())}`
}
