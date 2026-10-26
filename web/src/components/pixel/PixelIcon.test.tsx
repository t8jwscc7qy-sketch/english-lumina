import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ICON_NAMES, ICON_SIZE, iconArt } from './icons'
import { PixelIcon } from './PixelIcon'

describe('пиксельные иконки', () => {
  it('их девять, все 12×12 и не пустые', () => {
    expect(ICON_NAMES).toHaveLength(9)
    for (const name of ICON_NAMES) {
      const { rows } = iconArt(name)
      expect(rows, name).toHaveLength(ICON_SIZE)
      for (const r of rows) expect(r, name).toHaveLength(ICON_SIZE)
      expect(rows.join('').includes('X'), name).toBe(true)
    }
  })

  it('используют только X и точку, чтобы цвет задавал текст', () => {
    for (const name of ICON_NAMES) expect(iconArt(name).rows.join('')).toMatch(/^[X.]+$/)
  })

  it('все разные', () => {
    expect(new Set(ICON_NAMES.map((n) => iconArt(n).rows.join('|'))).size).toBe(ICON_NAMES.length)
  })

  it('декоративна и берёт размер из масштаба', () => {
    const { container } = render(<PixelIcon name="cards" scale={2} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).toHaveAttribute('width', '24')
  })
})
