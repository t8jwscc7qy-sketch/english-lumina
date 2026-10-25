import { describe, expect, it } from 'vitest'
import {
  emptyState,
  isComplete,
  optionLabel,
  pairColor,
  selectOption,
  selectWord,
  toGiven,
  undoLast,
  unpair,
  type MatchState,
} from './match'

const WORDS = ['apple', 'house', 'water', 'book', 'dog']
const OPTIONS = ['книга', 'яблоко', 'собака', 'вода', 'дом']

const pair = (s: MatchState, w: number, o: number) => selectOption(selectWord(s, w), o)

describe('match: выбор и соединение', () => {
  it('начальное состояние пустое', () => {
    expect(emptyState(5)).toEqual({ selected: null, pairs: [null, null, null, null, null], history: [] })
  })

  it('слово, затем перевод соединяют пару и снимают выбор', () => {
    const s = pair(emptyState(5), 0, 1)
    expect(s.pairs[0]).toBe(1)
    expect(s.selected).toBeNull()
    expect(s.history).toEqual([0])
  })

  it('повторное нажатие на выбранное слово снимает выбор', () => {
    const s = selectWord(selectWord(emptyState(5), 2), 2)
    expect(s.selected).toBeNull()
  })

  it('выбор другого слова переносит выбор', () => {
    const s = selectWord(selectWord(emptyState(5), 2), 3)
    expect(s.selected).toBe(3)
  })

  it('нажатие на слово, у которого есть пара, разрывает её', () => {
    const s = selectWord(pair(emptyState(5), 0, 1), 0)
    expect(s.pairs[0]).toBeNull()
    expect(s.selected).toBeNull()
    expect(s.history).toEqual([])
  })

  it('перевод без выбранного слова ничего не делает', () => {
    const s = emptyState(5)
    expect(selectOption(s, 3)).toEqual(s)
  })

  it('нажатие на занятый перевод без выбранного слова разрывает его пару', () => {
    const s = selectOption(pair(emptyState(5), 0, 1), 1)
    expect(s.pairs[0]).toBeNull()
    expect(s.history).toEqual([])
  })

  it('занятый перевод можно перенести на выбранное слово: старая пара рвётся', () => {
    let s = pair(emptyState(5), 0, 1)
    s = pair(s, 3, 1)
    expect(s.pairs[0]).toBeNull()
    expect(s.pairs[3]).toBe(1)
    expect(s.history).toEqual([3])
  })

  it('неверные индексы игнорируются', () => {
    const s = emptyState(5)
    expect(selectWord(s, -1)).toEqual(s)
    expect(selectWord(s, 5)).toEqual(s)
    expect(selectOption(selectWord(s, 0), 7)).toEqual(selectWord(s, 0))
  })

  it('unpair разрывает пару слова, если она есть', () => {
    const s = unpair(pair(emptyState(5), 2, 4), 2)
    expect(s.pairs[2]).toBeNull()
    expect(unpair(s, 2)).toEqual(s)
  })

  it('undoLast рвёт последнюю собранную пару', () => {
    let s = pair(emptyState(5), 0, 1)
    s = pair(s, 4, 0)
    s = undoLast(s)
    expect(s.pairs[4]).toBeNull()
    expect(s.pairs[0]).toBe(1)
    expect(undoLast(undoLast(s)).pairs[0]).toBeNull()
    expect(undoLast(emptyState(5))).toEqual(emptyState(5))
  })

  it('undoLast без пар снимает выбор слова', () => {
    const s = undoLast(selectWord(emptyState(5), 1))
    expect(s.selected).toBeNull()
  })

  it('isComplete только при всех пяти парах', () => {
    let s = emptyState(5)
    for (let i = 0; i < 4; i++) s = pair(s, i, i)
    expect(isComplete(s)).toBe(false)
    s = pair(s, 4, 4)
    expect(isComplete(s)).toBe(true)
  })
})

describe('match: ответ и оформление', () => {
  it('toGiven собирает пары в порядке слов', () => {
    let s = emptyState(5)
    // apple→яблоко(1), house→дом(4), water→вода(3), book→книга(0), dog→собака(2); соединяем не по порядку
    s = pair(s, 4, 2)
    s = pair(s, 0, 1)
    s = pair(s, 3, 0)
    s = pair(s, 2, 3)
    s = pair(s, 1, 4)
    expect(toGiven(WORDS, OPTIONS, s)).toBe('apple=яблоко;house=дом;water=вода;book=книга;dog=собака')
  })

  it('toGiven у неполного ответа пропускает свободные слова', () => {
    const s = pair(emptyState(5), 1, 4)
    expect(toGiven(WORDS, OPTIONS, s)).toBe('house=дом')
  })

  it('цвета пар по номеру слова, пять разных', () => {
    expect([0, 1, 2, 3, 4].map(pairColor)).toEqual([0, 1, 2, 3, 4])
    expect(pairColor(5)).toBe(0)
  })

  it('подписи переводов a–e', () => {
    expect([0, 1, 2, 3, 4].map(optionLabel)).toEqual(['a', 'b', 'c', 'd', 'e'])
  })
})
