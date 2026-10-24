import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, getMe, login, logout } from '@/api/client'

export const meKey = ['me'] as const

/** Текущий пользователь или null, если не вошёл. Сетевые и серверные сбои остаются ошибкой запроса. */
export async function fetchMe() {
  try {
    return await getMe()
  } catch (e) {
    if (e instanceof ApiError && e.code === 'unauthorized') return null
    throw e
  }
}

export function useMe() {
  return useQuery({ queryKey: meKey, queryFn: fetchMe, retry: false, staleTime: 5 * 60_000 })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ login: l, password }: { login: string; password: string }) => login(l, password),
    // Пользователь изменился — заставляем «кто я» перечитаться, даже если в кэше был null.
    onSuccess: () => qc.invalidateQueries({ queryKey: meKey }),
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: logout,
    // Выход всегда стирает локальные данные, даже если сервер не ответил: cookie мы всё равно теряем.
    onSettled: () => {
      qc.clear()
      qc.setQueryData(meKey, null)
    },
  })
}
