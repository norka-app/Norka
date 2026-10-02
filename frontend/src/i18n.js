import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import ru from './locales/ru.json'

export const LANGUAGE_STORAGE_KEY = 'lt.language'
export const LOCALE_STORAGE_KEY = 'lt.locale'

export function readStoredPreference() {
  if (typeof window === 'undefined') return 'auto'
  const saved = window.localStorage.getItem(LANGUAGE_STORAGE_KEY)
  if (saved === 'ru' || saved === 'en' || saved === 'auto') return saved
  return 'auto'
}

export function guessLocale() {
  if (typeof window === 'undefined') return 'en'
  const preference = readStoredPreference()
  if (preference === 'ru' || preference === 'en') return preference
  const cached = window.localStorage.getItem(LOCALE_STORAGE_KEY)
  if (cached === 'ru' || cached === 'en') return cached
  const nav = String(window.navigator?.language || '').toLowerCase()
  return nav.startsWith('ru') ? 'ru' : 'en'
}

const initialLocale = guessLocale()

if (typeof document !== 'undefined') {
  document.documentElement.lang = initialLocale
}

const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: {
    en: ['ru'],
    ru: ['en'],
  },
  messages: {
    en,
    ru,
  },
})

export default i18n
