// Видно ли окно приложения. WebView2 в Wails не всегда меняет document.visibilityState,
// когда окно прячется в трей или сворачивается, поэтому учитываем ещё событие Go
// 'window:visibility' (window_chrome.go: hide/show и пути трея) и собственное «Свернуть».
import { readonly, ref } from 'vue'
import { EventsOn, WindowIsMinimised } from '../../wailsjs/runtime/runtime'

const hiddenByApp = ref(false)
const documentHidden = ref(typeof document !== 'undefined' && document.visibilityState === 'hidden')
const visible = ref(!hiddenByApp.value && !documentHidden.value)
let subscribed = false

function update() {
  visible.value = !hiddenByApp.value && !documentHidden.value
}

export function setWindowShown(shown) {
  hiddenByApp.value = !shown
  update()
}

function subscribe() {
  if (subscribed || typeof window === 'undefined') return
  subscribed = true
  document.addEventListener('visibilitychange', () => {
    documentHidden.value = document.visibilityState === 'hidden'
    update()
  })
  // после сворачивания окно возвращается без события Go — фокус или ввод значат, что его снова видно
  const shown = () => { if (hiddenByApp.value) setWindowShown(true) }
  window.addEventListener('focus', shown)
  window.addEventListener('pointerdown', shown, true)
  if (window.runtime) EventsOn('window:visibility', (isVisible) => setWindowShown(isVisible !== false))
}

// Реактивный флаг видимости окна (подписка создаётся при первом вызове).
export function useWindowVisible() {
  subscribe()
  return readonly(visible)
}

// Точная проверка перед анимацией: окно видно и не свёрнуто (свернуть можно и с панели задач).
export async function isWindowShown() {
  subscribe()
  if (!visible.value) return false
  if (typeof window === 'undefined' || !window.runtime) return true
  try {
    return !(await WindowIsMinimised())
  } catch (_) {
    return true
  }
}
