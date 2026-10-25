import { describe, expect, it } from 'vitest'
import { nextPerKind, sortNew, type Milestone } from './milestones'

const m = (
  id: string,
  kind: string,
  threshold: number,
  reachedOn?: string,
  value = 0,
): Milestone => ({
  id,
  kind,
  title: id,
  threshold,
  ...(reachedOn ? { reachedOn } : {}),
  progress: { value: Math.min(value, threshold), target: threshold },
})

describe('nextPerKind', () => {
  it('по одной ближайшей недостигнутой на линейку серия, часы, ответы; недостигнутые «первое» все', () => {
    const list = [
      m('streak-3', 'streak', 3, '2026-10-20', 3),
      m('streak-7', 'streak', 7, undefined, 5),
      m('streak-14', 'streak', 14, undefined, 5),
      m('hours-1', 'hours', 1, '2026-10-20', 1),
      m('hours-5', 'hours', 5, undefined, 2),
      m('answers-100', 'answers', 100, undefined, 40),
      m('first-card', 'first', 1, '2026-10-19', 1),
      m('first-exercise', 'first', 1),
      m('first-mistake-closed', 'first', 1),
    ]
    expect(nextPerKind(list).map((x) => x.id)).toEqual([
      'streak-7',
      'hours-5',
      'answers-100',
      'first-exercise',
      'first-mistake-closed',
    ])
  })

  it('линейка полностью достигнута: ближайшей нет', () => {
    const list = [
      m('streak-3', 'streak', 3, '2026-10-20', 3),
      m('hours-1', 'hours', 1, '2026-10-20', 1),
    ]
    expect(nextPerKind(list)).toEqual([])
  })
})

describe('sortNew', () => {
  it('порядок как у сервера (в ids), неизвестные id пропускаются', () => {
    const list = [
      m('streak-7', 'streak', 7, '2026-10-25', 7),
      m('hours-5', 'hours', 5, '2026-10-25', 5),
      m('answers-100', 'answers', 100, '2026-10-25', 100),
    ]
    expect(sortNew(list, ['hours-5', 'nope', 'streak-7']).map((x) => x.id)).toEqual([
      'hours-5',
      'streak-7',
    ])
  })
  it('пустые ids — пусто', () => {
    expect(sortNew([m('streak-7', 'streak', 7)], [])).toEqual([])
  })
})
