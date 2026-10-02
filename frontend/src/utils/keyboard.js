// Раскладко-независимые клавиши. event.code — физическая клавиша (KeyK остаётся
// KeyK, когда event.key уже «л»). Если WebView2 отдал пустой code, буква ЙЦУКЕН
// переводится обратно в латинскую, чтобы Ctrl+K и захват хоткея не сохраняли «л».

const NAMED_KEYS = {
  ' ': 'space',
  Spacebar: 'space',
  Escape: 'escape',
  Esc: 'escape',
  Tab: 'tab',
  Enter: 'enter',
  ArrowDown: 'arrowdown',
  ArrowUp: 'arrowup',
  ArrowLeft: 'arrowleft',
  ArrowRight: 'arrowright',
  Home: 'home',
  End: 'end',
  PageUp: 'pageup',
  PageDown: 'pagedown',
  Shift: 'shift',
  Control: 'control',
  Alt: 'alt',
  Meta: 'meta',
  OS: 'meta'
}

// Буквы ЙЦУКЕН, у которых есть латинская пара. Знаки (х, ъ, ж, э, б, ю, ё) не
// входят: бэкенд их как горячую клавишу не принимает.
const RU_TO_LATIN = {
  й: 'q',
  ц: 'w',
  у: 'e',
  к: 'r',
  е: 't',
  н: 'y',
  г: 'u',
  ш: 'i',
  щ: 'o',
  з: 'p',
  ф: 'a',
  ы: 's',
  в: 'd',
  а: 'f',
  п: 'g',
  р: 'h',
  о: 'j',
  л: 'k',
  д: 'l',
  я: 'z',
  ч: 'x',
  с: 'c',
  м: 'v',
  и: 'b',
  т: 'n',
  ь: 'm'
}

const MODIFIER_KEYS = new Set(['shift', 'control', 'ctrl', 'alt', 'option', 'meta', 'os', 'super', 'win', 'cmd', 'command'])
const CHORD_KEYS = new Set(['space', 'escape', 'tab', 'enter'])

function keyFromCode(code) {
  const value = String(code || '')
  if (/^Key[A-Z]$/.test(value)) return value.slice(3).toLowerCase()
  if (/^Digit[0-9]$/.test(value)) return value.slice(5)
  if (/^F(?:[1-9]|1[0-2])$/.test(value)) return value.toLowerCase()
  switch (value) {
    case 'Space':
      return 'space'
    case 'Escape':
      return 'escape'
    case 'Tab':
      return 'tab'
    case 'Enter':
    case 'NumpadEnter':
      return 'enter'
    case 'ArrowDown':
      return 'arrowdown'
    case 'ArrowUp':
      return 'arrowup'
    case 'ArrowLeft':
      return 'arrowleft'
    case 'ArrowRight':
      return 'arrowright'
    case 'Home':
      return 'home'
    case 'End':
      return 'end'
    case 'PageUp':
      return 'pageup'
    case 'PageDown':
      return 'pagedown'
    case 'ShiftLeft':
    case 'ShiftRight':
      return 'shift'
    case 'ControlLeft':
    case 'ControlRight':
      return 'control'
    case 'AltLeft':
    case 'AltRight':
      return 'alt'
    case 'MetaLeft':
    case 'MetaRight':
      return 'meta'
    default:
      return ''
  }
}

function keyFromKey(key) {
  const value = String(key || '')
  if (!value) return ''
  if (Object.prototype.hasOwnProperty.call(NAMED_KEYS, value)) return NAMED_KEYS[value]
  const lower = value.toLowerCase()
  if (RU_TO_LATIN[lower]) return RU_TO_LATIN[lower]
  if (lower.length === 1 && ((lower >= 'a' && lower <= 'z') || (lower >= '0' && lower <= '9'))) return lower
  if (/^f(?:[1-9]|1[0-2])$/.test(lower)) return lower
  return lower
}

export function physicalKey(event) {
  const code = String(event?.code || '')
  if (code && code !== 'Unidentified') {
    const fromCode = keyFromCode(code)
    if (fromCode) return fromCode
  }
  return keyFromKey(event?.key)
}

export function matchesKey(event, name) {
  return physicalKey(event) === String(name || '').toLowerCase()
}

export function isQuickSearchChord(event) {
  if (!event || !(event.ctrlKey || event.metaKey) || event.altKey || event.shiftKey) return false
  return matchesKey(event, 'k')
}

function isChordKey(key) {
  if (CHORD_KEYS.has(key)) return true
  if (key.length === 1 && ((key >= 'a' && key <= 'z') || (key >= '0' && key <= '9'))) return true
  return /^f(?:[1-9]|1[0-2])$/.test(key)
}

// hotkeyChord records a settings chord. altName is "option" on macOS.
export function hotkeyChord(event, options = {}) {
  if (!event) return ''
  const parts = []
  if (event.ctrlKey) parts.push('ctrl')
  if (event.altKey) parts.push(options.altName || 'alt')
  if (event.shiftKey) parts.push('shift')
  if (event.metaKey) parts.push('meta')
  const key = physicalKey(event)
  if (!key || MODIFIER_KEYS.has(key) || !isChordKey(key)) return ''
  parts.push(key)
  if (parts.length < 2) return ''
  return parts.join('+')
}
