import { useStorage } from '@vueuse/core'
import { createI18n } from 'vue-i18n'

import en from '@/locales/en.json'
import ru from '@/locales/ru.json'

export const supportedLocales = ['ru', 'en']
export const fallbackLocale = 'ru'

const storedLocale = useStorage('gym-locale', '')
const browserLocale = navigator.language?.split('-')[0] ?? ''

function detectLocale() {
  if (supportedLocales.includes(storedLocale.value)) return storedLocale.value
  if (supportedLocales.includes(browserLocale)) return browserLocale
  return fallbackLocale
}

// Славянская плюрализация для ru: «1 день / 2 дня / 5 дней».
// en использует встроенное правило vue-i18n (one/other).
function ruPluralRules(choice, choicesLength) {
  const n = Math.abs(choice)
  if (choicesLength < 3) return n === 1 ? 0 : 1
  const teen = n % 100 >= 11 && n % 100 <= 14
  if (n % 10 === 1 && !teen) return 0
  if (!teen && n % 10 >= 2 && n % 10 <= 4) return 1
  return 2
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale,
  messages: { ru, en },
  pluralRules: { ru: ruPluralRules },
})

export function setLocale(locale) {
  if (!supportedLocales.includes(locale)) return
  i18n.global.locale.value = locale
  storedLocale.value = locale
  document.documentElement.setAttribute('lang', locale)
}

document.documentElement.setAttribute('lang', i18n.global.locale.value)
