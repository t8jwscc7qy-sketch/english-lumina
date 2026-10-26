import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { mistakesKey } from '@/features/mistakes/mistakes'
import { answerExercise, getNextExercise, listExerciseTopics } from '@/api/client'

export type { Exercise, AnswerResult, Rating } from '@/api/client'

export const nextExerciseKey = ['exercises', 'next'] as const
export const topicsKey = ['exercises', 'topics'] as const

/** Текущее упражнение. Кэш не переиспользуется; фокус окна и возврат сети его не меняют: следующее берётся только явным refetch после «Дальше». */
export function useNextExercise(mode?: 'camp') {
  return useQuery({ queryKey: mode ? [...nextExerciseKey, mode] : nextExerciseKey, queryFn: () => getNextExercise(mode), retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false })
}

export function useTopics() {
  return useQuery({ queryKey: topicsKey, queryFn: listExerciseTopics, retry: false })
}

export function useAnswer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ exerciseId, given, key }: { exerciseId: number; given: string; key: string }) => answerExercise(exerciseId, given, key),
    onSuccess: () => Promise.all([qc.invalidateQueries({ queryKey: topicsKey }), qc.invalidateQueries({ queryKey: mistakesKey })]),
  })
}
