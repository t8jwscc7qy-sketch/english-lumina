import { useQuery } from '@tanstack/react-query'
import { getSources } from '@/api/client'

export const sourcesKey = ['sources'] as const

/** Загруженные источники данных. Меняются только после импорта, поэтому кэш живёт долго. */
export function useSources() {
  return useQuery({ queryKey: sourcesKey, queryFn: getSources, retry: false, staleTime: 5 * 60_000 })
}
