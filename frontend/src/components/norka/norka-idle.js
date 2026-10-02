// Поведение Norka в простое: данные микроанимаций и выбор следующей.
// Поза глаза: s — раскрытость (1 — обычный, 0 — закрыт), x / y — направление взгляда (-1…1).
// Ключ: { ms, pose, ease }, pose — { s, x, y } для обоих глаз или { l: {...}, r: {...} };
// ease — плавность участка, который заканчивается на этом ключе.

const both = (pose) => ({ l: { ...pose }, r: { ...pose } })
const K = (ms, pose, ease = 'inOut') => ({ ms, pose: pose.l ? pose : both(pose), ease })
const blink = (t0, x = 0) => [
  K(t0, { s: 1, x }), K(t0 + 70, { s: 0.06, x }, 'in'), K(t0 + 120, { s: 0.06, x }), K(t0 + 250, { s: 1, x }, 'out')
]

export const IDLE_EASING = {
  inOut: 'cubic-bezier(0.45, 0, 0.55, 1)',
  out: 'cubic-bezier(0.33, 1, 0.68, 1)',
  in: 'cubic-bezier(0.32, 0, 0.67, 0)',
  linear: 'linear'
}

// Состояния: interval — пауза между действиями (мс), weights — веса случайного выбора.
export const IDLE_STATES = {
  // подключено — бодрая
  connected: {
    rest: 1,
    interval: [8000, 20000],
    weights: { blink: 40, doubleBlink: 15, glance: 22, squint: 10, wink: 5 },
    anims: {
      blink: blink(0),
      doubleBlink: [...blink(0), ...blink(380)],
      glance: [
        K(0, { s: 1, x: 0 }), K(160, { s: 1, x: -1 }, 'out'), K(1000, { s: 1, x: -1 }),
        K(1060, { s: 0.7, x: -0.2 }), K(1200, { s: 1, x: 1 }, 'out'), K(1950, { s: 1, x: 1 }), K(2150, { s: 1, x: 0 }, 'out')
      ],
      squint: [
        K(0, { s: 1, x: 0 }), K(320, { l: { s: 0.4, x: -0.5 }, r: { s: 0.52, x: -0.5 } }, 'out'),
        K(1500, { l: { s: 0.4, x: 0.6 }, r: { s: 0.52, x: 0.6 } }), K(1900, { l: { s: 0.4, x: 0.6 }, r: { s: 0.52, x: 0.6 } }),
        K(2200, { s: 1, x: 0 }, 'out')
      ],
      wink: [
        K(0, { s: 1, x: 0 }), K(90, { l: { s: 0.82, x: 0.2 }, r: { s: 0.05, x: 0.2 } }, 'in'),
        K(330, { l: { s: 0.82, x: 0.2 }, r: { s: 0.05, x: 0.2 } }), K(560, { s: 1, x: 0 }, 'out')
      ]
    }
  },
  // выключено — сонная: глаза закрыты, иногда приоткрываются
  stopped: {
    rest: 0,
    interval: [10000, 24000],
    weights: { peek: 55, peekGlance: 30, sigh: 15 },
    anims: {
      peek: [
        K(0, { s: 0, x: 0 }), K(650, { s: 0.42, x: 0, y: 0.2 }, 'out'), K(1500, { s: 0.42, x: 0.25, y: 0.2 }),
        K(1650, { s: 0.3, x: 0.25, y: 0.3 }), K(2400, { s: 0, x: 0 }, 'in')
      ],
      peekGlance: [
        K(0, { s: 0, x: 0 }), K(550, { s: 0.48, x: 0, y: 0.1 }, 'out'), K(800, { s: 0.48, x: -0.8, y: 0.1 }, 'out'),
        K(1350, { s: 0.46, x: -0.8 }), K(1600, { s: 0.46, x: 0.8 }, 'out'), K(2100, { s: 0.42, x: 0.8 }),
        K(2350, { s: 0.36, x: 0 }), K(3200, { s: 0, x: 0 }, 'in')
      ],
      sigh: [
        K(0, { s: 0, y: 0 }), K(900, { s: 0, y: -0.9 }), K(1300, { s: 0, y: -0.9 }), K(2400, { s: 0, y: 0.25 }), K(3000, { s: 0, y: 0 })
      ]
    }
  },
  // ошибка — нервная (паузы короче)
  error: {
    rest: 1,
    interval: [5000, 12000],
    weights: { lookAround: 45, nervousBlink: 30, worried: 15, glance: 10 },
    anims: {
      lookAround: [
        K(0, { s: 1, x: 0 }), K(110, { s: 1, x: -1, y: -0.2 }, 'out'), K(420, { s: 1, x: -1, y: -0.2 }),
        K(540, { s: 1, x: 1, y: 0 }, 'out'), K(860, { s: 1, x: 1 }), K(970, { s: 1, x: -0.7, y: 0.3 }, 'out'),
        K(1200, { s: 1, x: -0.7, y: 0.3 }), K(1320, { s: 1, x: 0.6 }, 'out'), K(1500, { s: 1, x: 0.6 }),
        K(1650, { s: 1, x: 0, y: 0 }, 'out')
      ],
      nervousBlink: [...blink(0, 0.1), ...blink(300, -0.1), ...blink(600, 0.1), K(900, { s: 1, x: 0 })],
      worried: [
        K(0, { s: 1, x: 0 }), K(250, { s: 0.62, x: -0.6 }, 'out'), K(650, { s: 0.62, x: -0.6 }),
        K(850, { s: 0.62, x: 0.6 }, 'out'), K(1250, { s: 0.62, x: 0.6 }), K(1500, { s: 1, x: 0 }, 'out')
      ],
      glance: [K(0, { s: 1, x: 0 }), K(110, { s: 1, x: 1 }, 'out'), K(600, { s: 1, x: 1 }), K(720, { s: 1, x: 0 }, 'out')]
    }
  }
}

