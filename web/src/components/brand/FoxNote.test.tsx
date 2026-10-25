import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FoxNote } from './FoxNote'

describe('FoxNote', () => {
  it('показывает текст, а лисёнок декоративный с нужным настроением', () => {
    const { container } = render(
      <FoxNote mood="sleeping">
        <p>Пока пусто</p>
      </FoxNote>,
    )
    expect(screen.getByText('Пока пусто')).toBeInTheDocument()
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(container.querySelector('[data-mood]')).toHaveAttribute('data-mood', 'sleeping')
  })

  it('по умолчанию бодрый', () => {
    const { container } = render(<FoxNote>текст</FoxNote>)
    expect(container.querySelector('[data-mood]')).toHaveAttribute('data-mood', 'neutral')
  })
})
