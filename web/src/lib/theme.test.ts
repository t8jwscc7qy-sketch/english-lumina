import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { applyTheme, readStoredTheme, resolveTheme, storeTheme } from './theme'

describe('resolveTheme', () => {
  it('явный выбор главнее системной настройки', () => {
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })

  it('«как в системе» следует настройке системы', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
  })
})

describe('хранение темы', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => document.documentElement.classList.remove('dark'))

  it('по умолчанию — «как в системе»', () => {
    expect(readStoredTheme()).toBe('system')
  })

  it('сохраняет и читает выбранную тему', () => {
    storeTheme('dark')
    expect(readStoredTheme()).toBe('dark')
  })

  it('игнорирует мусор в хранилище', () => {
    localStorage.setItem('verst-theme', 'purple')
    expect(readStoredTheme()).toBe('system')
  })

  it('applyTheme ставит и снимает класс dark на <html>', () => {
    applyTheme('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    applyTheme('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
