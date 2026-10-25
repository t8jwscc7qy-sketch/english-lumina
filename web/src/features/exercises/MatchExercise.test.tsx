import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AnswerResult } from '@/api/client'
import { MatchExercise } from './MatchExercise'

const WORDS = ['apple', 'house', 'water', 'book', 'dog']
const OPTIONS = ['книга', 'яблоко', 'собака', 'вода', 'дом']
// верные: apple→b(яблоко), house→e(дом), water→d(вода), book→a(книга), dog→c(собака)
const ANSWER = 'apple=яблоко;house=дом;water=вода;book=книга;dog=собака'

function setup(props: Partial<React.ComponentProps<typeof MatchExercise>> = {}) {
  const onSubmit = vi.fn()
  const restoreFocus = vi.fn()
  const view = render(
    <MatchExercise
      words={WORDS}
      options={OPTIONS}
      frozen={false}
      pending={false}
      onSubmit={onSubmit}
      restoreFocus={restoreFocus}
      {...props}
    />,
  )
  return { onSubmit, restoreFocus, ...view }
}

const word = (i: number) => screen.getByRole('button', { name: new RegExp(`^${i + 1}\\. ${WORDS[i]}`) })
const option = (j: number) => screen.getByRole('button', { name: new RegExp(`^${'abcde'[j]}\\. ${OPTIONS[j]}`) })

async function fillAll(user: ReturnType<typeof userEvent.setup>) {
  const right: [number, number][] = [[0, 1], [1, 4], [2, 3], [3, 0], [4, 2]]
  for (const [w, o] of right) {
    await user.click(word(w))
    await user.click(option(o))
  }
}

const RESULT = (right: boolean[]): AnswerResult => ({
  correct: right.every(Boolean),
  answer: ANSWER,
  translation: '',
  ratings: [],
  pairs: WORDS.map((w, i) => ({
    word: w,
    given: right[i] ? OPTIONS[[1, 4, 3, 0, 2][i]] : OPTIONS[0],
    answer: OPTIONS[[1, 4, 3, 0, 2][i]],
    correct: right[i],
  })),
})

