import { LEVELS, type Level } from '@/features/today/levels'

export type Range = { min: number; max: number }

export type ForwardForecast =
  | { kind: 'reached' }
  | { kind: 'estimate'; hours: Range; days: Range; finishBy: { earliest: Date; latest: Date } }

export type BackwardForecast =
  | { kind: 'reached' }
  | { kind: 'past' }
  | { kind: 'needed'; minutesPerDay: Range; unrealistic: boolean }

/**
 * Часы занятий на переход с уровня на следующий (индекс — уровень-источник). Это разности середин
 * накопленных часов Cambridge English (guided learning hours): A1 90–100, A2 180–200, B1 350–400,
 * B2 500–600, C1 700–800. Часы рассчитаны на занятия с преподавателем, поэтому есть поправка ниже.
 */
const TRANSITION_HOURS: Record<Exclude<Level, 'C1'>, number> = { A1: 95, A2: 185, B1: 175, B2: 200 }

/** Допущение (не данные Cambridge): самостоятельные занятия менее эффективны, нужно в 1,5–2 раза больше часов. */
const SELF_STUDY_FACTOR = { min: 1.5, max: 2.0 }

/** Больше этого в день заниматься нереально. Совпадает с верхней границей настроек. */
const MAX_MINUTES_PER_DAY = 240

const DAYS_PER_MONTH = 30.4
const DAYS_PER_WEEK = 7
const MS_PER_DAY = 86_400_000

/** Часы чистых занятий для перехода; 0, если цель не выше уровня. */
function baseHours(current: Level, target: Level): number {
  let total = 0
  for (let i = LEVELS.indexOf(current); i < LEVELS.indexOf(target); i++) {
    total += TRANSITION_HOURS[LEVELS[i] as Exclude<Level, 'C1'>]
  }
  return total
}

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

function addDays(d: Date, n: number): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate() + n)
}

export function forecastForward(
  current: Level,
  target: Level,
  minutesPerDay: number,
  today: Date,
): ForwardForecast {
  const base = baseHours(current, target)
  if (base === 0) return { kind: 'reached' }
  const hours = { min: base * SELF_STUDY_FACTOR.min, max: base * SELF_STUDY_FACTOR.max }
  const days = { min: (hours.min * 60) / minutesPerDay, max: (hours.max * 60) / minutesPerDay }
  return {
    kind: 'estimate',
    hours,
    days,
    finishBy: {
      earliest: addDays(today, Math.ceil(days.min)),
      latest: addDays(today, Math.ceil(days.max)),
    },
  }
}

export function forecastBackward(
  current: Level,
  target: Level,
  targetDate: Date,
  today: Date,
): BackwardForecast {
  const base = baseHours(current, target)
  if (base === 0) return { kind: 'reached' }
  const daysUntil = Math.round(
    (startOfDay(targetDate).getTime() - startOfDay(today).getTime()) / MS_PER_DAY,
  )
  if (daysUntil <= 0) return { kind: 'past' }
  const minutesPerDay = {
    min: (base * SELF_STUDY_FACTOR.min * 60) / daysUntil,
    max: (base * SELF_STUDY_FACTOR.max * 60) / daysUntil,
  }
  // Нереалистично, если даже оптимистичная граница выше предела.
  return { kind: 'needed', minutesPerDay, unrealistic: minutesPerDay.min > MAX_MINUTES_PER_DAY }
}

type Forms = { one: string; few: string; many: string }

const MONTH: Forms = { one: 'месяц', few: 'месяца', many: 'месяцев' }
const WEEK: Forms = { one: 'неделя', few: 'недели', many: 'недель' }

/** Форма после числа: 1 месяц, 2 месяца, 5 месяцев, 11 месяцев, 21 месяц. */
function plural(n: number, f: Forms): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return f.one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return f.few
  return f.many
}

/** После «около»: 1 месяца, 2 месяцев, 21 месяца. */
function approximate(n: number, genitive: { one: string; many: string }): string {
  const one = n % 10 === 1 && n % 100 !== 11
  return `около ${n} ${one ? genitive.one : genitive.many}`
}

/** «6–8 месяцев», «2–4 недели» или «около 2 месяцев», если границы после округления совпали. */
export function describeDuration(days: Range): string {
  if (days.max < DAYS_PER_MONTH) {
    const lo = Math.max(1, Math.round(days.min / DAYS_PER_WEEK))
    const hi = Math.max(1, Math.round(days.max / DAYS_PER_WEEK))
    if (lo === hi) return approximate(hi, { one: 'недели', many: 'недель' })
    return `${lo}–${hi} ${plural(hi, WEEK)}`
  }
  const lo = Math.max(1, Math.round(days.min / DAYS_PER_MONTH))
  const hi = Math.max(1, Math.round(days.max / DAYS_PER_MONTH))
  if (lo === hi) return approximate(hi, { one: 'месяца', many: 'месяцев' })
  return `${lo}–${hi} ${plural(hi, MONTH)}`
}

/** Разбирает YYYY-MM-DD как местный день (new Date('2027-01-01') дал бы UTC и сдвиг на сутки). */
export function parseDay(s: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s)
  if (!m) return null
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
}
