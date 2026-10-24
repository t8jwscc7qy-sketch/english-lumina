import { describe, expect, it } from 'vitest'
import { formatMinutes, sessionButtonLabel, streakWord } from './progress'

const base = { minimumDay: 10, goalMinutes: 15, streakDays: 0, today: { answers: 0, correct: 0, cardsReviewed: 0, mistakesOpened: 0, mistakesClosed: 0 } }

describe('sessionButtonLabel', () => {
  it('нет минут: «Начать сессию»', () => {
    expect(sessionButtonLabel({ ...base, minutes: 0, doneToday: false })).toBe('Начать сессию')
  })
  it('9,9 минуты: «Продолжить сессию»', () => {
    expect(sessionButtonLabel({ ...base, minutes: 9.9, doneToday: false })).toBe('Продолжить сессию')
  })
  it('минимум выполнен: «Позаниматься ещё»', () => {
    expect(sessionButtonLabel({ ...base, minutes: 10, doneToday: true })).toBe('Позаниматься ещё')
  })
  it('граница определяется сервером (doneToday), а не округлённым числом', () => {
    // 599 с сервер округлил до 10,0, но минимум ещё не выполнен
    expect(sessionButtonLabel({ ...base, minutes: 10, doneToday: false })).toBe('Продолжить сессию')
  })
  it('нет данных: «Начать сессию»', () => {
    expect(sessionButtonLabel(undefined)).toBe('Начать сессию')
  })
})

describe('formatMinutes', () => {
  it('целые без дроби, доля через запятую', () => {
    expect(formatMinutes(4)).toBe('4')
    expect(formatMinutes(4.3)).toBe('4,3')
    expect(formatMinutes(0)).toBe('0')
  })
})

describe('streakWord', () => {
  it('склоняет', () => {
    expect([0, 1, 2, 4, 5, 11, 21].map(streakWord)).toEqual(['дней', 'день', 'дня', 'дня', 'дней', 'дней', 'день'])
  })
})
