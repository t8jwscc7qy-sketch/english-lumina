import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { completeLesson, getCourse, getLesson } from '@/api/client'
import { cardQueueKey } from '@/features/cards/cards'
import { learnStatusKey } from '@/features/learn/learn'

export const courseKey = ['course'] as const
export const lessonKey = (id: string | null) => ['course', 'lesson', id ?? 'current'] as const

/** Путь: блоки и уроки с состоянием. */
export function useCourse() {
  return useQuery({ queryKey: courseKey, queryFn: getCourse, retry: false })
}

/** Урок не перечитывается сам: пока человек проходит шаги, урок не должен меняться под ним. */
export function useLesson(id: string | null) {
  return useQuery({
    queryKey: lessonKey(id),
    queryFn: () => getLesson(id ?? undefined),
    retry: false,
    staleTime: Infinity,
    gcTime: 0,
    refetchOnWindowFocus: false,
  })
}

export function useCompleteLesson() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { lessonId: string; key: string; seconds: number; mistakes: number }) =>
      completeLesson(v.lessonId, v.key, v.seconds, v.mistakes),
    onSuccess: () => {
      // Только путь (exact): иначе перечитается и открытый урок, и итог подменится следующим уроком.
      void qc.invalidateQueries({ queryKey: courseKey, exact: true })
      void qc.invalidateQueries({ queryKey: cardQueueKey })
      void qc.invalidateQueries({ queryKey: learnStatusKey })
    },
  })
}
