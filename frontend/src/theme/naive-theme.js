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

export function naiveThemeFor(mode) {
  return mode === 'dark' ? darkTheme : null
}

export function naiveThemeOverrides(mode) {
  if (mode === 'dark') {
    return {
      common: commonOverrides('#7dd3fc', '#bae6fd', '#38bdf8'),
    }
  }
  return {
    common: commonOverrides('#1d4ed8', '#1e40af', '#1e3a8a'),
  }
}
