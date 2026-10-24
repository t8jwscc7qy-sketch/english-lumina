import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { ExercisesPage } from './ExercisesPage'

afterEach(() => vi.unstubAllGlobals())

const order = (id: number, tiles = ['went', 'home', 'I', 'yesterday']) => ({
  id,
  kind: 'order',
  prompt: 'Я вчера пошёл домой.',
  translation: 'Я вчера пошёл домой.',
  choices: [],
  tiles,
  topic: 'skill:order',
})
const rating = (topic: string, r: number) => ({ topic, rating: r, deviation: 120, attempts: 4 })
const result = (correct = true) => ({
  correct,
  answer: 'I went home yesterday.',
  translation: 'Я вчера пошёл домой.',
  ratings: [rating('skill:order', 1512)],
})

type Body = { exerciseId: number; given: string; key: string }

function stubApi(opts: {
  next: () => Response
  answer?: (b: Body) => Response | Promise<Response>
}) {
  const answers: Body[] = []
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/exercises/next')) return opts.next()
    if (url.endsWith('/exercises/topics'))
      return jsonResponse(200, { topics: [rating('skill:order', 1500)] })
    if (url.endsWith('/exercises/answer')) {
      const body = JSON.parse(init?.body as string) as Body
      answers.push(body)
      return opts.answer ? opts.answer(body) : jsonResponse(200, result())
    }
    throw new Error(`неожиданный запрос ${url}`)
  })
  vi.stubGlobal('fetch', fn)
  return { fn, answers }
}

function renderPage() {
  return renderAt('/exercises', [
    { path: '/exercises', element: <ExercisesPage /> },
    { path: '/login', element: <p>Экран входа</p> },
  ])
}

const bank = () => screen.getByRole('group', { name: 'Слова' })
const row = () => screen.getByRole('list', { name: 'Твой ответ' })
const rowWords = () =>
  within(row())
    .queryAllByRole('button')
    .map((b) => b.textContent)

async function open() {
  const user = userEvent.setup()
  renderPage()
  await screen.findByText('Я вчера пошёл домой.')
  await waitFor(() => expect(screen.getByRole('article', { name: 'Упражнение' })).toHaveFocus())
  return user
}

