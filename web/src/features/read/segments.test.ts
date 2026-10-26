import { describe, expect, it } from 'vitest'
import { segment, wordAtChar } from './segments'

describe('segment', () => {
  it('режет предложение на слова и промежутки', () => {
    expect(segment("I don't know, well-known!")).toEqual([
      { text: 'I', word: true, key: 'i' },
      { text: ' ', word: false, key: '' },
      { text: "don't", word: true, key: "don't" },
      { text: ' ', word: false, key: '' },
      { text: 'know', word: true, key: 'know' },
      { text: ', ', word: false, key: '' },
      { text: 'well-known', word: true, key: 'well-known' },
      { text: '!', word: false, key: '' },
    ])
  })
  it('приводит ’ к обычному апострофу в ключе, текст не меняет', () => {
    const w = segment('It’s here').find((s) => s.word)
    expect(w).toEqual({ text: 'It’s', word: true, key: "it's" })
  })
  it('не падает на пустых и странных строках', () => {
    expect(segment('')).toEqual([])
    expect(segment('... !!')).toEqual([{ text: '... !!', word: false, key: '' }])
    expect(segment('<b>x</b>').map((s) => s.text).join('')).toBe('<b>x</b>')
  })
})

describe('wordAtChar', () => {
  it('находит номер слова по символу', () => {
    expect(wordAtChar('I like tea', 0)).toBe(0)
    expect(wordAtChar('I like tea', 2)).toBe(1)
    expect(wordAtChar('I like tea', 7)).toBe(2)
    expect(wordAtChar('I like tea', 1)).toBe(-1)
    expect(wordAtChar('I like tea', 99)).toBe(-1)
  })
})
