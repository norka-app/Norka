// Поведение Norka в простое: через случайные паузы проигрывает короткие микроанимации
// (Web Animations API поверх SVG NorkaIcon), веса и паузы — в norka-idle.js.
// Ничего не планируется при prefers-reduced-motion, в статусе «Подключение» и пока окно
// спрятано или свёрнуто. Таймеры только между действиями — постоянного цикла кадров нет.
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { isWindowShown, useWindowVisible } from '../../utils/window-visibility'
import {
  DOZE_AFTER_MS,
  DOZE_OFF,
  IDLE_STATES,
  WAKE_UP,
  idleKeyframes,
  idleStateFor,
  pickIdleAnim,
  randomBetween
} from './norka-idle'

const INPUT_EVENTS = ['pointermove', 'pointerdown', 'keydown', 'wheel']

function useReducedMotion() {
  const reduced = ref(false)
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return reduced
  const media = window.matchMedia('(prefers-reduced-motion: reduce)')
  reduced.value = media.matches
  const onChange = () => { reduced.value = media.matches }
  media.addEventListener('change', onChange)
  onBeforeUnmount(() => media.removeEventListener('change', onChange))
  return reduced
}

/**
 * @param host    ref на элемент, внутри которого лежит SVG NorkaIcon
 * @param status  функция/ref со статусом иконки (connected | connecting | error | partial | stopped)
 * @param enabled функция/ref: включено ли поведение для этого экземпляра (и не идёт ли подмигивание)
 */
export function useNorkaIdle(host, status, enabled) {
  const reduced = useReducedMotion()
  const windowVisible = useWindowVisible()
  const read = (v) => (typeof v === 'function' ? v() : v?.value)

  let timer = 0
  let dozeTimer = 0
  let running = [] // текущие Animation
  let dozing = null // Animation[] засыпания (держат позу до пробуждения)
  let prev = null
  let lastInput = Date.now()
  let dozeAfter = randomBetween(DOZE_AFTER_MS)
  let token = 0

  const stateName = () => idleStateFor(read(status))
  const active = () => !!read(enabled) && !reduced.value && windowVisible.value && !!stateName()

  function parts(side) {
    const root = host.value
    const eye = root?.querySelector(`[data-side="${side}"] .norka-eye`)
    if (!eye) return null
    return {
      eye,
      look: eye.querySelector('.norka-look'),
      closed: root.querySelector(`[data-side="${side}"] .norka-closed`)
    }
  }

  // Проигрывает ключи на обоих глазах; hold — оставить конечную позу (засыпание).
  function play(keys, sleepy, hold = false) {
    const anims = []
    for (const [side, name] of [['l', 'left'], ['r', 'right']]) {
      const el = parts(name)
      if (!el) continue
      const frames = idleKeyframes(keys, side, sleepy)
      const opts = { duration: frames.duration, fill: hold ? 'forwards' : 'none' }
      anims.push(el.eye.animate(frames.eye, opts))
      if (el.look) anims.push(el.look.animate(frames.look, opts))
      if (sleepy && el.closed) anims.push(el.closed.animate(frames.closed, opts))
    }
    return anims
  }

  const finished = (anims) => Promise.all(anims.map((a) => a.finished)).catch(() => {})

  function cancelAll() {
    token += 1
    for (const a of running) a.cancel()
    running = []
    if (dozing) dozing.forEach((a) => a.cancel())
    dozing = null
  }

  function stop() {
    clearTimeout(timer)
    clearTimeout(dozeTimer)
    timer = 0
    dozeTimer = 0
    cancelAll()
  }

  function scheduleNext() {
    clearTimeout(timer)
    if (!active()) return
    timer = setTimeout(tick, randomBetween(IDLE_STATES[stateName()].interval))
  }

  async function tick() {
    const name = stateName()
    if (!active() || dozing || running.length) return scheduleNext()
    const my = ++token
    if (!(await isWindowShown()) || my !== token || !active()) return scheduleNext()
    const cfg = IDLE_STATES[name]
    const anim = pickIdleAnim(cfg.weights, prev)
    prev = anim
    running = play(cfg.anims[anim], name === 'stopped')
    await finished(running)
    if (my === token) running = []
    scheduleNext()
  }

  // Засыпание: только в «подключено», после 3–5 минут без ввода в окне.
  function armDoze() {
    clearTimeout(dozeTimer)
    if (!active() || stateName() !== 'connected' || dozing) return
    const left = lastInput + dozeAfter - Date.now()
    dozeTimer = setTimeout(async () => {
      if (Date.now() - lastInput < dozeAfter) return armDoze()
      if (!active() || stateName() !== 'connected' || !(await isWindowShown())) return armDoze()
      for (const a of running) a.cancel()
      running = []
      dozing = play(DOZE_OFF, false, true)
    }, Math.max(1000, left))
  }

  async function wake() {
    const held = dozing
    dozing = null
    if (!held) return
    const my = ++token
    running = active() ? play(WAKE_UP, false) : []
    held.forEach((a) => a.cancel())
    await finished(running)
    if (my === token) running = []
  }

  function onInput() {
    lastInput = Date.now()
    if (dozing) {
      dozeAfter = randomBetween(DOZE_AFTER_MS)
      void wake().then(() => { scheduleNext(); armDoze() })
    }
  }

  function restart() {
    stop()
    if (!active()) return
    lastInput = Date.now()
    scheduleNext()
    armDoze()
  }

  onMounted(() => {
    INPUT_EVENTS.forEach((type) => window.addEventListener(type, onInput, { passive: true, capture: true }))
    restart()
  })
  watch([() => read(status), () => read(enabled), reduced, windowVisible], restart)
  onBeforeUnmount(() => {
    INPUT_EVENTS.forEach((type) => window.removeEventListener(type, onInput, { capture: true }))
    stop()
  })
}
