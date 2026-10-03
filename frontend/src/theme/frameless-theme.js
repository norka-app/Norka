// Токены темы Naive UI для вида «окно без рамки».
// Подключаются только когда процесс открылся с этим флагом.
// Раскладка и отступы остаются в styles/frameless.css.

const LIGHT = {
  hover: 'rgba(24, 24, 27, 0.045)',
  muted: '#71717a',
  pop: 'rgba(252, 252, 253, 0.94)',
  ok: '#16a34a',
  busy: '#d97706',
  err: '#dc2626',
  off: 'rgba(24, 24, 27, 0.055)',
  offInk: '#71717a',
}

const DARK = {
  hover: 'rgba(255, 255, 255, 0.055)',
  muted: '#a1a1aa',
  pop: 'rgba(44, 44, 52, 0.95)',
  ok: '#4ade80',
  busy: '#fbbf24',
  err: '#f87171',
  off: 'rgba(255, 255, 255, 0.07)',
  offInk: '#a1a1aa',
}

function tagFill(hex, alpha) {
  const value = hex.replace('#', '')
  const r = parseInt(value.slice(0, 2), 16)
  const g = parseInt(value.slice(2, 4), 16)
  const b = parseInt(value.slice(4, 6), 16)
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export function framelessNaiveOverrides(mode) {
  const tone = mode === 'dark' ? DARK : LIGHT
  return {
    Card: {
      color: 'transparent',
      colorModal: tone.pop,
      borderColor: 'transparent',
      boxShadow: 'none',
    },
    DataTable: {
      thColor: 'transparent',
      tdColor: 'transparent',
      thColorHover: 'transparent',
      tdColorHover: tone.hover,
      borderColor: 'transparent',
      thTextColor: tone.muted,
      thFontWeight: '500',
    },
    Tag: {
      borderRadius: '999px',
      border: 'none',
      borderSuccess: 'none',
      borderWarning: 'none',
      borderError: 'none',
      borderInfo: 'none',
      color: tone.off,
      textColor: tone.offInk,
      colorSuccess: tagFill(tone.ok, 0.14),
      textColorSuccess: tone.ok,
      colorWarning: tagFill(tone.busy, 0.16),
      textColorWarning: tone.busy,
      colorError: tagFill(tone.err, 0.14),
      textColorError: tone.err,
    },
    Collapse: {
      dividerColor: 'transparent',
      titleFontWeight: '600',
    },
    Modal: {
      color: tone.pop,
      boxShadow: 'none',
    },
  }
}

export function mergeNaiveTheme(base, extra) {
  if (!extra) return base
  const merged = { ...base }
  for (const key of Object.keys(extra)) {
    merged[key] = { ...(base?.[key] || {}), ...extra[key] }
  }
  return merged
}
