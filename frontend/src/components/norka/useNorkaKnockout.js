// Пасхалка «нокаут» (см. norka-knockout.js): считает клики и управляет показом варианта.
import { onBeforeUnmount, ref } from 'vue'
import {
  KNOCKOUT_CLICKS,
  KNOCKOUT_COOLDOWN_MS,
  KNOCKOUT_FADE_MS,
  KNOCKOUT_REDUCED,
  KNOCKOUT_VARIANTS,
  KNOCKOUT_WINDOW_MS,
  pickKnockout
} from './norka-knockout'
import { useReducedMotion } from './useNorkaIdle'

/**
 * @param enabled функция/ref: принимает ли этот экземпляр клики
 * @returns knockout — текущий вариант или null; leaving — идёт возврат к обычным глазам;
 *          still — статичная версия (prefers-reduced-motion); press — обработчик клика
 */
export function useNorkaKnockout(enabled) {
  const reduced = useReducedMotion()
  const read = (v) => (typeof v === 'function' ? v() : v?.value)
  const knockout = ref(null)
  const leaving = ref(false)
  const still = ref(false)
  let clicks = []
  let quietUntil = 0
  let prev = null
  let timers = []

  const later = (fn, ms) => timers.push(setTimeout(fn, ms))

  function start() {
    const calm = reduced.value
    const name = calm ? KNOCKOUT_REDUCED.variant : pickKnockout(prev)
    const duration = calm ? KNOCKOUT_REDUCED.duration : KNOCKOUT_VARIANTS[name]
    prev = name
    still.value = calm
    leaving.value = false
    knockout.value = name
    later(() => { leaving.value = true }, duration - KNOCKOUT_FADE_MS)
    later(() => {
      knockout.value = null
      leaving.value = false
      quietUntil = Date.now() + KNOCKOUT_COOLDOWN_MS
      timers = []
    }, duration)
  }

  // клики во время нокаута и паузы после него не считаются
  function press() {
    if (!read(enabled)) return
    const now = Date.now()
    if (knockout.value || now < quietUntil) {
      clicks = []
      return
    }
    clicks = clicks.filter((time) => now - time < KNOCKOUT_WINDOW_MS)
    clicks.push(now)
    if (clicks.length >= KNOCKOUT_CLICKS) {
      clicks = []
      start()
    }
  }

  onBeforeUnmount(() => timers.forEach(clearTimeout))

  return { knockout, leaving, still, press }
}
