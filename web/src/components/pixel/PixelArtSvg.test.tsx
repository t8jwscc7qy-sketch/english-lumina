import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PixelArtSvg } from './PixelArtSvg'

const art = { palette: { a: '#111111', b: '#222222' }, rows: ['aab', '.b.'] }

describe('PixelArtSvg', () => {
  it('рисует картинку целыми шагами масштаба', () => {
    const { container } = render(<PixelArtSvg art={art} scale={4} title="Знак" />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('width', '12')
    expect(svg).toHaveAttribute('height', '8')
    expect(svg).toHaveAttribute('shape-rendering', 'crispEdges')
  })

  it('с названием — картинка с подписью для вспомогательных технологий', () => {
    render(<PixelArtSvg art={art} title="Маскот Верста" />)
    expect(screen.getByRole('img', { name: 'Маскот Верста' })).toBeInTheDocument()
  })

  it('без названия — декоративная, скрыта от вспомогательных технологий', () => {
    const { container } = render(<PixelArtSvg art={art} />)
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })
})
