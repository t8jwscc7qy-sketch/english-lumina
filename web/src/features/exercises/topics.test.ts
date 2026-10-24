import { describe, expect, it } from 'vitest'
import { reliabilityLabel, topicName, weakestTopics } from './topics'

const t = (topic: string, rating: number, deviation = 100) => ({ topic, rating, deviation, attempts: 3 })

describe('topicName', () => {
  it('переводит известные темы', () => {
    expect(topicName('grammar:articles')).toBe('Артикли')
    expect(topicName('grammar:prepositions')).toBe('Предлоги')
    expect(topicName('grammar:future')).toBe('Будущее время')
    expect(topicName('grammar:modals')).toBe('Модальные глаголы')
    expect(topicName('grammar:past_simple')).toBe('Прошедшее простое')
    expect(topicName('skill:form')).toBe('Выбор формы')
    expect(topicName('level:A1')).toBe('Уровень A1')
    expect(topicName('pos:verb')).toBe('Глаголы')
    expect(topicName('pos:noun')).toBe('Существительные')
  })
  it('незнакомые ключи показывает как есть', () => {
    expect(topicName('grammar:nope')).toBe('grammar:nope')
    expect(topicName('whatever')).toBe('whatever')
  })
})

describe('weakestTopics', () => {
  it('берёт пять тем с наименьшим рейтингом по возрастанию и не меняет исходный список', () => {
    const src = [t('a', 1600), t('b', 1300), t('c', 1500), t('d', 1200), t('e', 1700), t('f', 1400), t('g', 1100)]
    const copy = [...src]
    expect(weakestTopics(src).map((x) => x.topic)).toEqual(['g', 'd', 'b', 'f', 'c'])
    expect(src).toEqual(copy)
  })
})

describe('reliabilityLabel', () => {
  it('чем меньше отклонение, тем надёжнее оценка', () => {
    expect(reliabilityLabel(60)).toBe('высокая')
    expect(reliabilityLabel(150)).toBe('средняя')
    expect(reliabilityLabel(350)).toBe('низкая')
  })
})
