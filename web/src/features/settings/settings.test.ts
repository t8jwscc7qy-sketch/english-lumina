import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import { errorResponse, jsonResponse } from '@/test/render'
import { fetchSettings } from './settings'

const saved = {
  currentLevel: 'B1',
  targetLevel: 'B2',
  dailyMinutes: 90,
  timezone: 'Europe/Moscow',
}

afterEach(() => vi.unstubAllGlobals())

describe('fetchSettings', () => {
  it('возвращает настройки, если они есть', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, saved)))
    expect(await fetchSettings()).toEqual(saved)
  })

  it('404 onboarding_required значит «онбординг не пройден» и даёт null', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => errorResponse(404, 'onboarding_required', 'Сначала настрой цель и уровень')),
    )
    expect(await fetchSettings()).toBeNull()
  })

  it('сбой сервера остаётся ошибкой, а не null', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errorResponse(500, 'internal', 'Внутренняя ошибка')))
    await expect(fetchSettings()).rejects.toBeInstanceOf(ApiError)
  })

  it('потеря связи остаётся ошибкой, а не null', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    await expect(fetchSettings()).rejects.toMatchObject({ code: 'network' })
  })

  it('401 тоже остаётся ошибкой: за вход отвечает другой защитник', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errorResponse(401, 'unauthorized', 'Нужно войти')))
    await expect(fetchSettings()).rejects.toMatchObject({ status: 401 })
  })
})
