import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { ExerciseCard } from './ExerciseCard'
import type { Exercise } from './exercises'

afterEach(() => vi.unstubAllGlobals())

const exercise: Exercise = {
  id: 5,
  kind: 'gap',
  prompt: 'She is _____ engineer.',
  translation: 'Она инженер.',
  choices: ['a', 'an', 'the', 'no article'],
  topic: 'grammar:articles',
}

function setup(onAnswered = vi.fn()) {
  const answers: { exerciseId: number; given: string; key: string }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/exercises/answer')) {
        answers.push(JSON.parse(init?.body as string))
        return jsonResponse(200, { correct: true, answer: 'an', translation: 'Она инженер.', ratings: [] })
      }
      if (url.endsWith('/exercises/topics')) return jsonResponse(200, { topics: [] })
      throw new Error(`неожиданный запрос ${url}`)
    }),
  )
  const onNext = vi.fn()
  renderAt('/x', [{ path: '/x', element: <ExerciseCard exercise={exercise} onNext={onNext} loadingNext={false} onAnswered={onAnswered} /> }])
  return { answers, onAnswered, onNext }
}

describe('ExerciseCard', () => {
  it('ответ клавишей 2 вызывает onAnswered один раз с результатом', async () => {
    const { answers, onAnswered } = setup()
    const user = userEvent.setup()
    await screen.findByLabelText('Упражнение')
    await user.keyboard('2')
    await screen.findByText('Верно')
    await waitFor(() => expect(onAnswered).toHaveBeenCalledTimes(1))
    expect(onAnswered).toHaveBeenCalledWith(expect.objectContaining({ correct: true }))
    expect(answers).toHaveLength(1)
    expect(answers[0]).toMatchObject({ exerciseId: 5, given: 'an' })
  })

  it('Пробел после ответа вызывает onNext, а до ответа onAnswered не вызывается', async () => {
    const { onAnswered, onNext } = setup()
    const user = userEvent.setup()
    await screen.findByLabelText('Упражнение')
    expect(onAnswered).not.toHaveBeenCalled()
    await user.keyboard('2')
    await screen.findByText('Верно')
    await user.keyboard(' ')
    expect(onNext).toHaveBeenCalledTimes(1)
  })
})

describe('ExerciseCard: сопоставление', () => {
  const match: Exercise = {
    id: 9,
    kind: 'match',
    prompt: 'Соедини слова с переводами',
    translation: '',
    choices: [],
    words: ['apple', 'house', 'water', 'book', 'dog'],
    options: ['книга', 'яблоко', 'собака', 'вода', 'дом'],
    topic: 'skill:match',
  }
  const pairs = [
    { word: 'apple', given: 'яблоко', answer: 'яблоко', correct: true },
    { word: 'house', given: 'дом', answer: 'дом', correct: true },
    { word: 'water', given: 'вода', answer: 'вода', correct: true },
    { word: 'book', given: 'собака', answer: 'книга', correct: false },
    { word: 'dog', given: 'книга', answer: 'собака', correct: false },
  ]

  function setupMatch() {
    const answers: { exerciseId: number; given: string; key: string }[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string, init?: RequestInit) => {
        if (url.endsWith('/exercises/answer')) {
          answers.push(JSON.parse(init?.body as string))
          return jsonResponse(200, { correct: false, answer: 'x', translation: '', ratings: [], pairs })
        }
        if (url.endsWith('/exercises/topics')) return jsonResponse(200, { topics: [] })
        throw new Error(`неожиданный запрос ${url}`)
      }),
    )
    const onNext = vi.fn()
    renderAt('/x', [{ path: '/x', element: <ExerciseCard exercise={match} onNext={onNext} loadingNext={false} /> }])
    return { answers, onNext }
  }

  it('цифры 1–4 не отправляют ответ как у вариантов; пары собираются, ответ уходит строкой', async () => {
    const { answers } = setupMatch()
    const user = userEvent.setup()
    await screen.findByLabelText('Упражнение')
    await user.keyboard('1')
    expect(answers).toHaveLength(0)
    await user.keyboard('b')
    await user.keyboard('2e3d4c5a')
    await user.keyboard('{Enter}')
    await screen.findByText('Верно 3 из 5')
    expect(answers).toHaveLength(1)
    expect(answers[0]).toMatchObject({ exerciseId: 9, given: 'apple=яблоко;house=дом;water=вода;book=собака;dog=книга' })
    expect(screen.getByText('Есть ошибки в парах')).toBeInTheDocument()
    expect(screen.queryByText(/Верный ответ:/)).not.toBeInTheDocument()
  })

  it('после ответа Пробел идёт дальше', async () => {
    const { onNext } = setupMatch()
    const user = userEvent.setup()
    await screen.findByLabelText('Упражнение')
    await user.keyboard('1b2e3d4c5a{Enter}')
    await screen.findByText('Верно 3 из 5')
    await user.keyboard(' ')
    expect(onNext).toHaveBeenCalledTimes(1)
  })
})
