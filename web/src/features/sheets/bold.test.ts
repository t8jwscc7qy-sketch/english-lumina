import { describe, expect, it } from 'vitest'
import { splitBold } from './bold'

describe('splitBold', () => {
  it('выделяет форму', () => {
    expect(splitBold('I **went** home')).toEqual([
      { text: 'I ', bold: false },
      { text: 'went', bold: true },
      { text: ' home', bold: false },
    ])
  })
  it('без разметки и с незакрытыми звёздочками — обычный текст', () => {
    expect(splitBold('plain')).toEqual([{ text: 'plain', bold: false }])
    expect(splitBold('a **b')).toEqual([{ text: 'a **b', bold: false }])
    expect(splitBold('')).toEqual([])
  })
  it('HTML остаётся текстом', () => {
    expect(splitBold('<b>x</b> **y**')[0]).toEqual({ text: '<b>x</b> ', bold: false })
  })
})
