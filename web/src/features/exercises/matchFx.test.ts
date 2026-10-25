import { afterEach, describe, expect, it, vi } from 'vitest'
import { confetti, lastInput, prefersReducedMotion, sparkOffsets, threadPath, trackInput } from './matchFx'

describe('sparkOffsets', () => {
  it('n точек на окружности радиуса 12 с точностью до округления', () => {
    const pts = sparkOffsets(7, 4)
    expect(pts).toHaveLength(4)
    for (const p of pts) expect(Math.abs(Math.hypot(p.dx, p.dy) - 12)).toBeLessThan(1.5)
  })

  it('детерминирован; другой seed поворачивает искры', () => {
    expect(sparkOffsets(3, 4)).toEqual(sparkOffsets(3, 4))
    expect(sparkOffsets(3, 4)).not.toEqual(sparkOffsets(40, 4))
  })

  it('точки разные и расходятся в разные стороны', () => {
    const pts = sparkOffsets(0, 4)
    expect(new Set(pts.map((p) => `${p.dx},${p.dy}`)).size).toBe(4)
    expect(sparkOffsets(1, 0)).toEqual([])
  })
})

describe('confetti', () => {
  it('n штук внутри области, задержки до 200 мс, цвета из пяти', () => {
    const c = confetti(5, 12)
    expect(c).toHaveLength(12)
    for (const p of c) {
      expect(p.x).toBeGreaterThanOrEqual(0)
      expect(p.x).toBeLessThanOrEqual(100)
      expect(p.y).toBeGreaterThanOrEqual(0)
      expect(p.y).toBeLessThanOrEqual(30)
      expect(p.delay).toBeGreaterThanOrEqual(0)
      expect(p.delay).toBeLessThanOrEqual(200)
      expect([0, 1, 2, 3, 4]).toContain(p.color)
    }
  })

  it('детерминирован и меняется с seed; цвета не все одинаковые', () => {
    expect(confetti(9, 12)).toEqual(confetti(9, 12))
    expect(confetti(9, 12)).not.toEqual(confetti(10, 12))
    expect(new Set(confetti(9, 12).map((p) => p.color)).size).toBeGreaterThan(1)
  })
})

describe('threadPath', () => {
  const parse = (d: string) => d.match(/[MHV][^MHV]*/g)!.map((s) => ({ c: s[0], v: s.slice(1).trim() }))

  it('начинается и кончается в заданных точках, внутри только прямые отрезки', () => {
    const d = threadPath({ x: 100, y: 20 }, { x: 180, y: 90 }, 0)
    const segs = parse(d)
    expect(segs[0]).toEqual({ c: 'M', v: '100 20' })
    expect(segs.slice(1).every((s) => s.c === 'H' || s.c === 'V')).toBe(true)
    expect(segs[segs.length - 1]).toEqual({ c: 'H', v: '180' })
    expect(d).toContain('V90')
  })

  it('вертикаль идёт по сетке 4px и сдвигается с дорожкой, чтобы нити не слипались', () => {
    const xm = (lane: number) => Number(parse(threadPath({ x: 100, y: 0 }, { x: 180, y: 50 }, lane))[1].v)
    for (let l = 0; l < 5; l++) expect(xm(l) % 4).toBe(0)
    expect(new Set([0, 1, 2, 3, 4].map(xm)).size).toBe(5)
    for (let l = 0; l < 5; l++) {
      expect(xm(l)).toBeGreaterThanOrEqual(100)
      expect(xm(l)).toBeLessThanOrEqual(180)
    }
  })

  it('на одной высоте — одна прямая', () => {
    expect(threadPath({ x: 10, y: 40 }, { x: 90, y: 40 }, 2)).toBe('M10 40 H90')
  })

  it('узкая щель не вылезает за края', () => {
    const d = threadPath({ x: 100, y: 0 }, { x: 108, y: 60 }, 4)
    const mid = Number(parse(d)[1].v)
    expect(mid).toBeGreaterThanOrEqual(100)
    expect(mid).toBeLessThanOrEqual(108)
  })
})

describe('источник ввода', () => {
  it('помнит последнее действие: клавиатура или указатель', () => {
    trackInput()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
    expect(lastInput()).toBe('keyboard')
    window.dispatchEvent(new Event('pointerdown'))
    expect(lastInput()).toBe('pointer')
  })

  it('повторная установка слушателей не плодит их', () => {
    trackInput()
    trackInput()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }))
    expect(lastInput()).toBe('keyboard')
  })
})

describe('prefersReducedMotion', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('читает системную настройку; без matchMedia — «обычное движение»', () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce') }))
    expect(prefersReducedMotion()).toBe(true)
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    expect(prefersReducedMotion()).toBe(false)
    vi.stubGlobal('matchMedia', undefined)
    expect(prefersReducedMotion()).toBe(false)
  })
})
