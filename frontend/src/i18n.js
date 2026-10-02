import { createI18n } from 'vue-i18n'
import ru from './locales/ru.json'

const i18n = createI18n({
    legacy: false,
    locale: 'ru',
    fallbackLocale: 'ru',
    messages: {
        ru
    }
})

export default i18n
