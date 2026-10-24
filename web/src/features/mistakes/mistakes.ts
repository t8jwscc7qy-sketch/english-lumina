import { useQuery } from '@tanstack/react-query'
import { getMistakes } from '@/api/client'

export type { Mistake, MistakeList } from '@/api/client'

export const mistakesKey = ['mistakes'] as const

/** Банк ошибок. Страница открывается после упражнений, поэтому данные всегда перечитываются. */
export function useMistakes() {
  return useQuery({ queryKey: mistakesKey, queryFn: getMistakes, retry: false, staleTime: 0, refetchOnWindowFocus: false })
}

const DAY = 24 * 3600_000

/** Номер локального календарного дня: разница номеров не зависит от перехода на летнее время. */
function dayNumber(d: Date): number {
  return Math.floor(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / DAY)
}

/** Когда ошибка вернётся, по локальным календарным дням: прошлое и неверная дата — «ждёт повтора», тот же день — «сегодня», следующий — «завтра», дальше — «через N дн.». */
export function formatDue(nextDue: string, now: Date): string {
  const due = new Date(nextDue)
  if (!(due.getTime() > now.getTime())) return 'ждёт повтора'
  const days = dayNumber(due) - dayNumber(now)
  if (days <= 0) return 'вернётся сегодня'
  if (days === 1) return 'вернётся завтра'
  return `через ${days} дн.`
}
