import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from './button'

describe('Button', () => {
  it('главная, вторичная, акцентная и опасная кнопки в пиксельной рамке и вдавливаются', () => {
    render(
      <>
        <Button>a</Button>
        <Button variant="secondary">b</Button>
        <Button variant="accent">c</Button>
        <Button variant="destructive">d</Button>
      </>,
    )
    for (const name of ['a', 'b', 'c', 'd']) {
      expect(screen.getByRole('button', { name })).toHaveClass('pixel-frame', 'pixel-press')
    }
  })

  it('прозрачные варианты без рамки', () => {
    render(
      <>
        <Button variant="ghost">g</Button>
        <Button variant="link">l</Button>
      </>,
    )
    expect(screen.getByRole('button', { name: 'g' })).not.toHaveClass('pixel-frame')
    expect(screen.getByRole('button', { name: 'l' })).not.toHaveClass('pixel-frame')
  })

  it('работает как обычная кнопка', async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>ok</Button>)
    await userEvent.click(screen.getByRole('button', { name: 'ok' }))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('недоступная не нажимается', async () => {
    const onClick = vi.fn()
    render(<Button disabled onClick={onClick}>no</Button>)
    await userEvent.click(screen.getByRole('button', { name: 'no' }))
    expect(onClick).not.toHaveBeenCalled()
  })
})
