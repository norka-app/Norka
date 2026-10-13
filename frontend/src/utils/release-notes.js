// Заметки выпуска лежат одним файлом: русский текст, маркер, английский.
// Окно «Что нового» показывает часть по языку интерфейса.
// На странице выпуска GitHub маркер — HTML-комментарий, видны оба языка.

export const RELEASE_NOTES_EN_MARKER = '<!-- norka:en -->'

const LANG_HEADING = {
  ru: '## Русский',
  en: '## English',
}

function stripLangHeading(text, heading) {
  const trimmed = String(text || '').replace(/^\uFEFF/, '').trim()
  const lines = trimmed.split('\n')
  if (lines[0]?.trim() !== heading) return trimmed
  return lines.slice(1).join('\n').trim()
}

export function notesForLocale(notes, locale) {
  const raw = String(notes || '').replace(/^\uFEFF/, '')
  const cut = raw.indexOf(RELEASE_NOTES_EN_MARKER)
  if (cut < 0) return raw.trim()
  const russian = stripLangHeading(raw.slice(0, cut), LANG_HEADING.ru)
  const english = stripLangHeading(raw.slice(cut + RELEASE_NOTES_EN_MARKER.length), LANG_HEADING.en)
  if (locale === 'en') return english || russian
  return russian || english
}
