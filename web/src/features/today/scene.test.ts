import { describe, expect, it } from 'vitest'
import { buildScene, FIRE_AT, FOX_AT, phaseOfHour, SCENE_H, SCENE_W } from './scene'

describe('phaseOfHour', () => {
  it('делит сутки: ночь до 5 и с 21, утро, день, вечер', () => {
    const cases: [number, string][] = [[0, 'night'], [4, 'night'], [5, 'morning'], [8, 'morning'], [9, 'day'], [16, 'day'], [17, 'dusk'], [20, 'dusk'], [21, 'night'], [23, 'night']]
    for (const [h, p] of cases) expect(phaseOfHour(h)).toBe(p)
  })
  it('терпит выход за 0–23', () => {
    expect(phaseOfHour(24)).toBe('night')
    expect(phaseOfHour(-1)).toBe('night')
  })
})

describe('buildScene', () => {
  const phases = ['morning', 'day', 'dusk', 'night'] as const

  it('все прямоугольники внутри сетки', () => {
    for (const ph of phases) {
      for (const r of buildScene(ph)) {
        expect(r.x).toBeGreaterThanOrEqual(0)
        expect(r.y).toBeGreaterThanOrEqual(0)
        expect(r.x + r.w).toBeLessThanOrEqual(SCENE_W)
        expect(r.y + r.h).toBeLessThanOrEqual(SCENE_H)
      }
    }
  })

  it('каждая клетка неба и земли закрашена: без дыр в картинке', () => {
    for (const ph of phases) {
      const covered = new Set<string>()
      for (const r of buildScene(ph).filter((x) => x.opacity === undefined))
        for (let k = 0; k < r.w; k++) covered.add(`${r.x + k},${r.y}`)
      expect(covered.size).toBe(SCENE_W * SCENE_H)
    }
  })

  it('сцена детерминирована', () => {
    expect(buildScene('night')).toEqual(buildScene('night'))
  })

  it('время суток меняет картинку; звёзды только вечером и ночью', () => {
    const stars = (ph: (typeof phases)[number]) => buildScene(ph).filter((r) => r.fill === '#f7f7ff' && r.y < 16).length
    expect(stars('night')).toBeGreaterThan(stars('dusk'))
    expect(stars('dusk')).toBeGreaterThan(0)
    expect(stars('day')).toBe(0)
    expect(JSON.stringify(buildScene('day'))).not.toBe(JSON.stringify(buildScene('night')))
  })

  it('свет костра днём не рисуется, ночью рисуется', () => {
    const glow = (ph: (typeof phases)[number]) => buildScene(ph).filter((r) => r.opacity !== undefined).length
    expect(glow('day')).toBe(0)
    expect(glow('night')).toBeGreaterThan(0)
  })

  it('лисёнок и костёр помещаются в сетку', () => {
    expect(FOX_AT.x + 28).toBeLessThanOrEqual(SCENE_W)
    expect(FOX_AT.y + 25).toBeLessThanOrEqual(SCENE_H)
    expect(FIRE_AT.x + 7).toBeLessThanOrEqual(SCENE_W)
    expect(FIRE_AT.y + 8).toBeLessThanOrEqual(SCENE_H)
  })
})
