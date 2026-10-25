import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Fox } from './Fox'
import { foxArt } from './foxSprite'

describe('Fox', () => {
  it('без title декоративный: скрыт от скринридеров', () => {
    const { container } = render(<Fox />)
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })

  it('с title читается как картинка', () => {
    const { getByRole } = render(<Fox title="Вёрст" />)
    expect(getByRole('img', { name: 'Вёрст' })).toBeInTheDocument()
  })

  it('размер клетки задаёт размер: 28 × 25 клеток', () => {
    const { container } = render(<Fox scale={5} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('width', '140')
    expect(svg).toHaveAttribute('height', '125')
  })

  it('все настроения одной сетки 28×25 и различаются лицом', () => {
    const moods = ['neutral', 'happy', 'thinking', 'sleeping'] as const
    const rows = moods.map((m) => foxArt(m).rows)
    for (const r of rows) {
      expect(r).toHaveLength(25)
      for (const line of r) expect(line).toHaveLength(28)
    }
    expect(new Set(rows.map((r) => r.join('|'))).size).toBe(4)
  })

  it('сон рисует букву Z, остальные настроения нет', () => {
    const hasZ = (m: 'neutral' | 'sleeping') => foxArt(m).rows.slice(1, 6).some((r) => r.slice(22).includes('B'))
    expect(hasZ('sleeping')).toBe(true)
    expect(hasZ('neutral')).toBe(false)
  })

  it('покачивание можно выключить', () => {
    const { container } = render(<Fox bob={false} />)
    expect(container.firstElementChild).not.toHaveClass('fox-bob')
  })
})
