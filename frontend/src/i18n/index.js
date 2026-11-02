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

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale,
  messages: { ru, en },
})

export function setLocale(locale) {
  if (!supportedLocales.includes(locale)) return
  i18n.global.locale.value = locale
  storedLocale.value = locale
  document.documentElement.setAttribute('lang', locale)
}

document.documentElement.setAttribute('lang', i18n.global.locale.value)
