import { describe, expect, it } from 'vitest'
import { formatHoursMinutes, formatRange } from './progress'

describe('formatHoursMinutes', () => {
  it('минуты и часы по-русски', () => {
    expect(formatHoursMinutes(0)).toBe('0 мин')
    expect(formatHoursMinutes(45)).toBe('45 мин')
    expect(formatHoursMinutes(60)).toBe('1 ч')
    expect(formatHoursMinutes(860)).toBe('14 ч 20 мин')
  })
  it('округляет до минуты и не показывает 60 мин', () => {
    expect(formatHoursMinutes(59.6)).toBe('1 ч')
    expect(formatHoursMinutes(120.4)).toBe('2 ч')
  })
})

describe('formatRange', () => {
  it('диапазон часов', () => {
    expect(formatRange(280, 370)).toBe('280–370 ч')
    expect(formatRange(262.5, 350)).toBe('263–350 ч')
  })
  it('границы совпали после округления — одно число', () => {
    expect(formatRange(100.2, 100.4)).toBe('100 ч')
  })
})
