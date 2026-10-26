import { useMutation, useQuery } from '@tanstack/react-query'
import { getNextText, knowTextWord, rateText, type ReadMode, type Understood } from '@/api/client'

export const nextTextKey = (mode: ReadMode) => ['texts', 'next', mode] as const

/** Текущий текст. Не перечитывается сам: пока читаешь, текст не должен меняться; следующий берётся явным refetch. */
export function useNextText(mode: ReadMode) {
  return useQuery({ queryKey: nextTextKey(mode), queryFn: () => getNextText(mode), retry: false, staleTime: Infinity, gcTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false })
}

export function useRateText() {
  return useMutation({ mutationFn: ({ textId, understood, mode }: { textId: number; understood: Understood; mode: ReadMode }) => rateText(textId, understood, mode) })
}

export function useKnowWord() {
  return useMutation({ mutationFn: (lexemeId: number) => knowTextWord(lexemeId) })
}
