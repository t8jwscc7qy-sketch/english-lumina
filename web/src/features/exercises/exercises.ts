import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { answerExercise, getNextExercise, listExerciseTopics } from '@/api/client'

export type { Exercise, AnswerResult, Rating } from '@/api/client'

export const nextExerciseKey = ['exercises', 'next'] as const
export const topicsKey = ['exercises', 'topics'] as const

/** Текущее упражнение. Кэш не переиспользуется: следующее берётся только явным refetch после «Дальше». */
export function useNextExercise() {
  return useQuery({ queryKey: nextExerciseKey, queryFn: getNextExercise, retry: false, staleTime: 0, gcTime: 0 })
}

export function useTopics() {
  return useQuery({ queryKey: topicsKey, queryFn: listExerciseTopics, retry: false })
}

export function useAnswer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ exerciseId, given, key }: { exerciseId: number; given: string; key: string }) => answerExercise(exerciseId, given, key),
    onSuccess: () => qc.invalidateQueries({ queryKey: topicsKey }),
  })
}
