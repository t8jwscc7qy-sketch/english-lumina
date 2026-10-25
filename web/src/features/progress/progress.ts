import { useQuery } from '@tanstack/react-query'
import { getProgressOverview } from '@/api/client'

export type { ProgressOverview } from '@/api/client'

export const overviewKey = ['progress', 'overview'] as const

/** Обзор прогресса. Страница открывается после занятий, поэтому данные всегда перечитываются. */
export function useProgressOverview() {
  return useQuery({ queryKey: overviewKey, queryFn: getProgressOverview, retry: false, staleTime: 0, refetchOnWindowFocus: false })
}

/** «14 ч 20 мин», «1 ч», «45 мин», «0 мин»: округление до минуты. */
export function formatHoursMinutes(minutes: number): string {
  const total = Math.round(minutes)
  const h = Math.floor(total / 60)
  const m = total % 60
  if (h === 0) return `${m} мин`
  return m === 0 ? `${h} ч` : `${h} ч ${m} мин`
}

/** Диапазон часов: «280–370 ч»; если границы после округления совпали — «100 ч». */
export function formatRange(minHours: number, maxHours: number): string {
  const lo = Math.round(minHours)
  const hi = Math.round(maxHours)
  return lo === hi ? `${hi} ч` : `${lo}–${hi} ч`
}

const MONTHS_GEN = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']
const MONTHS_SHORT = ['янв', 'фев', 'мар', 'апр', 'мая', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']

/** «12 марта»; год добавляется, только если он не совпадает с годом `today`: «12 марта 2027». */
export function formatFinishDate(d: Date, today: Date): string {
  const base = `${d.getDate()} ${MONTHS_GEN[d.getMonth()]}`
  return d.getFullYear() === today.getFullYear() ? base : `${base} ${d.getFullYear()}`
}

/** «28 июл» из «2026-07-28»; неверная строка показывается как есть. */
export function formatShortDay(iso: string): string {
  const m = /^\d{4}-(\d{2})-(\d{2})$/.exec(iso)
  const month = m ? MONTHS_SHORT[Number(m[1]) - 1] : undefined
  return m && month ? `${Number(m[2])} ${month}` : iso
}
