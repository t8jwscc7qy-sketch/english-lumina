const MINUTE = 60
const HOUR = 3600
const DAY = 86_400
const DAYS_PER_MONTH = 30.4

/**
 * Через сколько вернётся карточка, коротко: «10 мин», «3 дн», «1 мес.». Округление идёт по ступеням,
 * поэтому не бывает «60 мин» (это уже «1 ч») и «24 ч» (это «1 дн»).
 */
export function formatInterval(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < MINUTE) return 'меньше минуты'
  const minutes = Math.round(seconds / MINUTE)
  if (minutes < 60) return `${minutes} мин`
  const hours = Math.round(seconds / HOUR)
  if (hours < 24) return `${hours} ч`
  const days = Math.round(seconds / DAY)
  if (days < 30) return `${days} дн`
  const months = Math.round(days / DAYS_PER_MONTH)
  if (months < 12) return `${months} мес.`
  return `${Math.round(days / 365)} г.`
}