describe('OrderExercise', () => {
  it('показывает русский перевод один раз и плитки в порядке сервера', async () => {
    stubApi({ next: () => jsonResponse(200, order(1)) })
    await open()
    expect(screen.getAllByText('Я вчера пошёл домой.')).toHaveLength(1)
    expect(
      within(bank())
        .getAllByRole('button')
        .map((b) => b.querySelector('[lang=en]')?.textContent),
    ).toEqual(['went', 'home', 'I', 'yesterday'])
    expect(screen.getByText('went')).toHaveAttribute('lang', 'en')
    expect(within(screen.getByRole('article', { name: 'Упражнение' })).getByText('Порядок слов')).toBeInTheDocument()
  })

  it('клик по плитке переносит её в строку ответа, клик в строке возвращает', async () => {
    stubApi({ next: () => jsonResponse(200, order(1)) })
    const user = await open()
    await user.click(within(bank()).getByRole('button', { name: 'I' }))
    await user.click(within(bank()).getByRole('button', { name: 'went' }))
    expect(rowWords()).toEqual(['I', 'went'])
    expect(
      within(bank())
        .getAllByRole('button', { hidden: false })
        .filter((b) => !b.hasAttribute('disabled')),
    ).toHaveLength(2)
    await user.click(within(row()).getByRole('button', { name: /went/ }))
    expect(rowWords()).toEqual(['I'])
  })

  it('цифры переносят плитки по номеру в банке, 0 — десятую, Backspace и Esc возвращают', async () => {
    const tiles = ['a1', 'a2', 'a3', 'a4', 'a5', 'a6', 'a7', 'a8', 'a9', 'a10']
    stubApi({ next: () => jsonResponse(200, order(1, tiles)) })
    const user = await open()
    await user.keyboard('3')
    await user.keyboard('0')
    expect(rowWords()).toEqual(['a3', 'a10'])
    await user.keyboard('{Backspace}')
    expect(rowWords()).toEqual(['a3'])
    await user.keyboard('1{Escape}')
    expect(rowWords()).toEqual([])
    expect(within(bank()).getByRole('button', { name: 'a3' })).toBeEnabled()
  })

  it('Enter не отправляет, пока не использованы все плитки; потом отправляет слова через пробел один раз', async () => {
    let release: (r: Response) => void = () => {}
    const { answers } = stubApi({ next: () => jsonResponse(200, order(5)), answer: () => new Promise<Response>((r) => (release = r)) })
    const user = await open()
    await user.keyboard('3')
    await user.keyboard('{Enter}')
    expect(answers).toHaveLength(0)
    await user.keyboard('124')
    expect(rowWords()).toEqual(['I', 'went', 'home', 'yesterday'])
    await user.keyboard('{Enter}')
    await user.keyboard('{Enter}')
    release(jsonResponse(200, result()))
    await screen.findByText('Верно')
    expect(answers).toHaveLength(1)
    expect(answers[0]).toMatchObject({ exerciseId: 5, given: 'I went home yesterday' })
  })

  it('кнопка «Проверить» доступна только когда использованы все плитки', async () => {
    const { answers } = stubApi({ next: () => jsonResponse(200, order(5, ['a', 'b'])) })
    const user = await open()
    const check = screen.getByRole('button', { name: 'Проверить' })
    expect(check).toBeDisabled()
    await user.keyboard('21')
    expect(check).toBeEnabled()
    await user.click(check)
    await screen.findByText('Верно')
    expect(answers[0].given).toBe('b a')
  })

  it('после неверного ответа показан правильный порядок, перевод и «Дальше»; плитки неактивны', async () => {
    stubApi({
      next: () => jsonResponse(200, order(1)),
      answer: () => jsonResponse(200, result(false)),
    })
    const user = await open()
    await user.keyboard('1234')
    await user.keyboard('{Enter}')
    await screen.findByText('Неверно')
    expect(screen.getByText(/Верный ответ:/)).toHaveTextContent('I went home yesterday.')
    expect(screen.getByRole('button', { name: /Дальше/ })).toBeInTheDocument()
    expect(
      within(row())
        .getAllByRole('button')
        .every((b) => b.hasAttribute('disabled')),
    ).toBe(true)
    await user.keyboard('{Backspace}')
    expect(rowWords()).toHaveLength(4)
  })

  it('после ответа Пробел открывает следующее упражнение', async () => {
    let n = 0
    stubApi({
      next: () =>
        jsonResponse(
          200,
          n++ === 0
            ? order(1)
            : { ...order(2, ['x', 'y']), prompt: 'Дальше.', translation: 'Дальше.' },
        ),
    })
    const user = await open()
    await user.keyboard('1234{Enter}')
    await screen.findByText('Верно')
    await user.keyboard(' ')
    await screen.findByText('Дальше.')
    expect(rowWords()).toEqual([])
  })

  it('одинаковые плитки выбираются независимо', async () => {
    const { answers } = stubApi({
      next: () => jsonResponse(200, order(1, ['the', 'cat', 'the', 'dog'])),
    })
    const user = await open()
    await user.click(within(bank()).getAllByRole('button', { name: 'the' })[1])
    expect(
      within(bank())
        .getAllByRole('button', { name: 'the' })
        .filter((b) => !b.hasAttribute('disabled')),
    ).toHaveLength(1)
    await user.keyboard('1')
    expect(rowWords()).toEqual(['the', 'the'])
    await user.keyboard('24{Enter}')
    await screen.findByText('Верно')
    expect(answers[0].given).toBe('the the cat dog')
  })

  it('у плиток есть клавиша в aria-keyshortcuts, а в строке ответа — понятное имя', async () => {
    stubApi({ next: () => jsonResponse(200, order(1)) })
    const user = await open()
    expect(screen.getByRole('button', { name: 'went' })).toHaveAttribute('aria-keyshortcuts', '1')
    await user.keyboard('1')
    expect(within(row()).getByRole('button', { name: 'убрать went' })).toBeInTheDocument()
  })

  it('результат объявляется живым регионом', async () => {
    stubApi({ next: () => jsonResponse(200, order(1)) })
    const user = await open()
    const region = screen.getByRole('status')
    expect(region).toBeEmptyDOMElement()
    await user.keyboard('1234{Enter}')
    await within(region).findByText('Верно')
  })

  it('409 already_answered: сообщение и «Дальше»', async () => {
    stubApi({
      next: () => jsonResponse(200, order(1)),
      answer: () => errorResponse(409, 'already_answered', 'На это упражнение уже отвечали'),
    })
    const user = await open()
    await user.keyboard('1234{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent('уже отвечали')
    expect(screen.getByRole('button', { name: 'Дальше' })).toBeInTheDocument()
  })

  it('5xx: порядок замораживается, «Повторить» шлёт тот же ключ, «Пропустить» есть', async () => {
    let calls = 0
    const { answers } = stubApi({
      next: () => jsonResponse(200, order(1)),
      answer: () =>
        calls++ === 0
          ? errorResponse(500, 'internal', 'Сбой сервера')
          : jsonResponse(200, result()),
    })
    const user = await open()
    await user.keyboard('1234{Enter}')
    await screen.findByRole('alert')
    await user.keyboard('{Backspace}')
    expect(rowWords()).toHaveLength(4)
    expect(screen.getByRole('button', { name: 'Пропустить' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Повторить' }))
    await screen.findByText('Верно')
    expect(answers).toHaveLength(2)
    expect(answers[1].key).toBe(answers[0].key)
    expect(answers[1].given).toBe(answers[0].given)
  })

  it('401 ведёт на вход', async () => {
    stubApi({
      next: () => jsonResponse(200, order(1)),
      answer: () => errorResponse(401, 'unauthorized', 'Нужен вход'),
    })
    const user = await open()
    await user.keyboard('1234{Enter}')
    await screen.findByText('Экран входа')
  })

  it('с клавиатуры (Tab, Enter): после выбора плитки фокус на следующей свободной, а не на карточке', async () => {
    stubApi({ next: () => jsonResponse(200, order(1, ['a', 'b', 'c', 'd'])) })
    const user = await open()
    const bankBtn = (name: string) => within(bank()).getByRole('button', { name })
    bankBtn('b').focus()
    await user.keyboard('{Enter}')
    expect(bankBtn('c')).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(bankBtn('d')).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(bankBtn('a')).toHaveFocus() // за последней — первая свободная
    await user.keyboard('{Enter}')
    expect(screen.getByRole('button', { name: 'Проверить' })).toHaveFocus() // все использованы
    expect(rowWords()).toEqual(['b', 'c', 'd', 'a'])
  })

  it('с клавиатуры: возврат плитки из строки ставит фокус на соседнюю плитку строки, а при пустой строке на плитку в банке', async () => {
    stubApi({ next: () => jsonResponse(200, order(1, ['a', 'b', 'c', 'd'])) })
    const user = await open()
    await user.keyboard('123')
    within(row()).getByRole('button', { name: 'убрать b' }).focus()
    await user.keyboard('{Enter}')
    expect(within(row()).getByRole('button', { name: 'убрать c' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(within(row()).getByRole('button', { name: 'убрать a' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(within(bank()).getByRole('button', { name: 'a' })).toHaveFocus()
  })

  it('клик мышью возвращает фокус на карточку, как раньше', async () => {
    stubApi({ next: () => jsonResponse(200, order(1)) })
    const user = await open()
    await user.click(within(bank()).getByRole('button', { name: 'I' }))
    expect(screen.getByRole('article', { name: 'Упражнение' })).toHaveFocus()
  })
})
