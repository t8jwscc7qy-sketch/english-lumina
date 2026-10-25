import { describe, expect, it } from 'vitest'
import { goalFollows, settingsAfterAccept } from './level'

const S = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 30, timezone: 'UTC' } as const

describe('settingsAfterAccept', () => {
  it('повышение в пределах цели меняет только уровень', () => {
    expect(settingsAfterAccept(S, 'B2')).toEqual({ ...S, currentLevel: 'B2', targetLevel: 'B2' })
    expect(settingsAfterAccept({ ...S, targetLevel: 'C1' }, 'B2')).toEqual({ ...S, currentLevel: 'B2', targetLevel: 'C1' })
  })

  it('повышение выше цели поднимает цель вместе с уровнем: цель не бывает ниже уровня', () => {
    expect(settingsAfterAccept(S, 'C1')).toEqual({ ...S, currentLevel: 'C1', targetLevel: 'C1' })
  })

  it('понижение не трогает цель', () => {
    expect(settingsAfterAccept(S, 'A2')).toEqual({ ...S, currentLevel: 'A2', targetLevel: 'B2' })
  })

  it('прочие настройки сохраняются', () => {
    const withDate = { ...S, targetDate: '2027-01-01', dailyMinutes: 45 }
    expect(settingsAfterAccept(withDate, 'B2')).toMatchObject({ targetDate: '2027-01-01', dailyMinutes: 45 })
  })
})

describe('goalFollows', () => {
  it('только когда уровень выйдет выше цели', () => {
    expect(goalFollows(S, 'B2')).toBe(false)
    expect(goalFollows(S, 'C1')).toBe(true)
    expect(goalFollows(S, 'A2')).toBe(false)
  })
})
