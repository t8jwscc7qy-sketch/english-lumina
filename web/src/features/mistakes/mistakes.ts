import { useQuery } from '@tanstack/react-query'
import { getMistakes } from '@/api/client'

export type { Mistake, MistakeList } from '@/api/client'

export const mistakesKey = ['mistakes'] as const

/** Банк ошибок. Страница открывается после упражнений, поэтому данные всегда перечитываются. */
export function useMistakes() {
  return useQuery({ queryKey: mistakesKey, queryFn: getMistakes, retry: false, staleTime: 0, refetchOnWindowFocus: false })
}

const DAY = 24 * 3600_000

/** Когда ошибка вернётся: прошлое и настоящее — «ждёт повтора», до суток вперёд — «вернётся завтра», дальше — «через N дн.». */
export function formatDue(nextDue: string, now: Date): string {
  const diff = new Date(nextDue).getTime() - now.getTime()
  if (!(diff > 0)) return 'ждёт повтора'
  const days = Math.ceil(diff / DAY)
  return days <= 1 ? 'вернётся завтра' : `через ${days} дн.`
}
