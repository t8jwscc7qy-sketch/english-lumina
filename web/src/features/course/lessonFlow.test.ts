import { describe, expect, it } from 'vitest'
import { buildSteps, orderCorrect, withRetry, wordsWord, stageLabel } from './lessonFlow'
import { lessonFixture } from './fixtures'

describe('lessonFlow', () => {
  it('шаги идут по порядку: слова, текст, вопросы, задания, итог', () => {
    const steps = buildSteps(lessonFixture)
    expect(steps.map((s) => s.kind)).toEqual(['word', 'word', 'text', 'question', 'drill', 'drill', 'drill', 'summary'])
  })

  it('задание с ошибкой возвращается один раз перед итогом', () => {
    const steps = buildSteps(lessonFixture)
    const once = withRetry(steps, { kind: 'drill', index: 1 })
    expect(once.at(-2)).toEqual({ kind: 'drill', index: 1, retry: true })
    expect(withRetry(once, { kind: 'drill', index: 1, retry: true })).toBe(once)
    expect(withRetry(steps, { kind: 'question', index: 0 })).toBe(steps)
    expect(stageLabel({ kind: 'drill', index: 1, retry: true })).toBe('Ещё раз')
  })

  it('порядок слов сравнивается без регистра и знаков', () => {
    expect(orderCorrect(['How', 'are', 'you'], 'How are you')).toBe(true)
    expect(orderCorrect(['are', 'How', 'you'], 'How are you')).toBe(false)
    expect(orderCorrect(['i', 'am', 'fine,'], 'I am fine')).toBe(true)
  })

  it('склоняет «слово»', () => {
    expect([1, 2, 5, 11, 21, 22].map(wordsWord)).toEqual(['слово', 'слова', 'слов', 'слов', 'слово', 'слова'])
  })
})
