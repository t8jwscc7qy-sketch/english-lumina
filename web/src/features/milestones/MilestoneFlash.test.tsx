import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MilestoneFlash } from './MilestoneFlash'
import type { Milestone } from './milestones'

const m = (id: string, kind: string, title: string, threshold: number): Milestone => ({
  id,
  kind,
  title,
  threshold,
  reachedOn: '2026-10-25',
  progress: { value: threshold, target: threshold },
})
const LIST = [
  m('streak-7', 'streak', 'Серия 7 дней', 7),
  m('hours-5', 'hours', '5 часов занятий', 5),
  m('first-card', 'first', 'Первая карточка', 1),
]

describe('MilestoneFlash', () => {
  it('без новых вех ничего не показывает', () => {
    const { container } = render(<MilestoneFlash milestones={LIST} ids={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('одна веха: «Новая веха» и название в области состояния', () => {
    render(<MilestoneFlash milestones={LIST} ids={['hours-5']} />)
    const status = screen.getByRole('status')
    expect(within(status).getByText('Новая веха')).toBeInTheDocument()
    expect(within(status).getByText('5 часов занятий')).toBeInTheDocument()
    expect(within(status).queryByText('Серия 7 дней')).not.toBeInTheDocument()
  })

  it('несколько вех: списком в порядке сервера, первая — главная', () => {
    render(<MilestoneFlash milestones={LIST} ids={['streak-7', 'hours-5', 'first-card']} />)
    const items = within(screen.getByRole('status')).getAllByRole('listitem')
    expect(items.map((li) => li.textContent)).toEqual([
      'Серия 7 дней',
      '5 часов занятий',
      'Первая карточка',
    ])
    expect(items[0]).toHaveAttribute('data-primary', 'true')
    expect(items[1]).not.toHaveAttribute('data-primary')
  })

  it('движение только там, где пользователь его не отключил (motion-safe), и на самой карточке, а не на тексте', () => {
    render(<MilestoneFlash milestones={LIST} ids={['hours-5']} />)
    expect(screen.getByRole('status').className).toMatch(/motion-safe:animate-/)
  })

  it('неизвестные id пропускаются', () => {
    const { container } = render(<MilestoneFlash milestones={LIST} ids={['nope']} />)
    expect(container).toBeEmptyDOMElement()
  })
})
