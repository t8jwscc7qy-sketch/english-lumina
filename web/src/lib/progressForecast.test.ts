import { describe, expect, it } from 'vitest'
import { forecastForward } from './forecast'
import { forecastFromPace } from './progressForecast'

const today = new Date(2026, 9, 25)

describe('forecastFromPace', () => {
  it('цель не выше уровня: reached', () => {
    expect(forecastFromPace('B2', 'B2', 0, 30, true, 20, today)).toEqual({ kind: 'reached' })
    expect(forecastFromPace('C1', 'B1', 0, 30, true, 20, today)).toEqual({ kind: 'reached' })
  })

  it('темп ещё не готов: collecting с числом суток', () => {
    expect(forecastFromPace('B1', 'B2', 100, 30, false, 5, today)).toEqual({ kind: 'collecting', daysSinceStart: 5 })
    expect(forecastFromPace('B1', 'B2', 0, 0, false, 0, today)).toEqual({ kind: 'collecting', daysSinceStart: 0 })
  })

  it('темп готов, но 0: idle (без деления на ноль)', () => {
    expect(forecastFromPace('B1', 'B2', 600, 0, true, 20, today)).toEqual({ kind: 'idle' })
  })

  it('оценка B1→B2 при 30 минутах и 0 набранных совпадает с forecastForward', () => {
    const f = forecastFromPace('B1', 'B2', 0, 30, true, 20, today)
    const base = forecastForward('B1', 'B2', 30, today)
    if (f.kind !== 'estimate' || base.kind !== 'estimate') throw new Error('ожидали оценку')
    expect(f.days.min).toBeCloseTo(base.days.min, 6)
    expect(f.days.max).toBeCloseTo(base.days.max, 6)
    expect(f.finishBy.earliest).toEqual(base.finishBy.earliest)
    expect(f.finishBy.latest).toEqual(base.finishBy.latest)
    expect(f.needed).toEqual(base.hours)
    expect(f.unrealistic).toBe(false)
  })

  it('набранные часы вычитаются отдельно по границам: нижняя граница 0 суток, верхняя больше', () => {
    // нужно 262,5–350 ч; набрано ровно 262,5 ч
    const f = forecastFromPace('B1', 'B2', 262.5 * 60, 30, true, 90, today)
    if (f.kind !== 'estimate') throw new Error('ожидали оценку')
    expect(f.remaining.min).toBe(0)
    expect(f.remaining.max).toBeCloseTo(87.5, 6)
    expect(f.days.min).toBe(0)
    expect(f.days.max).toBeCloseTo(175, 6)
    expect(f.finishBy.earliest).toEqual(today)
  })

  it('набрано не меньше верхней границы: enough', () => {
    expect(forecastFromPace('B1', 'B2', 350 * 60, 30, true, 400, today)).toEqual({ kind: 'enough' })
    expect(forecastFromPace('B1', 'B2', 400 * 60, 30, false, 3, today)).toEqual({ kind: 'enough' })
  })

  it('подсказка «+10 минут»: разница верхних границ в неделях', () => {
    const f = forecastFromPace('B1', 'B2', 0, 30, true, 20, today)
    if (f.kind !== 'estimate') throw new Error('ожидали оценку')
    // 700 суток при 30 мин и 525 при 40 мин: 175 суток = 25 недель
    expect(f.saveWithTenMore).toEqual({ weeks: 25 })
  })

  it('экономия меньше недели: подсказки нет', () => {
    const f = forecastFromPace('B1', 'B2', 0, 236, true, 20, today)
    if (f.kind !== 'estimate') throw new Error('ожидали оценку')
    expect(f.saveWithTenMore).toBeUndefined()
    expect(f.unrealistic).toBe(false)
  })

  it('темп выше 240 минут в день: нереально, подсказки нет', () => {
    const f = forecastFromPace('B1', 'B2', 0, 300, true, 20, today)
    if (f.kind !== 'estimate') throw new Error('ожидали оценку')
    expect(f.unrealistic).toBe(true)
    expect(f.saveWithTenMore).toBeUndefined()
  })
})
