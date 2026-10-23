import { describe, expect, it } from 'vitest'
import { describeDuration, forecastBackward, forecastForward } from './forecast'

const today = new Date(2026, 9, 23)

function plusDays(d: Date, n: number) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate() + n)
}

describe('forecastForward', () => {
  it('считает B1→B2 при 90 минутах в день: 6–8 месяцев, 175–233 дня', () => {
    const f = forecastForward('B1', 'B2', 90, today)
    if (f.kind !== 'estimate') throw new Error('ожидали оценку')
    expect(f.hours.min).toBeCloseTo(262.5)
    expect(f.hours.max).toBeCloseTo(350)
    expect(f.days.min).toBeCloseTo(175)
    expect(f.days.max).toBeCloseTo(233.33, 1)
    expect(f.finishBy.earliest).toEqual(plusDays(today, 175))
    expect(f.finishBy.latest).toEqual(plusDays(today, 234))
  })

  it('складывает переходы через несколько уровней: A1→C1', () => {
    const f = forecastForward('A1', 'C1', 60, today)
    if (f.kind !== 'estimate') throw new Error('ожидали оценку')
    expect(f.hours.min).toBeCloseTo((95 + 185 + 175 + 200) * 1.5)
    expect(f.hours.max).toBeCloseTo((95 + 185 + 175 + 200) * 2)
  })

  it('цель равна уровню или ниже него: цель достигнута, без деления на ноль', () => {
    expect(forecastForward('B2', 'B2', 30, today)).toEqual({ kind: 'reached' })
    expect(forecastForward('C1', 'B1', 30, today)).toEqual({ kind: 'reached' })
  })
})

describe('forecastBackward', () => {
  it('B1→B2 за 200 дней: нужно около 79–105 минут в день', () => {
    const f = forecastBackward('B1', 'B2', plusDays(today, 200), today)
    if (f.kind !== 'needed') throw new Error('ожидали «нужно»')
    expect(f.minutesPerDay.min).toBeCloseTo((262.5 * 60) / 200)
    expect(f.minutesPerDay.max).toBeCloseTo((350 * 60) / 200)
    expect(f.unrealistic).toBe(false)
  })

  it('за 30 дней нужно больше 240 минут в день: нереалистично', () => {
    const f = forecastBackward('B1', 'B2', plusDays(today, 30), today)
    if (f.kind !== 'needed') throw new Error('ожидали «нужно»')
    expect(f.unrealistic).toBe(true)
  })

  it('дата в прошлом: past', () => {
    expect(forecastBackward('B1', 'B2', plusDays(today, -5), today)).toEqual({ kind: 'past' })
  })

  it('дата сегодня допустима: это нереалистично, но не «прошла»', () => {
    const f = forecastBackward('B1', 'B2', today, today)
    if (f.kind !== 'needed') throw new Error('ожидали «нужно»')
    expect(f.unrealistic).toBe(true)
  })

  it('цель равна уровню: reached, даже если дата в прошлом', () => {
    expect(forecastBackward('B2', 'B2', plusDays(today, -5), today)).toEqual({ kind: 'reached' })
  })
})

describe('describeDuration', () => {
  const m = (n: number) => n * 30.4

  it('диапазон месяцев', () => {
    expect(describeDuration({ min: 175, max: 233 })).toBe('6–8 месяцев')
  })

  it.each([
    [1, 2, '1–2 месяца'],
    [3, 5, '3–5 месяцев'],
    [10, 11, '10–11 месяцев'],
    [11, 12, '11–12 месяцев'],
    [13, 14, '13–14 месяцев'],
    [20, 21, '20–21 месяц'],
    [21, 22, '21–22 месяца'],
  ])('склоняет месяцы по последнему числу: %i–%i', (a, b, text) => {
    expect(describeDuration({ min: m(a), max: m(b) })).toBe(text)
  })

  it.each([
    [7, 14, '1–2 недели'],
    [21, 28, '3–4 недели'],
    [14, 30, '2–4 недели'],
  ])('меньше месяца считает в неделях: %i–%i дней', (a, b, text) => {
    expect(describeDuration({ min: a, max: b })).toBe(text)
  })

  it('одинаковые границы после округления: «около N»', () => {
    expect(describeDuration({ min: m(2), max: m(2) })).toBe('около 2 месяцев')
    expect(describeDuration({ min: m(1), max: m(1) })).toBe('около 1 месяца')
    expect(describeDuration({ min: m(21), max: m(21) })).toBe('около 21 месяца')
    expect(describeDuration({ min: 7, max: 7 })).toBe('около 1 недели')
    expect(describeDuration({ min: 35, max: 35 })).toBe('около 1 месяца')
  })
})
