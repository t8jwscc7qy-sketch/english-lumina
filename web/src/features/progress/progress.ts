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
