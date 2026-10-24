import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { addCard, getCardQueue, reviewCard } from '@/api/client'

export const cardQueueKey = ['cards', 'queue'] as const

/** Очередь на сейчас. Срок карточек наступает со временем, поэтому данные не считаются свежими и берутся заново при каждом открытии. */
export function useCardQueue() {
  return useQuery({ queryKey: cardQueueKey, queryFn: getCardQueue, retry: false })
}

export function useReviewCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ cardId, rating, key }: { cardId: number; rating: 1 | 2 | 3 | 4; key: string }) => reviewCard(cardId, rating, key),
    // Ответ уже содержит очередь после оценки: второй запрос не нужен.
    onSuccess: (queue) => qc.setQueryData(cardQueueKey, queue),
  })
}

export function useAddCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ lemma, pos }: { lemma: string; pos: string }) => addCard(lemma, pos),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: cardQueueKey })
      void qc.invalidateQueries({ queryKey: ['words'] }) // у слова в поиске появится признак inCards
    },
  })
}
