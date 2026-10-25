import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Fox } from './Fox'

describe('Fox', () => {
  it('без title декоративный: скрыт от скринридеров', () => {
    const { container } = render(<Fox />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).not.toHaveAttribute('role')
  })

  it('с title читается как картинка', () => {
    const { getByRole } = render(<Fox title="Вёрст" />)
    expect(getByRole('img', { name: 'Вёрст' })).toBeInTheDocument()
  })

  it('размер задаёт ширину и высоту', () => {
    const { container } = render(<Fox size={96} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('width', '96')
    expect(svg).toHaveAttribute('height', '96')
  })

  it('у каждого настроения свои признаки: улыбка, сон, брови', () => {
    const marks = (mood: 'neutral' | 'happy' | 'thinking' | 'sleeping') => {
      const { container } = render(<Fox mood={mood} />)
      return {
        blink: container.querySelectorAll('.fox-blink').length,
        zzz: container.querySelectorAll('.fox-zzz').length,
        tongue: container.querySelectorAll('ellipse[fill="var(--coral-soft)"]').length,
        brows: container.querySelectorAll('g[stroke="var(--fox-dark)"] path').length,
      }
    }
    expect(marks('neutral')).toMatchObject({ blink: 1, zzz: 0, tongue: 0, brows: 0 })
    expect(marks('happy')).toMatchObject({ blink: 0, tongue: 1 })
    expect(marks('thinking')).toMatchObject({ blink: 1, brows: 2 })
    expect(marks('sleeping')).toMatchObject({ blink: 0, zzz: 2 })
  })

  it('две лисы на странице не делят id градиентов', () => {
    const { container } = render(
      <>
        <Fox />
        <Fox />
      </>,
    )
    const ids = Array.from(container.querySelectorAll('[id]')).map((e) => e.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
