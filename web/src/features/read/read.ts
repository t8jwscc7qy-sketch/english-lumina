import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getNextText, knowTextWord, rateText, type ReadMode, type Understood } from '@/api/client'

export const nextTextKey = (mode: ReadMode) => ['texts', 'next', mode] as const

/** Текущий текст. Не перечитывается сам: пока читаешь, текст не должен меняться; следующий берётся явным refetch. */
export function useNextText(mode: ReadMode) {
  return useQuery({ queryKey: nextTextKey(mode), queryFn: () => getNextText(mode), retry: false, staleTime: Infinity, gcTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false })
}

/** Другой текст (или следующий после оценки): сервер собирает новый, а не отдаёт прежний неоценённый. */
export function useFreshText(mode: ReadMode) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => getNextText(mode, true),
    onSuccess: (text) => qc.setQueryData(nextTextKey(mode), text),
  })
}

export function useRateText() {
  return useMutation({ mutationFn: ({ textId, understood, mode }: { textId: number; understood: Understood; mode: ReadMode }) => rateText(textId, understood, mode) })
}

export function useKnowWord() {
  return useMutation({ mutationFn: (lexemeId: number) => knowTextWord(lexemeId) })
}
