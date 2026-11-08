// Геометрия Dynamic Island. Размер капсулы меняет Go (PlaceIslandCapsule),
// чтобы окно совпало с пилюлей и осталось у верхнего края экрана.
import {
  ClearIslandChrome,
  IslandShapeSupported,
  LayoutIslandStage,
  PlaceIslandCapsule,
  PlaceIslandHost,
  SetIslandBackdrop,
  SetIslandShapes,
  SetWindowPresentation,
} from '../../wailsjs/go/main/App'
import { islandDuration, islandEase, prefersReducedIslandMotion } from '../components/island/island-metrics'

function hasBackend() {
  return typeof window !== 'undefined' && !!window.go?.main?.App
}

export async function detectIslandShape() {
  if (!hasBackend()) return false
  try {
    return (await IslandShapeSupported()) === true
  } catch (_) {
    return false
  }
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

export async function paintIslandBackdrop(theme) {
  if (!hasBackend()) return
  try {
    await SetIslandBackdrop(theme === 'dark', false)
  } catch (_) {
    /* фон окна не обязателен */
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

// animateIslandCapsule меняет окно от from к to. onFrame получает текущий размер,
// чтобы радиус капсулы совпадал с вырезом окна.
export function animateIslandCapsule(from, to, onFrame) {
  const ms = prefersReducedIslandMotion() ? 0 : islandDuration(from?.phase, to?.phase)
  const start = typeof performance !== 'undefined' ? performance.now() : 0
  const token = {}
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
  return token
}

export async function openIslandHost() {
  if (!hasBackend()) return null
  try {
    const host = await PlaceIslandHost()
    if (!host?.ok) return null
    return host
  } catch (_) {
    return null
  }
}

export async function layoutIslandStage(hostW, hostH, island, panel) {
  const fallback = {
    islandX: Math.max(0, Math.round((hostW - island.w) / 2)),
    islandY: 12,
    islandW: island.w,
    islandH: island.h,
    islandR: island.r,
    panelX: panel?.x ?? 0,
    panelY: panel?.y ?? 76,
    panelW: panel?.w ?? 1024,
    panelH: panel?.h ?? 768,
    panelR: 14,
  }
  if (!hasBackend()) return fallback
  try {
    const layout = await LayoutIslandStage(
      hostW,
      hostH,
      island.w,
      island.h,
      island.r,
      panel?.x ?? -1,
      panel?.y ?? -1,
      panel?.w ?? 0,
      panel?.h ?? 0,
    )
    return layout || fallback
  } catch (_) {
    return fallback
  }
}

export async function setIslandShapes(shapes) {
  if (!hasBackend()) return
  try {
    await SetIslandShapes(shapes)
  } catch (_) {
    /* без вырезания капсула всё равно рисуется */
  }
}
