import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MilestonesBlock } from './MilestonesBlock'
import type { Milestone } from './milestones'

const m = (
  id: string,
  kind: string,
  title: string,
  threshold: number,
  reachedOn?: string,
  value = 0,
): Milestone => ({
  id,
  kind,
  title,
  threshold,
  ...(reachedOn ? { reachedOn } : {}),
  progress: { value: Math.min(value, threshold), target: threshold },
})

const LIST: Milestone[] = [
  m('streak-3', 'streak', 'Серия 3 дня', 3, '2026-10-19', 3),
  m('streak-7', 'streak', 'Серия 7 дней', 7, undefined, 5),
  m('hours-1', 'hours', '1 час занятий', 1, '2026-10-20', 1),
  m('hours-5', 'hours', '5 часов занятий', 5, undefined, 2.5),
  m('answers-100', 'answers', '100 ответов', 100, undefined, 33),
  m('first-card', 'first', 'Первая карточка', 1, '2026-10-18', 1),
  m('first-exercise', 'first', 'Первое упражнение', 1),
]

describe('MilestonesBlock', () => {
  it('заголовок и достигнутые вехи с датой, новые сверху', () => {
    render(<MilestonesBlock milestones={LIST} />)
    expect(screen.getByRole('heading', { name: 'Вехи' })).toBeInTheDocument()
    const reached = screen.getByRole('list', { name: 'Достигнутые вехи' })
    const items = within(reached)
      .getAllByRole('listitem')
      .map((li) => li.textContent)
    expect(items).toEqual([
      '1 час занятий · 20 октября',
      'Серия 3 дня · 19 октября',
      'Первая карточка · 18 октября',
    ])
  })

  it('ближайшие: полоса и «значение из порога», без полосы для «первого»', () => {
    render(<MilestonesBlock milestones={LIST} />)
    const next = screen.getByRole('list', { name: 'Ближайшие вехи' })
    expect(within(next).getByText('Серия 7 дней: 5 из 7')).toBeInTheDocument()
    expect(within(next).getByText('5 часов занятий: 2,5 из 5')).toBeInTheDocument()
    expect(within(next).getByText('100 ответов: 33 из 100')).toBeInTheDocument()
    expect(within(next).getByText('Первое упражнение: ещё впереди')).toBeInTheDocument()
    expect(within(next).getAllByRole('progressbar')).toHaveLength(3)
    expect(within(next).getByRole('progressbar', { name: 'Серия 7 дней' })).toHaveAttribute(
      'aria-valuenow',
      '5',
    )
  })

  it('ничего не достигнуто: подсказка вместо пустого списка, ближайшие остаются', () => {
    const none = LIST.map((x) => ({
      ...x,
      reachedOn: undefined,
      progress: { ...x.progress, value: 0 },
    }))
    render(<MilestonesBlock milestones={none} />)
    expect(screen.getByText(/первая веха ждёт тебя после занятий/i)).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Достигнутые вехи' })).not.toBeInTheDocument()
    expect(screen.getByRole('list', { name: 'Ближайшие вехи' })).toBeInTheDocument()
  })

  it('всё достигнуто: ближайших нет', () => {
    const all = LIST.map((x) => ({
      ...x,
      reachedOn: '2026-10-01',
      progress: { value: x.threshold, target: x.threshold },
    }))
    render(<MilestonesBlock milestones={all} />)
    expect(screen.queryByRole('list', { name: 'Ближайшие вехи' })).not.toBeInTheDocument()
  })

  it('недостигнутая веха не показывает «5 из 5»: значение округляется вниз до десятых', () => {
    const almost = [m('hours-5', 'hours', '5 часов занятий', 5, undefined, 4.95)]
    render(<MilestonesBlock milestones={almost} />)
    expect(screen.getByText('5 часов занятий: 4,9 из 5')).toBeInTheDocument()
    expect(screen.queryByText('5 часов занятий: 5 из 5')).not.toBeInTheDocument()
  })
})
