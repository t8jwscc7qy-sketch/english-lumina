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
  it('нижний регистр, без знаков и лишних пробелов', () => {
    expect(lightNorm('  We EAT,  red apples! ')).toBe('we eat red apples')
  })
  it('как на сервере: однозначные сокращения раскрываются, остальные нет', () => {
    expect(lightNorm('I don’t know.')).toBe('i do not know')
    expect(lightNorm("I'm sure they're here, we'll go")).toBe('i am sure they are here we will go')
    expect(lightNorm("He won't go and can't stay, cannot wait, shan't")).toBe('he will not go and can not stay can not wait shall not')
    expect(lightNorm("It's his and he'd go")).toBe("it's his and he'd go")
  })
  it('дефис как пробел, разные апострофы', () => {
    expect(lightNorm('A well-known fact')).toBe('a well known fact')
    expect(lightNorm('I don`t, I donʼt, I don´t')).toBe('i do not i do not i do not')
  })
})

describe('подсветка при сокращениях', () => {
  it("«I'm» в ответе и «I am» в эталоне с опечаткой: место найдено", () => {
    const given = lightNorm("I'm tired of aples")
    const expected = lightNorm('I am tired of apples')
    expect(diffOneEdit(given, expected)).toEqual({ start: 16, end: 16, missing: 'p' })
  })
})
