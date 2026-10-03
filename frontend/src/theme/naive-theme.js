import { darkTheme } from 'naive-ui'

const fontFamily = '"Segoe UI Variable Text", "Segoe UI Variable", "Segoe UI", system-ui, sans-serif'

function commonOverrides(primary, hover, pressed) {
  return {
    primaryColor: primary,
    primaryColorHover: hover,
    primaryColorPressed: pressed,
    primaryColorSuppl: primary,
    borderRadius: '8px',
    fontFamily,
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
    }
  }
  return {
    common: commonOverrides('#1d4ed8', '#1e40af', '#1e3a8a'),
    Form: formOverrides,
  }
}
