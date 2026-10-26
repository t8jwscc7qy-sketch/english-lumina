import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { listSheets, openSheet, reviewSheet, setSheetFavorite, type SheetSummary } from '@/api/client'

export const sheetsKey = ['sheets', 'list'] as const
export const sheetKey = (id: string) => ['sheets', 'sheet', id] as const

/** Список шпаргалок. Ошибка не мешает странице: блок «Закреплённое» на «Сегодня» просто не показывается. */
export function useSheets() {
  return useQuery({ queryKey: sheetsKey, queryFn: listSheets, retry: false, refetchOnWindowFocus: false })
}

/** Шпаргалка с данными пользователя; открытие запоминается на сервере, поэтому данные не переиспользуются. */
export function useSheet(id: string) {
  return useQuery({ queryKey: sheetKey(id), queryFn: () => openSheet(id), retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false })
}

export function useSetFavorite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, favorite }: { id: string; favorite: boolean }) => setSheetFavorite(id, favorite),
    onSuccess: (_, { id, favorite }) => {
      qc.setQueryData(sheetKey(id), (s: { favorite: boolean } | undefined) => (s ? { ...s, favorite } : s))
      void qc.invalidateQueries({ queryKey: sheetsKey })
    },
  })
}

export function useReviewSheet() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => reviewSheet(id),
    onSuccess: (_, id) => {
      qc.setQueryData(sheetKey(id), (s: { status: string } | undefined) => (s ? { ...s, status: 'reviewed' } : s))
      void qc.invalidateQueries({ queryKey: sheetsKey })
    },
  })
}

/** Закреплённые: недавно открытые первыми. */
export function pinned(sheets: SheetSummary[]): SheetSummary[] {
  return sheets
    .filter((s) => s.favorite)
    .sort((a, b) => (b.openedAt ?? '').localeCompare(a.openedAt ?? ''))
}

/** Поиск по названию, описанию и тегам без учёта регистра; пустой запрос и пустой тег ничего не отсекают. */
export function filterSheets(sheets: SheetSummary[], query: string, tag: string | null): SheetSummary[] {
  const q = query.trim().toLowerCase()
  return sheets.filter(
    (s) =>
      (tag === null || s.tags.includes(tag)) &&
      (q === '' || [s.title, s.summary, ...s.tags].some((t) => t.toLowerCase().includes(q))),
  )
}
