import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { SessionPage } from './SessionPage'

afterEach(() => vi.unstubAllGlobals())

const SENTENCE = {
  en: { text: 'I like to run every morning.', author: 'CK', tatoebaId: 11 },
  ru: { text: 'Я люблю бегать по утрам.', author: 'marafon', tatoebaId: 22 },
  gap: { start: 10, end: 13 },
}
const card = (id: number) => ({
  cardId: id, lemma: 'run', pos: 'verb', level: 'B2', state: 0,
  intervals: { again: 60, hard: 360, good: 600, easy: 345_600 }, sentence: SENTENCE,
})
const exercise = (id: number) => ({
  id, kind: 'gap', prompt: 'She is _____ engineer.', translation: 'Она инженер.',
  choices: ['a', 'an', 'the', 'no article'], topic: 'grammar:articles',
})
const progress = (minutes: number, extra: Record<string, unknown> = {}) => ({
  minutes, minimumDay: 10, goalMinutes: 15, doneToday: minutes >= 10, streakDays: 3,
  today: { answers: 5, correct: 4, cardsReviewed: 2, mistakesOpened: 1, mistakesClosed: 2 }, ...extra,
})

type World = {
  due: number // сколько карточек ждёт; 0 — нет
  minutes: number
  /** на сколько минут вырастает прогресс после каждого ответа (карточки и упражнения) */
  step: number
  noExercise?: boolean
  progressStatus?: number
  extra?: Record<string, unknown>
}

function stubApi(w: World) {
  const calls = { reviews: 0, answers: 0, nexts: 0 }
  let nextId = 100
  const queue = () => jsonResponse(200, { dueCount: w.due, totalCount: w.due + 3, ...(w.due > 0 ? { card: card(1000 + calls.reviews) } : {}) })
  const fn = vi.fn(async (url: string) => {
    if (url.endsWith('/cards/queue')) return queue()
    if (url.endsWith('/cards/review')) {
      calls.reviews++
      w.due = Math.max(0, w.due - 1)
      w.minutes += w.step
      return queue()
    }
    if (url.endsWith('/progress/today')) {
      if (w.progressStatus) return errorResponse(w.progressStatus, w.progressStatus === 401 ? 'unauthorized' : 'internal', 'Внутренняя ошибка')
      return jsonResponse(200, progress(w.minutes, w.extra))
    }
    if (url.endsWith('/exercises/next')) {
      calls.nexts++
      if (w.noExercise) return errorResponse(404, 'no_exercise', 'Подходящих упражнений пока нет')
      return jsonResponse(200, exercise(nextId++))
    }
    if (url.endsWith('/exercises/answer')) {
      calls.answers++
      w.minutes += w.step
      return jsonResponse(200, { correct: true, answer: 'an', translation: 'Она инженер.', ratings: [] })
    }
    if (url.endsWith('/exercises/topics')) return jsonResponse(200, { topics: [] })
    throw new Error(`неожиданный запрос ${url}`)
  })
  vi.stubGlobal('fetch', fn)
  return calls
}

function renderSession() {
  return renderAt('/session', [
    { path: '/session', element: <SessionPage /> },
    { path: '/', element: <p>Главная</p> },
    { path: '/login', element: <p>Экран входа</p> },
  ])
}

const waitForExercise = () => screen.findByLabelText('Упражнение')

describe('SessionPage: шаги', () => {
  it('без карточек сразу упражнения, шага «Карточки» нет', async () => {
    stubApi({ due: 0, minutes: 0, step: 1 })
    renderSession()
    await waitForExercise()
    expect(screen.queryByLabelText('Карточка')).not.toBeInTheDocument()
    expect(screen.queryByText(/Карточка \d+ из/)).not.toBeInTheDocument()
  })

  it('карточки по одной со счётчиком, после последней — упражнения', async () => {
    const calls = stubApi({ due: 2, minutes: 0, step: 0.5 })
    renderSession()
    const user = userEvent.setup()
    expect(await screen.findByText('Карточка 1 из 2')).toBeInTheDocument()
    await user.keyboard(' ')
    await user.keyboard('3')
    expect(await screen.findByText('Карточка 2 из 2')).toBeInTheDocument()
    await user.keyboard(' ')
    await user.keyboard('3')
    await waitForExercise()
    expect(calls.reviews).toBe(2)
  })

  it('не больше 10 карточек за сессию, даже если в очереди 50', async () => {
    const calls = stubApi({ due: 50, minutes: 0, step: 0.1 })
    renderSession()
    const user = userEvent.setup()
    expect(await screen.findByText('Карточка 1 из 10')).toBeInTheDocument()
    for (let i = 1; i <= 10; i++) {
      await screen.findByText(`Карточка ${i} из 10`)
      await user.keyboard(' ')
      await user.keyboard('3')
    }
    await waitForExercise()
    expect(calls.reviews).toBe(10)
    expect(screen.queryByText(/Карточка \d+ из/)).not.toBeInTheDocument()
  })

  it('полоса минут обновляется после ответа', async () => {
    stubApi({ due: 0, minutes: 2, step: 3 })
    renderSession()
    const user = userEvent.setup()
    await waitForExercise()
    const bar = await screen.findByRole('progressbar', { name: 'Минимальный день' })
    expect(bar).toHaveAttribute('aria-valuenow', '2')
    await user.keyboard('2')
    await screen.findByText('Верно')
    await waitFor(() => expect(screen.getByRole('progressbar', { name: 'Минимальный день' })).toHaveAttribute('aria-valuenow', '5'))
  })
})

