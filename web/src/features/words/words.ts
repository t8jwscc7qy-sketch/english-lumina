import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getWordExamples, searchWords } from '@/api/client'

/** Поиск слова. Пустой запрос не отправляется; пока набирается следующий, прежние результаты остаются на экране. */
export function useWordSearch(term: string) {
  return useQuery({
    queryKey: ['words', term],
    queryFn: () => searchWords(term),
    enabled: term !== '',
    retry: false,
    staleTime: 5 * 60_000,
    placeholderData: keepPreviousData,
  })
}

/** Примеры слова. Запрашиваются, только когда панель раскрыта; результат кэшируется. */
export function useWordExamples(lemma: string, pos: string) {
  return useQuery({
    queryKey: ['examples', lemma, pos],
    queryFn: () => getWordExamples(lemma, pos),
    retry: false,
    staleTime: 5 * 60_000,
  })
}
