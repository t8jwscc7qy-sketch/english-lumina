import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { XpBar } from './XpBar'

describe('XpBar', () => {
  it('сообщает значение вспомогательным технологиям', () => {
    render(<XpBar value={4} total={10} label="Минимальный день" />)
    const bar = screen.getByRole('progressbar', { name: 'Минимальный день' })
    expect(bar).toHaveAttribute('aria-valuenow', '4')
    expect(bar).toHaveAttribute('aria-valuemin', '0')
    expect(bar).toHaveAttribute('aria-valuemax', '10')
  })

  it('закрашивает долю сегментов', () => {
    const { container } = render(<XpBar value={4} total={10} label="Опыт" segments={10} />)
    expect(container.querySelectorAll('[data-filled="true"]')).toHaveLength(4)
    expect(container.querySelectorAll('[data-filled="false"]')).toHaveLength(6)
  })

  it('значение выше итога не ломает полоску', () => {
    const { container } = render(<XpBar value={50} total={10} label="Опыт" segments={8} />)
    expect(container.querySelectorAll('[data-filled="true"]')).toHaveLength(8)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '10')
  })
})
