// Утверждённые размеры Dynamic Island при 1920×1080 и масштабе 100%.
export const ISLAND_TOP_GAP = 12

export const ISLAND_SIZE = {
  idle: { w: 96, h: 36, r: 18 },
  connecting: { w: 360, h: 36, r: 18 },
  one: { w: 360, h: 36, r: 18 },
  multi: { w: 280, h: 36, r: 18 },
  error: { w: 360, h: 36, r: 18 },
  expanded: { w: 360, h: 280, r: 32 },
  conflict: { w: 420, h: 52, r: 26 },
}

// Длительности морфа из утверждённого брифа, миллисекунды.
export function islandDuration(fromPhase, toPhase) {
  if (toPhase === 'expanded') return 350
  if (fromPhase === 'expanded') return 280
  if (toPhase === 'conflict' || fromPhase === 'conflict') return 250
  if (toPhase === 'error') return 200
  if (toPhase === 'connecting') return 300
  if (fromPhase === 'connecting') return 200
  if (toPhase === 'multi' || fromPhase === 'multi') return 250
  return 220
}

export function islandSize(phase) {
  return ISLAND_SIZE[phase] || ISLAND_SIZE.idle
}

export function prefersReducedIslandMotion() {
  return typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Мягкая пружина: небольшой перелёт и возврат, без ухода в минус.
export function islandEase(t) {
  const clamped = Math.min(1, Math.max(0, t))
  const c1 = 1.12
  const c3 = c1 + 1
  return 1 + c3 * (clamped - 1) ** 3 + c1 * (clamped - 1) ** 2
}

export function lerpIsland(from, to, t) {
  const e = islandEase(t)
  const mix = (a, b) => Math.max(1, Math.round(a + (b - a) * e))
  return {
    w: mix(from.w, to.w),
    h: mix(from.h, to.h),
    r: Math.max(0, Math.round(from.r + (to.r - from.r) * e)),
  }
}