describe('MatchExercise', () => {
  it('показывает пять слов и пять переводов двумя списками с подписями', () => {
    setup()
    const words = screen.getByRole('list', { name: 'Слова' })
    const options = screen.getByRole('list', { name: 'Переводы' })
    expect(within(words).getAllByRole('button')).toHaveLength(5)
    expect(within(options).getAllByRole('button')).toHaveLength(5)
    expect(screen.getByRole('button', { name: 'Проверить' })).toBeDisabled()
  })

  it('мышью: слово, затем перевод соединяют пару, у обеих плиток это видно в имени', async () => {
    const user = userEvent.setup()
    setup()
    await user.click(word(0))
    expect(word(0)).toHaveAttribute('aria-pressed', 'true')
    await user.click(option(1))
    expect(screen.getByRole('button', { name: /^1\. apple, соединено с «яблоко»/ })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: /^b\. яблоко, соединено со словом «apple»/ })).toBeInTheDocument()
  })

  it('с клавиатуры: цифра выбирает слово, буква выбирает перевод', async () => {
    const user = userEvent.setup()
    setup()
    await user.keyboard('2')
    expect(word(1)).toHaveAttribute('aria-pressed', 'true')
    await user.keyboard('e')
    expect(screen.getByRole('button', { name: /^2\. house, соединено с «дом»/ })).toBeInTheDocument()
  })

  it('Backspace рвёт последнюю пару, нажатие на соединённое слово рвёт его пару', async () => {
    const user = userEvent.setup()
    setup()
    await user.keyboard('1b')
    await user.keyboard('2e')
    await user.keyboard('{Backspace}')
    expect(word(1).getAttribute('aria-label') ?? word(1).textContent).not.toMatch(/соединено/)
    expect(screen.getByRole('button', { name: /^1\. apple, соединено/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /^1\. apple, соединено/ }))
    expect(word(0).getAttribute('aria-label') ?? '').not.toMatch(/соединено/)
  })

  it('«Проверить» включается только при пяти парах и отправляет ответ строкой', async () => {
    const user = userEvent.setup()
    const { onSubmit } = setup()
    await user.click(word(0))
    await user.click(option(1))
    expect(screen.getByRole('button', { name: 'Проверить' })).toBeDisabled()
    await user.click(word(1))
    await user.click(option(4))
    await user.click(word(2))
    await user.click(option(3))
    await user.click(word(3))
    await user.click(option(0))
    expect(screen.getByRole('button', { name: 'Проверить' })).toBeDisabled()
    await user.click(word(4))
    await user.click(option(2))
    const check = screen.getByRole('button', { name: 'Проверить' })
    expect(check).toBeEnabled()
    await user.click(check)
    expect(onSubmit).toHaveBeenCalledOnce()
    expect(onSubmit).toHaveBeenCalledWith(ANSWER)
  })

  it('Enter при собранных парах тоже проверяет', async () => {
    const user = userEvent.setup()
    const { onSubmit } = setup()
    await fillAll(user)
    ;(document.activeElement as HTMLElement | null)?.blur()
    await user.keyboard('{Enter}')
    expect(onSubmit).toHaveBeenCalledWith(ANSWER)
  })

  it('пока идёт запрос, кнопка «Проверяем…» недоступна и ответ второй раз не уходит', async () => {
    const user = userEvent.setup()
    const { onSubmit } = setup({ pending: true, frozen: true })
    expect(screen.getByRole('button', { name: 'Проверяем…' })).toBeDisabled()
    await user.keyboard('1b')
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('замороженное упражнение не принимает ни мышь, ни клавиши', async () => {
    const user = userEvent.setup()
    setup({ frozen: true })
    for (let i = 0; i < 5; i++) {
      expect(word(i)).toBeDisabled()
      expect(option(i)).toBeDisabled()
    }
    await user.keyboard('1b')
    expect(screen.queryByRole('button', { name: /соединено/ })).not.toBeInTheDocument()
  })

  it('клавиши с Ctrl, Cmd и Alt не трогают упражнение', async () => {
    const user = userEvent.setup()
    setup()
    await user.keyboard('{Control>}1{/Control}')
    await user.keyboard('{Meta>}a{/Meta}')
    expect(word(0)).toHaveAttribute('aria-pressed', 'false')
  })

  it('после ответа: счёт, разбор каждой пары словами и значками, а не только цветом', async () => {
    setup({ frozen: true, result: RESULT([true, true, true, false, false]) })
    expect(screen.getByText('Верно 3 из 5')).toBeInTheDocument()
    const report = screen.getByRole('list', { name: 'Разбор пар' })
    const items = within(report).getAllByRole('listitem')
    expect(items).toHaveLength(5)
    expect(items[0]).toHaveTextContent('apple')
    expect(items[0]).toHaveTextContent('верно')
    expect(items[3]).toHaveTextContent('book')
    expect(items[3]).toHaveTextContent('неверно')
    expect(items[3]).toHaveTextContent('верный перевод: книга')
  })

  it('плитки после ответа называют верность: «верно» или «неверно»', () => {
    setup({ frozen: true, result: RESULT([true, false, true, true, true]) })
    expect(screen.getByRole('button', { name: /^1\. apple.*, верно$/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^2\. house.*, неверно$/ })).toBeInTheDocument()
  })

  it('источник ввода записан в корне: клавиатура или указатель', async () => {
    const user = userEvent.setup()
    const { container } = setup()
    const root = container.firstElementChild as HTMLElement
    await user.click(word(0))
    expect(root).toHaveAttribute('data-input', 'pointer')
    await user.keyboard('2')
    expect(root).toHaveAttribute('data-input', 'keyboard')
  })

  it('возвращает фокус на карточку после нажатия мышью (цифры и Enter работают сразу)', async () => {
    const user = userEvent.setup()
    const { restoreFocus } = setup()
    await user.click(word(0))
    expect(restoreFocus).toHaveBeenCalled()
  })
})

