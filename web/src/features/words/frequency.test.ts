import { describe, expect, it } from 'vitest'
import { describeFrequency } from './frequency'

const norm = (s: string) => s.replace(/\s/g, ' ') // обычные и неразрывные пробелы считаем одинаковыми

describe('describeFrequency', () => {
  it.each([
    [1, 'очень частое слово, № 1 из 50 000'],
    [870, 'очень частое слово, № 870 из 50 000'],
    [1000, 'очень частое слово, № 1 000 из 50 000'],
    [1001, 'частое слово, № 1 001 из 50 000'],
    [5000, 'частое слово, № 5 000 из 50 000'],
    [5001, 'обычное слово, № 5 001 из 50 000'],
    [20000, 'обычное слово, № 20 000 из 50 000'],
    [20001, 'редкое слово, № 20 001 из 50 000'],
    [50000, 'редкое слово, № 50 000 из 50 000'],
  ])('место %i → «%s»', (rank, text) => {
    expect(norm(describeFrequency(rank))).toBe(text)
  })

  it('слова нет в списке: редкое, реже последнего места', () => {
    expect(norm(describeFrequency(undefined))).toBe('редкое слово, реже № 50 000')
  })

  it('фраза без места: частоты для фраз нет, а не «редкое»', () => {
    expect(describeFrequency(undefined, 'look after')).toBe('для фраз частоты нет')
  })
})
