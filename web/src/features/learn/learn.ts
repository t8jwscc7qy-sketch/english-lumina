import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getLearnBlock,
  getLearnPreferences,
  getLearnStatus,
  getPlacement,
  introduceWords,
  putLearnPreferences,
  submitPlacement,
  type LearnPreferences,
} from '@/api/client'
import { cardQueueKey } from '@/features/cards/cards'
import { progressKey } from '@/features/today/progress'

export const learnStatusKey = ['learn', 'status'] as const
export const learnBlockKey = ['learn', 'block'] as const
export const learnPrefsKey = ['learn', 'preferences'] as const

/** Сводка знакомства. Ошибка не мешает странице: карточка на «Сегодня» просто не показывается. */
export function useLearnStatus() {
  return useQuery({ queryKey: learnStatusKey, queryFn: getLearnStatus, retry: false, refetchOnWindowFocus: false })
}

/** Блок слов не перечитывается сам: пока человек читает слова, набор не должен меняться под ним. */
export function useLearnBlock() {
  return useQuery({ queryKey: learnBlockKey, queryFn: getLearnBlock, retry: false, staleTime: Infinity, gcTime: 0, refetchOnWindowFocus: false })
}

export function useIntroduce() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ids, key }: { ids: number[]; key: string }) => introduceWords(ids, key),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: learnStatusKey })
      void qc.invalidateQueries({ queryKey: cardQueueKey })
      void qc.invalidateQueries({ queryKey: progressKey })
      qc.removeQueries({ queryKey: learnBlockKey })
    },
  })
}

export function usePlacementPage(skip: number) {
  return useQuery({ queryKey: ['learn', 'placement', skip], queryFn: () => getPlacement(skip), retry: false, staleTime: Infinity, gcTime: 0, refetchOnWindowFocus: false })
}

export function useSubmitPlacement() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ known, shown, key }: { known: number[]; shown: number[]; key: string }) => submitPlacement(known, shown, key),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: learnStatusKey })
      qc.removeQueries({ queryKey: learnBlockKey })
    },
  })
}

export function useLearnPreferences() {
  return useQuery({ queryKey: learnPrefsKey, queryFn: getLearnPreferences, retry: false })
}

export function useSaveLearnPreferences() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (p: LearnPreferences) => putLearnPreferences(p),
    onSuccess: (saved) => {
      qc.setQueryData(learnPrefsKey, saved)
      void qc.invalidateQueries({ queryKey: learnStatusKey })
    },
  })
}

/** Ключ идемпотентности: новый на каждый показанный блок или страницу. */
export function newKey(): string {
  return typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

/** Сколько слов пропустить на следующей странице теста: «знаю» из списка пропадают, «не знаю» остаются. */
export function nextSkip(skip: number, shown: number, known: number): number {
  return skip + (shown - known)
}
