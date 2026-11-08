// Геометрия Dynamic Island. Размер капсулы меняет Go (PlaceIslandCapsule),
// чтобы окно совпало с пилюлей и осталось у верхнего края экрана.
// Скругление рисует CSS: маска окна даёт лесенку.
import {
  ClearIslandChrome,
  PlaceIslandCapsule,
  SetAdvancedChrome,
  SetWindowPresentation,
} from '../../wailsjs/go/main/App'
import { islandDuration, islandEase, prefersReducedIslandMotion } from '../components/island/island-metrics'

function hasBackend() {
  return typeof window !== 'undefined' && !!window.go?.main?.App
}

export async function placeIslandCapsule(metrics) {
  if (!hasBackend() || !metrics) return
  try {
    await PlaceIslandCapsule(metrics.w, metrics.h, metrics.r)
  } catch (_) {
    /* капсула останется прежнего размера */
  }
}

export async function clearIslandChrome() {
  if (!hasBackend()) return
  try {
    await ClearIslandChrome()
  } catch (_) {
    /* best-effort */
  }
}

export async function setAdvancedChrome(enabled) {
  if (!hasBackend()) return
  try {
    await SetAdvancedChrome(!!enabled)
  } catch (_) {
    /* без бэкенда размер не блокируется */
  }
}

export async function syncWindowPresentation(mode, compactRestore) {
  if (!hasBackend()) return
  try {
    await SetWindowPresentation(mode, !!compactRestore)
  } catch (_) {
    /* трей обновится следующим опросом, если метод ещё не собран */
  }
}

let placeChain = Promise.resolve()

function queueCapsule(metrics) {
  placeChain = placeChain.then(() => placeIslandCapsule(metrics))
  return placeChain
}

// animateIslandCapsule меняет окно от from к to. onFrame получает текущий размер.
export function animateIslandCapsule(from, to, onFrame) {
  const ms = prefersReducedIslandMotion() ? 0 : islandDuration(from?.phase, to?.phase)
  const start = typeof performance !== 'undefined' ? performance.now() : 0
  let frame = 0
  const step = (now) => {
    const t = ms === 0 ? 1 : Math.min(1, (now - start) / ms)
    const e = ms === 0 ? 1 : islandEase(t)
    const metrics = {
      phase: to.phase,
      w: Math.max(1, Math.round(from.w + (to.w - from.w) * e)),
      h: Math.max(1, Math.round(from.h + (to.h - from.h) * e)),
      r: Math.max(0, Math.round(from.r + (to.r - from.r) * e)),
    }
    onFrame?.(metrics)
    frame += 1
    if (t === 1 || frame % 2 === 0) void queueCapsule(metrics)
    if (t < 1) requestAnimationFrame(step)
  }
  requestAnimationFrame(step)
}
