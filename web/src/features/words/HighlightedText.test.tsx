import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { HighlightedText } from './HighlightedText'

describe('HighlightedText', () => {
  it('выделяет диапазон в <mark>, остальное оставляет текстом', () => {
    const { container } = render(<HighlightedText text="I like to run every morning." range={{ start: 10, end: 13 }} />)
    expect(container.textContent).toBe('I like to run every morning.')
    const marks = container.querySelectorAll('mark')
    expect(marks).toHaveLength(1)
    expect(marks[0].textContent).toBe('run')
  })

  it('диапазон считается в символах, а не в единицах UTF-16 (эмодзи занимает две)', () => {
    const { container } = render(<HighlightedText text="😀 run fast" range={{ start: 2, end: 5 }} />)
    expect(container.querySelector('mark')?.textContent).toBe('run')
    expect(container.textContent).toBe('😀 run fast')
  })

  it('без диапазона — просто текст', () => {
    const { container } = render(<HighlightedText text="No match here" />)
    expect(container.querySelector('mark')).toBeNull()
    expect(container.textContent).toBe('No match here')
  })

  it('кривой диапазон не ломает вывод', () => {
    for (const range of [{ start: 5, end: 3 }, { start: -1, end: 4 }, { start: 100, end: 200 }]) {
      const { container, unmount } = render(<HighlightedText text="short" range={range} />)
      expect(container.textContent).toBe('short')
      expect(container.querySelector('mark')).toBeNull()
      unmount()
    }
  })

  it('разметка в тексте остаётся текстом', () => {
    const { container } = render(<HighlightedText text="<b>x</b> run" range={{ start: 8, end: 11 }} />)
    expect(container.querySelector('b')).toBeNull()
    expect(container.textContent).toBe('<b>x</b> run')
  })
})