// Засыпание после долгого простоя (только «подключено»): глаза медленно слипаются и
// остаются полуприкрытыми (fill: forwards), пока пользователь не пошевелит мышью или не нажмёт клавишу.
export const DOZE_AFTER_MS = [3 * 60 * 1000, 5 * 60 * 1000]
const DOZED = { s: 0.22, y: 0.55 }
export const DOZE_OFF = [
  K(0, { s: 1, x: 0, y: 0 }), K(1300, { s: 0.6, y: 0.3 }), K(1800, { s: 0.6, y: 0.3 }),
  K(2000, { s: 0.9, y: 0 }, 'out'), K(2300, { s: 0.62, y: 0.3 }), K(3800, { s: 0.34, y: 0.5 }),
  K(4400, { s: 0.34, y: 0.5 }), K(5600, DOZED)
]
export const WAKE_UP = [
  K(0, DOZED), K(120, { s: 1.06, x: -0.4, y: 0 }, 'out'), K(350, { s: 1, x: 0.35 }, 'out'), K(600, { s: 1, x: 0 }, 'out')
]

// 'connecting' — своя CSS-анимация.
export function idleStateFor(status) {
  if (status === 'connected') return 'connected'
  if (status === 'error') return 'error'
  if (status === 'stopped' || !status) return 'stopped'
  return null
}

export function idleDuration(keys) {
  return keys[keys.length - 1].ms
}

export function randomBetween([min, max], rnd = Math.random) {
  return min + rnd() * (max - min)
}

// Взвешенный случайный выбор; редкие действия (вес < 20) не повторяются подряд.
export function pickIdleAnim(weights, prev = null, rnd = Math.random) {
  const entries = Object.entries(weights).filter(([name, w]) => !(name === prev && w < 20))
  const total = entries.reduce((sum, [, w]) => sum + w, 0)
  let r = rnd() * total
  for (const [name, w] of entries) {
    r -= w
    if (r <= 0) return name
  }
  return entries[entries.length - 1][0]
}

const clamp01 = (v) => Math.max(0, Math.min(1, v))
const px = (v) => `${Number(v.toFixed(2))}px`

// Ключи → кадры Web Animations API для глаза (масштаб по вертикали и сдвиг), группы зрачка
// (взгляд) и линии закрытого глаза (у сонной Norka открытый глаз проявляется поверх неё).
export function idleKeyframes(keys, side, sleepy) {
  const duration = idleDuration(keys) || 1
  const eye = []
  const look = []
  const closed = []
  keys.forEach((key, i) => {
    const p = { s: sleepy ? 0 : 1, x: 0, y: 0, ...key.pose[side] }
    const offset = key.ms / duration
    const easing = IDLE_EASING[keys[i + 1]?.ease] || 'linear'
    const s = Math.max(0.02, p.s)
    eye.push({
      offset,
      easing,
      transform: `translate(${px(p.x * 3)}, ${px(p.y * 3)}) scaleY(${Number(s.toFixed(3))})`,
      ...(sleepy ? { opacity: Number(clamp01((p.s - 0.04) / 0.1).toFixed(3)) } : {})
    })
    look.push({ offset, easing, transform: `translate(${px(p.x * 13)}, ${px(p.y * 5)})` })
    if (sleepy) {
      closed.push({
        offset,
        easing,
        opacity: Number((1 - clamp01((p.s - 0.02) / 0.12)).toFixed(3)),
        transform: `translateY(${px(p.y * 5)})`
      })
    }
  })
  return { duration, eye, look, closed }
}
