import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getMe, login, logout } from './client'

function mockFetch(response: Response | Error) {
  const fn = vi.fn(async (_url: string, _init?: RequestInit) => {
    if (response instanceof Error) throw response
    return response
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => vi.unstubAllGlobals())

describe('login', () => {
  it('отправляет логин и пароль JSON-ом на /api/v1/auth/login с cookie', async () => {
    const fetchMock = mockFetch(new Response(null, { status: 204 }))
    await login('firimon', 'secret-password-1')
    const [url, init = {}] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/auth/login')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('same-origin')
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    expect(JSON.parse(init.body as string)).toEqual({
      login: 'firimon',
      password: 'secret-password-1',
    })
  })

  it('превращает ответ сервера в ApiError с кодом и текстом', async () => {
    mockFetch(
      json(401, { error: { code: 'invalid_credentials', message: 'Неверный логин или пароль' } }),
    )
    const err = await login('a', 'b').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({
      status: 401,
      code: 'invalid_credentials',
      message: 'Неверный логин или пароль',
    })
  })

  it.each([
    [429, 'too_many_attempts'],
    [503, 'busy'],
    [400, 'invalid_request'],
    [403, 'forbidden_origin'],
  ] as const)('ответ %i с кодом %s доходит как ApiError', async (status, code) => {
    mockFetch(json(status, { error: { code, message: 'текст' } }))
    await expect(login('a', 'b')).rejects.toMatchObject({ status, code })
  })

  it('потеря сети даёт ApiError с кодом network и понятным текстом', async () => {
    mockFetch(new TypeError('Failed to fetch'))
    const err = await login('a', 'b').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ code: 'network', status: 0 })
    expect((err as ApiError).message).toMatch(/сервер/i)
  })

  it('ответ не в формате API (например, страница прокси) даёт code internal', async () => {
    mockFetch(new Response('<html>Bad gateway</html>', { status: 502 }))
    await expect(login('a', 'b')).rejects.toMatchObject({ status: 502, code: 'internal' })
  })
})

describe('getMe', () => {
  it('возвращает логин пользователя', async () => {
    const fetchMock = mockFetch(json(200, { login: 'firimon' }))
    await expect(getMe()).resolves.toEqual({ login: 'firimon' })
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/auth/me')
  })

  it('401 даёт ApiError unauthorized', async () => {
    mockFetch(json(401, { error: { code: 'unauthorized', message: 'Нужно войти' } }))
    await expect(getMe()).rejects.toMatchObject({ status: 401, code: 'unauthorized' })
  })
})

describe('logout', () => {
  it('отправляет POST и принимает 204', async () => {
    const fetchMock = mockFetch(new Response(null, { status: 204 }))
    await expect(logout()).resolves.toBeUndefined()
    const [url, init = {}] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/auth/logout')
    expect(init.method).toBe('POST')
  })
})
