import type { components } from './schema.gen'

type ServerErrorCode = components['schemas']['Error']['error']['code']

/** Коды сервера плюс «network» — запрос вообще не дошёл до сервера. */
export type ErrorCode = ServerErrorCode | 'network'

/** Ошибка API: статус 0 означает потерю связи. Текст на русском, его можно показывать пользователю. */
export class ApiError extends Error {
  readonly status: number
  readonly code: ErrorCode

  constructor(status: number, code: ErrorCode, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const NETWORK_MESSAGE = 'Нет связи с сервером. Проверь, что он запущен, и попробуй ещё раз.'
const INTERNAL_MESSAGE = 'Сервер ответил неожиданно. Попробуй ещё раз чуть позже.'

async function parseError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as Partial<components['schemas']['Error']>
    const e = body.error
    if (e && typeof e.code === 'string' && typeof e.message === 'string') {
      return new ApiError(res.status, e.code, e.message)
    }
  } catch {
    // тело не JSON — например, страница прокси; ниже вернём общую ошибку
  }
  return new ApiError(res.status, 'internal', INTERNAL_MESSAGE)
}

async function request(path: string, init: RequestInit = {}): Promise<Response> {
  let res: Response
  try {
    res = await fetch(path, { credentials: 'same-origin', ...init })
  } catch {
    throw new ApiError(0, 'network', NETWORK_MESSAGE)
  }
  if (!res.ok) throw await parseError(res)
  return res
}

export async function login(login: string, password: string): Promise<void> {
  await request('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login, password }),
  })
}

export async function logout(): Promise<void> {
  await request('/api/v1/auth/logout', { method: 'POST' })
}

export async function getMe(): Promise<components['schemas']['Me']> {
  const res = await request('/api/v1/auth/me')
  return (await res.json()) as components['schemas']['Me']
}

export type Settings = components['schemas']['Settings']

export async function getSettings(): Promise<Settings> {
  const res = await request('/api/v1/settings')
  return (await res.json()) as Settings
}

export async function putSettings(settings: Settings): Promise<Settings> {
  const res = await request('/api/v1/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  return (await res.json()) as Settings
}
