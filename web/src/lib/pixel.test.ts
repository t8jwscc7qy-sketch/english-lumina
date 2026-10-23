import { describe, expect, it } from 'vitest'
import { artToRects, ditherCells, filledSegments, masteryLevel } from './pixel'

describe('artToRects', () => {
  it('склеивает соседние клетки одного цвета в строке в один прямоугольник', () => {
    const rects = artToRects({
      palette: { a: '#111111', b: '#222222' },
      rows: ['aab.a', '.bb..'],
    })
    expect(rects).toEqual([
      { x: 0, y: 0, w: 2, fill: '#111111' },
      { x: 2, y: 0, w: 1, fill: '#222222' },
      { x: 4, y: 0, w: 1, fill: '#111111' },
      { x: 1, y: 1, w: 2, fill: '#222222' },
    ])
  })

  it('пропускает пустые клетки и пустые строки', () => {
    expect(artToRects({ palette: {}, rows: ['...', ''] })).toEqual([])
  })

  it('бросает понятную ошибку, если символа нет в палитре', () => {
    expect(() => artToRects({ palette: { a: '#000000' }, rows: ['ab'] })).toThrow(/«b»/)
  })
})

describe('ditherCells', () => {
  it('шкала из пяти ступеней: 0, 1, 2, 3 и 4 закрашенных клетки из четырёх', () => {
    expect([0, 1, 2, 3, 4].map((l) => ditherCells(l as 0 | 1 | 2 | 3 | 4).length)).toEqual([
      0, 1, 2, 3, 4,
    ])
  })

  it('клетки лежат внутри плитки 2×2 и не повторяются', () => {
    for (const level of [1, 2, 3, 4] as const) {
      const cells = ditherCells(level)
      expect(new Set(cells.map(([x, y]) => `${x},${y}`)).size).toBe(cells.length)
      for (const [x, y] of cells) {
        expect(x === 0 || x === 1).toBe(true)
        expect(y === 0 || y === 1).toBe(true)
      }
    }
  })

  it('ступень 2 — шахматка (диагональ)', () => {
    expect(ditherCells(2)).toEqual([
      [0, 0],
      [1, 1],
    ])
  })
})

describe('masteryLevel', () => {
  it('переводит долю освоения 0…1 в ступень 0…4', () => {
    expect(masteryLevel(0)).toBe(0)
    expect(masteryLevel(0.1)).toBe(0)
    expect(masteryLevel(0.25)).toBe(1)
    expect(masteryLevel(0.5)).toBe(2)
    expect(masteryLevel(0.75)).toBe(3)
    expect(masteryLevel(1)).toBe(4)
  })

  it('обрезает значения вне диапазона и не ломается на NaN', () => {
    expect(masteryLevel(-3)).toBe(0)
    expect(masteryLevel(7)).toBe(4)
    expect(masteryLevel(Number.NaN)).toBe(0)
  })
})

describe('filledSegments', () => {
  it('считает закрашенные сегменты полоски опыта', () => {
    expect(filledSegments(4, 10, 10)).toBe(4)
    expect(filledSegments(0, 10, 10)).toBe(0)
    expect(filledSegments(10, 10, 10)).toBe(10)
    expect(filledSegments(5, 10, 8)).toBe(4)
  })

  it('не выходит за границы и терпит нулевой итог', () => {
    expect(filledSegments(99, 10, 10)).toBe(10)
    expect(filledSegments(-1, 10, 10)).toBe(0)
    expect(filledSegments(3, 0, 10)).toBe(0)
  })
})
