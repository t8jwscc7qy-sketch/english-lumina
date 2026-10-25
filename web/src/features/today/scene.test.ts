import { describe, expect, it } from 'vitest'
import { buildScene, fireFrames, fireLevel, FIRE_BASE_Y, FIRE_CX, FOX_AT, logCount, phaseOfHour, SCENE_H, SCENE_W } from './scene'

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
    expect(FIRE_BASE_Y).toBeLessThanOrEqual(SCENE_H)
  })
})

describe('костёр по серии', () => {
  it('ступени: нет серии — угли, 1–2 — малый, 3–6 — средний, 7–13 — большой, 14+ — с искрами', () => {
    const cases: [number, number][] = [[0, 0], [1, 1], [2, 1], [3, 2], [6, 2], [7, 3], [13, 3], [14, 4], [100, 4]]
    for (const [streak, level] of cases) expect(fireLevel(streak, false)).toBe(level)
  })

  it('выполненный минимум поднимает ступень на одну, но не выше 4; без серии не зажигает', () => {
    expect(fireLevel(1, true)).toBe(2)
    expect(fireLevel(7, true)).toBe(4)
    expect(fireLevel(30, true)).toBe(4)
    expect(fireLevel(0, true)).toBe(0)
  })

  it('мусор не ломает: отрицательная и дробная серия', () => {
    expect(fireLevel(-3, false)).toBe(0)
    expect(fireLevel(Number.NaN, false)).toBe(0)
    expect(fireLevel(2.9, false)).toBe(1)
  })

  it('кадры каждой ступени одного размера, пламя растёт со ступенью, низ ровный', () => {
    let prev = 0
    for (const level of [0, 1, 2, 3, 4] as const) {
      const [a, b] = fireFrames(level)
      expect(a.rows.length).toBe(b.rows.length)
      expect(a.rows[0].length).toBe(b.rows[0].length)
      expect(a.rows.length).toBeGreaterThanOrEqual(prev)
      prev = a.rows.length
      expect(FIRE_CX - Math.floor(a.rows[0].length / 2)).toBeGreaterThanOrEqual(0)
      for (const r of [...a.rows, ...b.rows]) expect(r.length).toBe(a.rows[0].length)
    }
  })

  it('свет костра растёт со ступенью и не рисуется днём и при углях', () => {
    const glow = (ph: 'night' | 'day', level: 0 | 1 | 2 | 3 | 4) =>
      buildScene(ph, { fireLevel: level }).filter((r) => r.opacity !== undefined).reduce((n, r) => n + r.w, 0)
    expect(glow('night', 0)).toBe(0)
    expect(glow('night', 1)).toBeGreaterThan(0)
    expect(glow('night', 4)).toBeGreaterThan(glow('night', 2))
    expect(glow('night', 2)).toBeGreaterThan(glow('night', 1))
    expect(glow('day', 4)).toBe(0)
  })
})

describe('поленница', () => {
  it('одно полено на день серии, не больше 10', () => {
    expect(logCount(0)).toBe(0)
    expect(logCount(4)).toBe(4)
    expect(logCount(10)).toBe(10)
    expect(logCount(99)).toBe(10)
    expect(logCount(-2)).toBe(0)
  })

  it('число клеток поленьев растёт с серией', () => {
    const cells = (n: number) => buildScene('day', { logs: n }).filter((r) => r.fill === '#c48a52').length
    expect(cells(0)).toBe(0)
    expect(cells(1)).toBeGreaterThan(0)
    expect(cells(10)).toBeGreaterThan(cells(5))
  })
})

describe('вехи вдоль дороги', () => {
  const count = (fill: string, opts: Parameters<typeof buildScene>[1]) =>
    buildScene('day', opts).filter((r) => r.fill === fill).reduce((n, r) => n + r.w * r.h, 0)

  it('достигнутые рисуются жёлтыми полосами, больше вех — больше жёлтого', () => {
    expect(count('#ffd23f', { posts: 0 })).toBe(0)
    expect(count('#ffd23f', { posts: 3 })).toBeGreaterThan(0)
    expect(count('#ffd23f', { posts: 7 })).toBeGreaterThan(count('#ffd23f', { posts: 3 }))
  })

  it('ближайшая недостигнутая серая, и только если она есть', () => {
    expect(count('#9aa3c7', { nextPost: false })).toBe(0)
    expect(count('#9aa3c7', { nextPost: true })).toBeGreaterThan(0)
  })

  it('больше 7 вех не рисуется: берутся последние', () => {
    expect(count('#ffd23f', { posts: 50 })).toBe(count('#ffd23f', { posts: 7 }))
  })

  it('вехи и поленья внутри сетки при любых числах', () => {
    for (const r of buildScene('night', { posts: 7, nextPost: true, logs: 10, fireLevel: 4 })) {
      expect(r.x).toBeGreaterThanOrEqual(0)
      expect(r.x + r.w).toBeLessThanOrEqual(SCENE_W)
      expect(r.y).toBeGreaterThanOrEqual(0)
      expect(r.y + r.h).toBeLessThanOrEqual(SCENE_H)
    }
  })
})
