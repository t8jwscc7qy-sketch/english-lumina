import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, getSettings, putSettings, type Settings } from '@/api/client'

export const settingsKey = ['settings'] as const

/** Настройки или null, если онбординг не пройден. Все прочие сбои остаются ошибкой запроса. */
export async function fetchSettings(): Promise<Settings | null> {
  try {
    return await getSettings()
  } catch (e) {
    if (e instanceof ApiError && e.code === 'onboarding_required') return null
    throw e
  }
}

export function useSettings() {
  return useQuery({ queryKey: settingsKey, queryFn: fetchSettings, retry: false, staleTime: 5 * 60_000 })
}

export function useSaveSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: putSettings,
    onSuccess: (saved) => qc.setQueryData(settingsKey, saved),
  })
}
