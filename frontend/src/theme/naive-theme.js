import { darkTheme } from 'naive-ui'

// Один стек на всё приложение: Windows берёт Segoe, превью и Linux — Selawik
// (метрически близкий к Segoe UI). Тот же список в style.css (--norka-font-family).
export const APP_FONT_FAMILY =
  '"Segoe UI Variable Text", "Segoe UI Variable", "Segoe UI", Selawik, system-ui, sans-serif'

// Шапка и подвал модалки: 16 по вертикали, 24 по горизонтали. Зазор кнопок — 8 (form-layout).
const cardOverrides = {
  paddingMedium: '16px 24px 16px',
}

function commonOverrides(primary, hover, pressed) {
  return {
    primaryColor: primary,
    primaryColorHover: hover,
    primaryColorPressed: pressed,
    primaryColorSuppl: primary,
    borderRadius: '8px',
    fontFamily: APP_FONT_FAMILY,
  }
}

/** One label rhythm for every n-form (see form-layout.js for grid gaps). */
const formOverrides = {
  labelFontWeight: '600',
  labelPaddingVertical: '0 0 6px 0',
  feedbackPadding: '4px 0 0 0',
}

export function naiveThemeFor(mode) {
  return mode === 'dark' ? darkTheme : null
}

export function naiveThemeOverrides(mode) {
  // те же цвета, что --lt-brand / --lt-brand-hover / --lt-brand-pressed в style.css
  if (mode === 'dark') {
    return {
      common: commonOverrides('#7dd3fc', '#bae6fd', '#38bdf8'),
      Form: formOverrides,
      Card: cardOverrides,
    }
  }
  return {
    common: commonOverrides('#1d4ed8', '#1e40af', '#1e3a8a'),
    Form: formOverrides,
    Card: cardOverrides,
  }
}
