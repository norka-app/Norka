import { createApp } from 'vue'
import naive from 'naive-ui'
import App from './App.vue'
import i18n from './i18n'
import { APP_FONT_FAMILY } from './theme/naive-theme'
import './style.css';

document.documentElement.style.setProperty('--norka-font-family', APP_FONT_FAMILY)
import './styles/form-kit.css';
import './styles/frameless.css';
import { LogError } from '../wailsjs/runtime/runtime'

// Safety net: without this, an uncaught error during render/reactivity (e.g. an
// unescaped special character in a translation string) aborts the whole Vue
// render and leaves a blank window with no diagnostics. Forwarding to the Go
// log lets us grab details from the packaged binary's stdout instead of
// re-instrumenting the app every time this class of bug resurfaces.
function reportRendererError(source, err, detail) {
  const message = err && err.message ? err.message : String(err)
  try {
    LogError(`[renderer ${source}] ${message}${detail ? ` | ${detail}` : ''}`)
  } catch (_) {
    // LogError requires the Wails runtime bridge; ignore if unavailable (e.g. during `vite build` preview).
  }
  console.error(`[renderer ${source}]`, err, detail)
}

window.addEventListener('error', (event) => {
  reportRendererError('window.onerror', event.error || event.message, `${event.filename}:${event.lineno}:${event.colno}`)
})
window.addEventListener('unhandledrejection', (event) => {
  reportRendererError('unhandledrejection', event.reason)
})

// Never let the WebView open a file dropped onto the window (it would replace
// the app with the file's contents). Registered before the app mounts so it is
// active from the first frame; preventDefault does not stop propagation, so the
// Wails file-drop handler on window and the overlay in App.vue still run.
for (const type of ['dragover', 'drop']) {
  document.addEventListener(type, (event) => {
    if (Array.from(event.dataTransfer?.types || []).includes('Files')) {
      event.preventDefault()
    }
  })
}

const app = createApp(App)
app.config.errorHandler = (err, instance, info) => {
  reportRendererError('vue.errorHandler', err, info)
}
app.use(i18n)
app.use(naive)
app.mount('#app')
