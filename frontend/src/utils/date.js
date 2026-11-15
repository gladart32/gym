// Хелперы локальных дат: «сегодня» и сдвиг на N дней определяет клиент,
// чтобы сервер (UTC) не решал за пользователя в другом часовом поясе.

export function todayStr() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function shiftDays(dateStr, days) {
  const [y, m, d] = dateStr.split('-').map(Number)
  const date = new Date(y, m - 1, d + days)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

// IANA-зона браузера («Europe/Moscow»); '' если определить не удалось
export function browserTimezone() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
}

// Смещение зоны для отображения («GMT+3»); '' для неизвестной зоны
export function timezoneOffsetLabel(tz) {
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' }).formatToParts(new Date())
    return parts.find((p) => p.type === 'timeZoneName')?.value.replace('GMT', 'UTC') ?? ''
  } catch {
    return ''
  }
}

// Популярные зоны для выбора в настройках; зону браузера добавляем динамически
export const POPULAR_TIMEZONES = [
  'Europe/Moscow',
  'Europe/Kaliningrad',
  'Europe/Minsk',
  'Europe/Kyiv',
  'Europe/Istanbul',
  'Asia/Almaty',
  'Asia/Tbilisi',
  'Asia/Yerevan',
  'Asia/Baku',
  'Asia/Yekaterinburg',
  'Asia/Novosibirsk',
  'Asia/Krasnoyarsk',
  'Asia/Irkutsk',
  'Asia/Vladivostok',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'UTC',
  'America/New_York',
  'America/Chicago',
  'America/Los_Angeles',
  'Australia/Sydney',
]
