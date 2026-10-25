import { describe, expect, it } from 'vitest'
import { diffOneEdit, lightNorm } from './translateDiff'

describe('diffOneEdit', () => {
  it('замена одной буквы', () => {
    expect(diffOneEdit('we eat red applez', 'we eat red apples')).toEqual({ start: 16, end: 17 })
  })
  it('лишняя буква в ответе', () => {
    expect(diffOneEdit('we eat red appples', 'we eat red apples')).toEqual({ start: 14, end: 15 })
  })
  it('пропущенная буква: указано место и недостающая буква', () => {
    expect(diffOneEdit('we eat red aples', 'we eat red apples')).toEqual({ start: 13, end: 13, missing: 'p' })
  })
  it('перестановка соседних букв', () => {
    expect(diffOneEdit('we eat red appels', 'we eat red apples')).toEqual({ start: 14, end: 16 })
  })
  it('лишняя или пропущенная буква в конце', () => {
    expect(diffOneEdit('applesx', 'apples')).toEqual({ start: 6, end: 7 })
    expect(diffOneEdit('apple', 'apples')).toEqual({ start: 5, end: 5, missing: 's' })
  })
  it('нет отличий или больше одной правки: null', () => {
    expect(diffOneEdit('same', 'same')).toBeNull()
    expect(diffOneEdit('we eat red aples evry', 'we eat red apples every')).toBeNull()
    expect(diffOneEdit('abc', 'abcde')).toBeNull()
    expect(diffOneEdit('', '')).toBeNull()
  })
  it('символы вне BMP считаются одним символом', () => {
    expect(diffOneEdit('a😀c', 'a😁c')).toEqual({ start: 1, end: 2 })
  })
})

describe('lightNorm', () => {
  it('нижний регистр, апостроф, без знаков и лишних пробелов', () => {
    expect(lightNorm('  We EAT,  red apples! ')).toBe('we eat red apples')
    expect(lightNorm('I don’t know.')).toBe("i don't know")
  })
})
