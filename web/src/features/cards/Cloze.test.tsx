import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Cloze } from './Cloze'

describe('Cloze', () => {
  it('скрывает слово по диапазону и ставит пропуск', () => {
    const { container } = render(<Cloze text="I like to run every morning." gap={{ start: 10, end: 13 }} revealed={false} />)
    expect(container.textContent).toBe('I like to _____ every morning.')
    expect(screen.getByRole('img', { name: 'пропуск' })).toBeInTheDocument()
    expect(container.querySelector('mark')).toBeNull()
  })

  it('на обороте показывает предложение целиком и выделяет слово', () => {
    const { container } = render(<Cloze text="I like to run every morning." gap={{ start: 10, end: 13 }} revealed />)
    expect(container.textContent).toBe('I like to run every morning.')
    expect(container.querySelector('mark')?.textContent).toBe('run')
  })

  it('диапазон считается в символах: эмодзи до слова ничего не сдвигает', () => {
    const { container, rerender } = render(<Cloze text="😀 I run fast" gap={{ start: 4, end: 7 }} revealed={false} />)
    expect(container.textContent).toBe('😀 I _____ fast')
    rerender(<Cloze text="😀 I run fast" gap={{ start: 4, end: 7 }} revealed />)
    expect(container.querySelector('mark')?.textContent).toBe('run')
  })

  it('кривой диапазон не ломает вывод и ничего не скрывает', () => {
    for (const gap of [{ start: 5, end: 3 }, { start: -1, end: 4 }, { start: 100, end: 200 }]) {
      const { container, unmount } = render(<Cloze text="short text" gap={gap} revealed={false} />)
      expect(container.textContent).toBe('short text')
      unmount()
    }
  })

  it('разметка в тексте остаётся текстом', () => {
    const { container } = render(<Cloze text="<b>x</b> run" gap={{ start: 9, end: 12 }} revealed={false} />)
    expect(container.querySelector('b')).toBeNull()
    expect(container.textContent).toBe('<b>x</b> _____')
  })
})
