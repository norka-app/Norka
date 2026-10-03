import { reactive } from 'vue'

// Рамка, с которой открылся этот процесс. Переключатель в настройках
// меняет config.toml сразу, а этот объект — только после перезапуска.
export const windowChrome = reactive({
  ready: false,
  frameless: false,
  platform: '',
  backdrop: 'native',
  settingsTab: 'general',
})

function osAttr(platform) {
  if (platform === 'darwin') return 'mac'
  if (platform === 'windows') return 'win'
  if (platform === 'linux') return 'linux'
  return ''
}

export function applyWindowChrome(chrome) {
  windowChrome.frameless = !!chrome?.frameless
  windowChrome.platform = chrome?.platform || ''
  windowChrome.backdrop = chrome?.backdrop || 'native'
  windowChrome.ready = true
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.classList.toggle('frameless', windowChrome.frameless)
  if (!windowChrome.frameless) {
    delete root.dataset.os
    delete root.dataset.backdrop
    return
  }
  const os = osAttr(windowChrome.platform)
  if (os) root.dataset.os = os
  root.dataset.backdrop = windowChrome.backdrop
}

export function isLinuxChrome() {
  if (windowChrome.platform === 'linux') return true
  if (windowChrome.ready) return false
  if (typeof navigator === 'undefined') return false
  return /Linux/.test(navigator.userAgent) && !/Android/.test(navigator.userAgent)
}
