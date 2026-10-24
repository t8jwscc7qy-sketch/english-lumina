import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { searchWords } from '@/api/client'

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
