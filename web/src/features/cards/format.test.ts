import { describe, expect, it } from 'vitest'
import { formatInterval } from './format'

const DAY = 86_400

describe('formatInterval', () => {
  it.each([
    [0, 'меньше минуты'],
    [30, 'меньше минуты'],
    [59, 'меньше минуты'],
    [60, '1 мин'],
    [360, '6 мин'],
    [600, '10 мин'],
    [3570, '1 ч'], // 59,5 минуты округляется в час, а не в «60 мин»
    [3600, '1 ч'],
    [7200, '2 ч'],
    [DAY - 1, '1 дн'], // 23,99 часа округляется в день, а не в «24 ч»
    [DAY, '1 дн'],
    [3 * DAY, '3 дн'],
    [29 * DAY, '29 дн'],
    [30 * DAY, '1 мес.'],
    [45 * DAY, '1 мес.'],
    [90 * DAY, '3 мес.'],
    [364 * DAY, '1 г.'],
    [400 * DAY, '1 г.'],
    [800 * DAY, '2 г.'],
  ])('%i секунд → «%s»', (seconds, text) => {
    expect(formatInterval(seconds)).toBe(text)
  })

  it('отрицательное и нечисловое значения не ломают вывод', () => {
    expect(formatInterval(-5)).toBe('меньше минуты')
    expect(formatInterval(Number.NaN)).toBe('меньше минуты')
  })
})
