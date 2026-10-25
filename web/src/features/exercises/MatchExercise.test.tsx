import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
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