describe('SessionPage: минимум и итог', () => {
  it('когда минуты дошли до 10, после «Дальше» экран «Минимум на сегодня выполнен»', async () => {
    stubApi({ due: 0, minutes: 8, step: 3 })
    renderSession()
    const user = userEvent.setup()
    await waitForExercise()
    await user.keyboard('2')
    await screen.findByText('Верно') // результат остаётся на экране, пока пользователь не нажмёт «Дальше»
    expect(screen.queryByRole('heading', { name: 'Минимум на сегодня выполнен' })).not.toBeInTheDocument()
    await user.keyboard(' ')
    expect(await screen.findByRole('heading', { name: 'Минимум на сегодня выполнен' })).toBeInTheDocument()
  })

  it('«Ещё немного» возвращает к упражнениям и экран минимума больше не появляется', async () => {
    const calls = stubApi({ due: 0, minutes: 8, step: 3 })
    renderSession()
    const user = userEvent.setup()
    await waitForExercise()
    await user.keyboard('2')
    await screen.findByText('Верно')
    await user.keyboard(' ')
    await user.click(await screen.findByRole('button', { name: 'Ещё немного' }))
    await waitForExercise()
    await user.keyboard('2')
    await screen.findByText('Верно')
    await user.keyboard(' ')
    await waitForExercise() // снова упражнение, а не экран минимума
    expect(screen.queryByRole('heading', { name: 'Минимум на сегодня выполнен' })).not.toBeInTheDocument()
    expect(calls.answers).toBe(2)
  })

  it('«Закончить» с экрана минимума ведёт к итогу дня', async () => {
    stubApi({ due: 0, minutes: 8, step: 3 })
    renderSession()
    const user = userEvent.setup()
    await waitForExercise()
    await user.keyboard('2')
    await screen.findByText('Верно')
    await user.keyboard(' ')
    await user.click(await screen.findByRole('button', { name: 'Закончить' }))
    expect(await screen.findByRole('heading', { name: 'Итог дня' })).toBeInTheDocument()
  })

  it('«Закончить» посреди карточки не теряет оценки и показывает итог с числами', async () => {
    const calls = stubApi({ due: 5, minutes: 0, step: 1 })
    renderSession()
    const user = userEvent.setup()
    await screen.findByText('Карточка 1 из 5')
    await user.keyboard(' ')
    await user.keyboard('3')
    await screen.findByText('Карточка 2 из 5')
    await user.click(screen.getByRole('button', { name: 'Закончить' }))
    expect(await screen.findByRole('heading', { name: 'Итог дня' })).toBeInTheDocument()
    expect(calls.reviews).toBe(1)
    // числа из ответа сервера: 5 ответов, 4 верных, ошибок открыто 1 и закрыто 2, серия 3 дня
    expect(screen.getByText(/5 ответов/)).toBeInTheDocument()
    expect(screen.getByText(/4 верных/)).toBeInTheDocument()
    expect(screen.getByText(/открыто 1/i)).toBeInTheDocument()
    expect(screen.getByText(/закрыто 2/i)).toBeInTheDocument()
    expect(screen.getByText(/3 дня/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/')
  })

  it('итог называет день, который спасла заморозка', async () => {
    stubApi({ due: 0, minutes: 3, step: 1, extra: { freezeUsedOn: '2026-10-24' } })
    renderSession()
    const user = userEvent.setup()
    await waitForExercise()
    await user.click(screen.getByRole('button', { name: 'Закончить' }))
    expect(await screen.findByText(/заморозка спасла 24 октября/i)).toBeInTheDocument()
  })

  it('на итоге «Ещё немного» возвращает к упражнениям', async () => {
    stubApi({ due: 0, minutes: 3, step: 1 })
    renderSession()
    const user = userEvent.setup()
    await waitForExercise()
    await user.click(screen.getByRole('button', { name: 'Закончить' }))
    await user.click(await screen.findByRole('button', { name: 'Ещё немного' }))
    await waitForExercise()
  })
})

describe('SessionPage: состояния', () => {
  it('упражнений нет вообще: сообщение, «На главную» и «Закончить», без зацикливания', async () => {
    const calls = stubApi({ due: 0, minutes: 0, step: 1, noExercise: true })
    renderSession()
    expect(await screen.findByRole('heading', { name: 'Подходящих упражнений пока нет' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/')
    await new Promise((r) => setTimeout(r, 300))
    expect(calls.nexts).toBe(1) // запрос один, а не цикл
  })

  it('401 ведёт на вход', async () => {
    stubApi({ due: 0, minutes: 0, step: 1, progressStatus: 401 })
    renderSession()
    expect(await screen.findByText('Экран входа')).toBeInTheDocument()
  })

  it('ошибка загрузки прогресса: сообщение и «Повторить», страница работает', async () => {
    stubApi({ due: 0, minutes: 0, step: 1, progressStatus: 500 })
    renderSession()
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
    await waitForExercise()
  })
})
