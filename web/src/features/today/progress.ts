import { useQuery } from '@tanstack/react-query'
import { getProgressToday, type ProgressToday } from '@/api/client'

export type { ProgressToday } from '@/api/client'

export const progressKey = ['progress', 'today'] as const

/** Прогресс за сегодня. Меняется после каждого ответа, поэтому данные всегда перечитываются. */
export function useProgressToday() {
  return useQuery({ queryKey: progressKey, queryFn: getProgressToday, retry: false, staleTime: 0, refetchOnWindowFocus: false })
}

/** Подпись кнопки сессии по прогрессу. «Выполнен» решает сервер (doneToday): округлённые минуты на границе врут. */
export function sessionButtonLabel(p: Pick<ProgressToday, 'minutes' | 'doneToday'> | undefined): 'Начать сессию' | 'Продолжить сессию' | 'Позаниматься ещё' {
  if (!p) return 'Начать сессию'
  if (p.doneToday) return 'Позаниматься ещё'
  return p.minutes > 0 ? 'Продолжить сессию' : 'Начать сессию'
}

/** Минуты для показа: целые без дроби, доля через запятую («4,3»). */
export function formatMinutes(minutes: number): string {
  return minutes.toLocaleString('ru-RU', { maximumFractionDigits: 1 })
}

/** «день / дня / дней» для серии. */
export function streakWord(n: number): 'день' | 'дня' | 'дней' {
  const mod100 = n % 100
  if (mod100 >= 11 && mod100 <= 14) return 'дней'
  const mod10 = n % 10
  if (mod10 === 1) return 'день'
  if (mod10 >= 2 && mod10 <= 4) return 'дня'
  return 'дней'
}

const MONTHS = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']

/** «24 октября» из «2026-10-24»; неверная строка показывается как есть. */
export function formatDayRu(iso: string): string {
  const m = /^\d{4}-(\d{2})-(\d{2})$/.exec(iso)
  if (!m) return iso
  const month = MONTHS[Number(m[1]) - 1]
  return month ? `${Number(m[2])} ${month}` : iso
}
