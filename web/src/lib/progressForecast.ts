import type { Level } from '@/features/today/levels'
import { addDays, MAX_MINUTES_PER_DAY, neededHours, type Range } from './forecast'

export type PaceForecast =
  | { kind: 'reached' }
  | { kind: 'enough' }
  | { kind: 'collecting'; daysSinceStart: number }
  | { kind: 'idle' }
  | { kind: 'distant' }
  | {
      kind: 'estimate'
      needed: Range
      remaining: Range
      days: Range
      finishBy: { earliest: Date; latest: Date }
      /** На сколько недель короче срок при темпе на 10 минут больше; нет поля, если меньше недели или темп уже нереальный. */
      saveWithTenMore?: { weeks: number }
      unrealistic: boolean
    }

const EXTRA_MINUTES = 10
/** Дальше десяти лет дату не называем: при таком темпе цифра ничего не говорит (сотни месяцев). */
const MAX_FORECAST_DAYS = 3650
const DAYS_PER_WEEK = 7

/**
 * Прогноз по реальному темпу. Нужные часы — диапазон из forecast.ts; набранные в сервисе вычитаются отдельно
 * из каждой границы (ниже нуля не падает). Пока темп не готов (меньше 14 суток занятий), даты нет: честнее
 * сказать «собираю данные».
 */
export function forecastFromPace(
  current: Level,
  target: Level,
  studiedMinutes: number,
  paceMinutesPerDay: number,
  paceReady: boolean,
  daysSinceStart: number,
  today: Date,
): PaceForecast {
  const needed = neededHours(current, target)
  if (!needed) return { kind: 'reached' }
  const studied = studiedMinutes / 60
  if (studied >= needed.max) return { kind: 'enough' }
  if (!paceReady) return { kind: 'collecting', daysSinceStart }
  if (paceMinutesPerDay <= 0) return { kind: 'idle' }

  const remaining = { min: Math.max(0, needed.min - studied), max: needed.max - studied }
  const daysAt = (pace: number): Range => ({ min: (remaining.min * 60) / pace, max: (remaining.max * 60) / pace })
  const days = daysAt(paceMinutesPerDay)
  if (days.max > MAX_FORECAST_DAYS) return { kind: 'distant' }
  const unrealistic = paceMinutesPerDay > MAX_MINUTES_PER_DAY

  let saveWithTenMore: { weeks: number } | undefined
  if (!unrealistic) {
    const saved = days.max - daysAt(paceMinutesPerDay + EXTRA_MINUTES).max
    if (saved >= DAYS_PER_WEEK) saveWithTenMore = { weeks: Math.round(saved / DAYS_PER_WEEK) }
  }
  return {
    kind: 'estimate',
    needed,
    remaining,
    days,
    finishBy: { earliest: addDays(today, Math.ceil(days.min)), latest: addDays(today, Math.ceil(days.max)) },
    ...(saveWithTenMore ? { saveWithTenMore } : {}),
    unrealistic,
  }
}