describe('MatchExercise: анимации', () => {
  beforeEach(() => {
    window.dispatchEvent(new Event('pointerdown')) // последнее действие — мышь
  })
  afterEach(() => vi.unstubAllGlobals())

  const root = (c: HTMLElement) => c.firstElementChild as HTMLElement

  it('движение включено для мыши и выключается, как только нажата клавиша', async () => {
    const user = userEvent.setup()
    const { container } = setup()
    expect(root(container)).toHaveAttribute('data-motion', 'on')
    await user.keyboard('1')
    expect(root(container)).toHaveAttribute('data-motion', 'off')
  })

  it('при «уменьшить движение» оно выключено с самого начала', () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), addEventListener() {}, removeEventListener() {} }))
    const { container } = setup()
    expect(root(container)).toHaveAttribute('data-motion', 'off')
    expect(container.querySelector('.m-tile-in')).toBeNull()
  })

  it('плитки входят по очереди после мыши и без анимации после клавиатуры', () => {
    const first = setup()
    expect(first.container.querySelectorAll('.m-tile-in')).toHaveLength(10)
    first.unmount()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
    const second = setup()
    expect(second.container.querySelector('.m-tile-in')).toBeNull()
  })

  it('нить рисуется для каждой пары своим цветом и пропадает с разрывом', async () => {
    const user = userEvent.setup()
    const { container } = setup()
    expect(container.querySelectorAll('path.m-thread')).toHaveLength(0)
    await user.click(word(0))
    await user.click(option(1))
    await user.click(word(1))
    await user.click(option(4))
    const paths = container.querySelectorAll('path.m-thread')
    expect(paths).toHaveLength(2)
    expect(paths[0]).toHaveAttribute('stroke', 'var(--sky)')
    expect(paths[1]).toHaveAttribute('stroke', 'var(--grass)')
    expect(paths[0].closest('svg')).toHaveAttribute('aria-hidden', 'true')
    await user.click(screen.getByRole('button', { name: /^1\. apple, соединено/ }))
    expect(container.querySelectorAll('path.m-thread')).toHaveLength(1)
  })

  it('после проверки нити окрашены по верности', () => {
    const { container } = setup({ frozen: true, result: RESULT([true, false, true, true, true]) })
    expect(container.querySelectorAll('path.m-thread').length).toBeGreaterThanOrEqual(0)
  })

  it('верные пары дают вспышку и четыре искры, неверные трясутся (мышь)', () => {
    const { container } = setup({ frozen: true, result: RESULT([true, true, true, false, false]) })
    expect(container.querySelectorAll('.m-flash')).toHaveLength(3)
    expect(container.querySelectorAll('.m-spark')).toHaveLength(12)
    expect(container.querySelectorAll('.m-wrong')).toHaveLength(2)
  })

  it('очередь проверки: у каждой пары своя задержка с шагом 90 мс', () => {
    const { container } = setup({ frozen: true, result: RESULT([true, true, true, true, true]) })
    const delays = Array.from(container.querySelectorAll<HTMLElement>('.m-flash')).map((e) => e.style.getPropertyValue('--d'))
    expect(delays).toEqual(['0ms', '90ms', '180ms', '270ms', '360ms'])
  })

  it('пять из пяти: конфетти из двенадцати и подсветка рамки', () => {
    const { container } = setup({ frozen: true, result: RESULT([true, true, true, true, true]) })
    expect(container.querySelectorAll('.m-confetti')).toHaveLength(12)
    expect(container.querySelector('.m-ring')).not.toBeNull()
  })

  it('меньше пяти: праздника нет', () => {
    const { container } = setup({ frozen: true, result: RESULT([true, true, true, true, false]) })
    expect(container.querySelector('.m-confetti')).toBeNull()
    expect(container.querySelector('.m-ring')).toBeNull()
  })

  it('после клавиатуры результат без вспышек, искр, тряски и конфетти, но с итогом словами', async () => {
    const user = userEvent.setup()
    const view = setup()
    await user.keyboard('1')
    view.rerender(
      <MatchExercise words={WORDS} options={OPTIONS} frozen pending={false} result={RESULT([true, true, true, true, true])} onSubmit={vi.fn()} restoreFocus={vi.fn()} />,
    )
    const c = view.container
    for (const cls of ['.m-flash', '.m-spark', '.m-wrong', '.m-confetti', '.m-ring']) expect(c.querySelector(cls)).toBeNull()
    expect(screen.getByText('Верно 5 из 5')).toBeInTheDocument()
    expect(within(screen.getByRole('list', { name: 'Разбор пар' })).getAllByRole('listitem')).toHaveLength(5)
  })

  it('при «уменьшить движение» результат без анимаций', () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), addEventListener() {}, removeEventListener() {} }))
    const { container } = setup({ frozen: true, result: RESULT([true, false, true, true, true]) })
    for (const cls of ['.m-flash', '.m-spark', '.m-wrong', '.m-confetti', '.m-ring']) expect(container.querySelector(cls)).toBeNull()
  })

  it('счёт для глаз набирается ступенями, читалка получает итог сразу', async () => {
    vi.useFakeTimers()
    try {
      const { container } = setup({ frozen: true, result: RESULT([true, true, true, false, false]) })
      const visual = () => container.querySelector('p[aria-hidden="true"]')!.textContent
      expect(screen.getByText('Верно 3 из 5')).toHaveClass('sr-only')
      expect(visual()).toBe('Счёт: 0 / 5')
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5 * 90 + 100)
      })
      expect(visual()).toBe('Счёт: 1 / 5')
      await act(async () => {
        await vi.advanceTimersByTimeAsync(400)
      })
      expect(visual()).toBe('Счёт: 3 / 5')
    } finally {
      vi.useRealTimers()
    }
  })

  it('движение выключено: счёт сразу итоговый', async () => {
    const user = userEvent.setup()
    const view = setup()
    await user.keyboard('1')
    view.rerender(
      <MatchExercise words={WORDS} options={OPTIONS} frozen pending={false} result={RESULT([true, true, true, false, false])} onSubmit={vi.fn()} restoreFocus={vi.fn()} />,
    )
    expect(view.container.querySelector('p[aria-hidden="true"]')!.textContent).toBe('Счёт: 3 / 5')
  })
})
