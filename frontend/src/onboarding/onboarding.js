// Шаги первого тура и решение, когда его показывать.
// Копирайт флагов (команда SSH, быстрый поиск) отфильтровывается здесь,
// чтобы компонент только рисовал готовый список.
import { matchesKey } from '../utils/keyboard.js'

const ADD_SSH = 'onboarding.addSsh'
const TRAY_QUICK = 'onboarding.trayQuick'

function flagOn(flags, id) {
  return !!flags?.[id]
}

export function onboardingSteps(flags = {}) {
  return [
    {
      id: 'welcome',
      target: 'mascot',
      page: '',
      titleKey: 'onboarding.welcomeTitle',
      bodyKeys: ['onboarding.welcomeBody'],
      showMascot: true,
    },
    {
      id: 'add-tunnel',
      target: 'add-tunnel',
      page: 'tunnels',
      titleKey: 'onboarding.addTitle',
      bodyKeys: ['onboarding.addBody'].concat(flagOn(flags, 'ssh_command') ? [ADD_SSH] : []),
    },
    {
      id: 'eyes',
      target: 'mascot',
      page: '',
      titleKey: 'onboarding.eyesTitle',
      bodyKeys: ['onboarding.eyesBody'],
      legend: [
        { status: 'connected', labelKey: 'onboarding.eyesGreen' },
        { status: 'error', labelKey: 'onboarding.eyesRed' },
        { status: 'stopped', labelKey: 'onboarding.eyesGrey' },
      ],
    },
    {
      id: 'tray',
      target: '',
      page: '',
      titleKey: 'onboarding.trayTitle',
      bodyKeys: ['onboarding.trayBody'].concat(flagOn(flags, 'quick_search') ? [TRAY_QUICK] : []),
    },
    {
      id: 'features',
      target: 'features',
      page: 'config',
      titleKey: 'onboarding.featuresTitle',
      bodyKeys: ['onboarding.featuresBody'],
    },
  ].filter((step) => step.bodyKeys.length > 0)
}

// Показ при первом запуске: флаг включён, список туннелей уже загружен и пуст,
// тур не завершён и не пропущен. Повтор из настроек игнорирует список туннелей.
export function shouldShowOnboarding({ enabled, ready, tunnelCount, done, replay } = {}) {
  if (!enabled) return false
  if (replay) return true
  if (!ready) return false
  return Number(tunnelCount) === 0 && !done
}

export function reduceOnboarding(state, action) {
  const count = Math.max(1, Number(state?.count) || 1)
  const index = Math.min(Math.max(0, Number(state?.index) || 0), count - 1)
  const next = {
    index,
    count,
    open: state?.open !== false,
    done: !!state?.done,
    enabled: state?.enabled !== false,
    replay: !!state?.replay,
  }
  if (action === 'next') {
    if (index >= count - 1) return { ...next, open: false, done: true }
    return { ...next, index: index + 1, open: true, done: false }
  }
  if (action === 'back') return { ...next, index: Math.max(0, index - 1), open: true }
  if (action === 'skip' || action === 'done') return { ...next, open: false, done: true }
  if (action === 'restart') {
    if (!next.enabled) return { ...next, open: false, replay: false }
    return { ...next, open: true, done: false, index: 0, replay: true }
  }
  return next
}

// Стрелки и Esc — по физической клавише (event.code), не по букве раскладки.
export function onboardingKeyAction(event, { index = 0 } = {}) {
  if (!event) return ''
  if (matchesKey(event, 'escape')) return 'skip'
  if (matchesKey(event, 'arrowright')) return 'next'
  if (matchesKey(event, 'arrowleft')) return index > 0 ? 'back' : ''
  return ''
}

export function visibleTargetRect(rect, viewport, min = 8) {
  if (!rect || rect.hidden) return null
  const width = Number(rect.width) || 0
  const height = Number(rect.height) || 0
  if (width < 2 || height < 2) return null
  const left = Number(rect.left) || 0
  const top = Number(rect.top) || 0
  const right = left + width
  const bottom = top + height
  const viewW = Number(viewport?.width) || 0
  const viewH = Number(viewport?.height) || 0
  const visibleW = Math.min(right, viewW) - Math.max(left, 0)
  const visibleH = Math.min(bottom, viewH) - Math.max(top, 0)
  if (visibleW < min || visibleH < min) return null
  return { left, top, right, bottom, width, height }
}

export function spotlightBox(rect, viewport, pad = 6) {
  const visible = visibleTargetRect(rect, viewport)
  if (!visible) return null
  const viewW = Number(viewport?.width) || 0
  const viewH = Number(viewport?.height) || 0
  const left = Math.max(0, visible.left - pad)
  const top = Math.max(0, visible.top - pad)
  const right = Math.min(viewW, visible.right + pad)
  const bottom = Math.min(viewH, visible.bottom + pad)
  const width = right - left
  const height = bottom - top
  if (width < 8 || height < 8) return null
  return { left, top, right, bottom, width, height }
}

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), Math.max(min, max))
}

// Карточка рядом с целью. Если цель не видна — по центру и без подсветки.
export function placeOnboardingCard({ target, card, viewport, margin = 12, gap = 12 } = {}) {
  const vw = Number(viewport?.width) || 0
  const vh = Number(viewport?.height) || 0
  const cw = Math.min(Number(card?.width) || 0, Math.max(0, vw - margin * 2))
  const ch = Math.min(Number(card?.height) || 0, Math.max(0, vh - margin * 2))
  const centered = {
    spotlight: false,
    top: clamp((vh - ch) / 2, margin, vh - ch - margin),
    left: clamp((vw - cw) / 2, margin, vw - cw - margin),
  }
  if (!target) return centered
  const candidates = [
    { top: target.bottom + gap, left: target.left },
    { top: target.bottom + gap, left: target.right - cw },
    { top: target.top - gap - ch, left: target.left },
    { top: target.top - gap - ch, left: target.right - cw },
    { top: target.top, left: target.right + gap },
    { top: target.bottom - ch, left: target.right + gap },
    { top: target.top, left: target.left - gap - cw },
    { top: target.bottom - ch, left: target.left - gap - cw },
  ]
  const fits = (pos) => pos.top >= margin - 0.5
    && pos.left >= margin - 0.5
    && pos.top + ch <= vh - margin + 0.5
    && pos.left + cw <= vw - margin + 0.5
  for (const pos of candidates) {
    if (fits(pos)) return { spotlight: true, top: pos.top, left: pos.left }
  }
  return {
    spotlight: true,
    top: clamp(target.bottom + gap, margin, vh - ch - margin),
    left: clamp(target.left, margin, vw - cw - margin),
  }
}
