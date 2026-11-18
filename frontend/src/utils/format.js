// Форматирование длительности в секундах как MM:SS или H:MM:SS.
export function formatClock(totalSeconds) {
  const s = Math.max(0, Math.floor(Number(totalSeconds) || 0))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const mm = String(m).padStart(2, '0')
  const ss = String(sec).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`
}

// Тоннаж (Σ вес×повторы): до 10 т — в килограммах («1 700 кг»),
// от 10 т — в тоннах с одним знаком («12,5 т»).
export function formatTonnage(kg, locale = 'ru', kgLabel = 'кг', tLabel = 'т') {
  const value = Number(kg) || 0
  if (value >= 10000) {
    return `${(value / 1000).toLocaleString(locale, { maximumFractionDigits: 1 })} ${tLabel}`
  }
  return `${Math.round(value).toLocaleString(locale)} ${kgLabel}`
}
