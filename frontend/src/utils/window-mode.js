// Переключение окна между расширенным и простым (компактным) режимом.
//
// Геометрию окна меняет Go (app: SaveAdvancedWindow / ResizeWindowBy / RestoreAdvancedWindow,
// см. window_mode.go): вызовы Wails runtime из JS выполняются бэкендом параллельно и без
// гарантии порядка, а координаты WindowGetPosition/WindowSetPosition на Windows не совпадают,
// поэтому восстанавливать позицию из JS ненадёжно. Go держит окно в рабочей области монитора.
import {
  EnsureWindowOnScreen,
  ResizeWindowBy,
  RestoreAdvancedWindow,
  SaveAdvancedWindow
} from '../../wailsjs/go/main/App'
import { WindowSetAlwaysOnTop } from '../../wailsjs/runtime/runtime'

export const WINDOW_MODE_STORAGE_KEY = 'lt.window.mode'
export const SIMPLE_ON_TOP_STORAGE_KEY = 'lt.simple.on-top'
export const SIMPLE_TUNNEL_STORAGE_KEY = 'lt.simple.tunnel-id'

// Размер содержимого простого режима (без строки заголовка)
export const SIMPLE_CONTENT_WIDTH = 413
export const SIMPLE_CONTENT_HEIGHT = 181
// Собственная строка заголовка (AppTitleBar.vue) — часть WebView на Windows (окно без рамки)
export const TITLE_BAR_HEIGHT = 32

function hasBackend() {
  return typeof window !== 'undefined' && !!window.go?.main?.App
}

function waitForResize(timeoutMs = 250) {
  return new Promise((resolve) => {
    let done = false
    const finish = () => {
      if (done) return
      done = true
      window.removeEventListener('resize', finish)
      requestAnimationFrame(() => resolve())
    }
    window.addEventListener('resize', finish)
    setTimeout(finish, timeoutMs)
  })
}

// Запоминает размер и позицию расширенного окна (вызывать до перехода в простой режим).
export async function captureAdvancedBounds() {
  if (!hasBackend()) return
  try {
    await SaveAdvancedWindow()
  } catch (_) {
    /* best-effort */
  }
}

// Подгоняет окно так, чтобы область WebView стала SIMPLE_CONTENT_WIDTH × (SIMPLE_CONTENT_HEIGHT +
// своя строка заголовка, если она есть). Рамка окна вне WebView (системный заголовок на macOS
// и Linux, у окна без рамки на Windows её нет) зависит от системы, поэтому меняем внешний размер
// на разницу с текущим innerWidth/innerHeight и при необходимости делаем один проход поправки.
export async function applySimpleWindow({ onTop = false, titleBar = false } = {}) {
  if (!hasBackend()) return
  const height = SIMPLE_CONTENT_HEIGHT + (titleBar ? TITLE_BAR_HEIGHT : 0)
  try {
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const dw = SIMPLE_CONTENT_WIDTH - window.innerWidth
      const dh = height - window.innerHeight
      if (dw === 0 && dh === 0) break
      await ResizeWindowBy(dw, dh)
      await waitForResize()
    }
    setSimpleAlwaysOnTop(onTop)
  } catch (_) {
    // окно останется прежнего размера — простой режим всё равно работает
  }
}

export function setSimpleAlwaysOnTop(onTop) {
  if (typeof window === 'undefined' || !window.runtime) return
  try {
    WindowSetAlwaysOnTop(!!onTop)
  } catch (_) {
    /* best-effort */
  }
}

export async function restoreAdvancedWindow() {
  if (!hasBackend()) return
  try {
    await RestoreAdvancedWindow()
  } catch (_) {
    /* best-effort */
  }
}

// Страховка при старте: если окно оказалось за пределами экрана — вернуть его.
export async function ensureWindowOnScreen() {
  if (!hasBackend()) return
  try {
    await EnsureWindowOnScreen()
  } catch (_) {
    /* best-effort */
  }
}
