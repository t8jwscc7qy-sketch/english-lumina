import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getMe, addCard, getCardQueue, getSettings, getSources, getWordExamples, login, reviewCard, searchWords, logout, putSettings } from './client'

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

describe('settings', () => {
  const settings = {
    currentLevel: 'B1',
    targetLevel: 'B2',
    targetDate: '2027-06-01',
    dailyMinutes: 90,
    timezone: 'Europe/Moscow',
  } as const

  it('getSettings читает /api/v1/settings', async () => {
    const fetchMock = mockFetch(json(200, settings))
    expect(await getSettings()).toEqual(settings)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/settings')
  })

  it('putSettings отправляет настройки JSON-ом методом PUT и возвращает ответ сервера', async () => {
    const fetchMock = mockFetch(json(200, settings))
    expect(await putSettings(settings)).toEqual(settings)
    const [url, init = {}] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/settings')
    expect(init.method).toBe('PUT')
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    expect(JSON.parse(init.body as string)).toEqual(settings)
  })

  it('404 onboarding_required становится ApiError с этим кодом', async () => {
    mockFetch(json(404, { error: { code: 'onboarding_required', message: 'Сначала настрой' } }))
    await expect(getSettings()).rejects.toMatchObject({ status: 404, code: 'onboarding_required' })
  })
})

describe('sources', () => {
  it('getSources читает /api/v1/sources и отдаёт список', async () => {
    const list = { sources: [{ name: 'cefrj', version: '1.5', license: 'l', url: 'u', attribution: 'a', rowCount: 3, importedAt: '2026-10-23T12:00:00Z' }] }
    const fetchMock = mockFetch(json(200, list))
    expect(await getSources()).toEqual(list)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/sources')
  })
})

describe('words', () => {
  it('searchWords экранирует запрос в URL и отдаёт список', async () => {
    const list = { words: [{ lemma: 'café', pos: 'noun', level: 'A1', levels: [] }] }
    const fetchMock = mockFetch(json(200, list))
    expect(await searchWords('Café & more/50%')).toEqual(list)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/words?q=Caf%C3%A9%20%26%20more%2F50%25')
  })
})

describe('examples', () => {
  it('getWordExamples передаёт слово и часть речи, экранируя их', async () => {
    const list = { examples: [] }
    const fetchMock = mockFetch(json(200, list))
    expect(await getWordExamples('cafe/café', 'noun')).toEqual(list)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/words/examples?lemma=cafe%2Fcaf%C3%A9&pos=noun')
  })
})

describe('cards', () => {
  it('getCardQueue читает очередь', async () => {
    const queue = { dueCount: 2, totalCount: 5 }
    const fetchMock = mockFetch(json(200, queue))
    expect(await getCardQueue()).toEqual(queue)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/cards/queue')
  })

  it('reviewCard отправляет номер, оценку и ключ телом POST', async () => {
    const queue = { dueCount: 1, totalCount: 5 }
    const fetchMock = mockFetch(json(200, queue))
    expect(await reviewCard(7, 3, 'key-1')).toEqual(queue)
    const [url, init = {}] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/cards/review')
    expect(init.method).toBe('POST')
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    expect(JSON.parse(init.body as string)).toEqual({ cardId: 7, rating: 3, key: 'key-1' })
  })

  it('addCard отправляет слово и часть речи', async () => {
    const fetchMock = mockFetch(json(201, { cardId: 9, created: true }))
    expect(await addCard('run', 'verb')).toEqual({ cardId: 9, created: true })
    const [url, init = {}] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/cards')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({ lemma: 'run', pos: 'verb' })
  })

  it('409 no_examples становится ApiError с этим кодом', async () => {
    mockFetch(json(409, { error: { code: 'no_examples', message: 'Нет примеров' } }))
    await expect(addCard('zebra', 'noun')).rejects.toMatchObject({ status: 409, code: 'no_examples' })
  })
})
