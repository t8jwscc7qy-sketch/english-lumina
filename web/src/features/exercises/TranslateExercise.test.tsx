import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { ExercisesPage } from './ExercisesPage'

afterEach(() => vi.unstubAllGlobals())

const translate = (id = 1, repeat = false) => ({
  id,
  kind: 'translate',
  prompt: 'Мы едим красные яблоки каждое утро.',
  translation: '',
  choices: [],
  topic: 'skill:translate',
  ...(repeat ? { repeat: true } : {}),
})
const rating = (topic: string, r: number) => ({ topic, rating: r, deviation: 120, attempts: 4 })
const MAIN = 'We eat red apples every morning.'
const result = (extra: Record<string, unknown> = {}) => ({
  correct: true,
  answer: MAIN,
  translation: '',
  ratings: [rating('skill:translate', 1512)],
  accepted: [],
  ...extra,
})

type Body = { exerciseId: number; given: string; key: string }

function stubApi(answer: () => Response = () => jsonResponse(200, result()), repeat = false) {
  const answers: Body[] = []
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/exercises/next')) return jsonResponse(200, translate(1, repeat))
    if (url.endsWith('/exercises/topics'))
      return jsonResponse(200, { topics: [rating('skill:translate', 1500)] })
    if (url.endsWith('/exercises/answer')) {
      answers.push(JSON.parse(init?.body as string) as Body)
      return answer()
    }
    throw new Error(`неожиданный запрос ${url}`)
  })
  vi.stubGlobal('fetch', fn)
  return { answers }
}

async function renderCard() {
  renderAt('/exercises', [
    { path: '/exercises', element: <ExercisesPage /> },
    { path: '/login', element: <p>Экран входа</p> },
  ])
  await screen.findByRole('article', { name: 'Упражнение' })
}

const field = () => screen.getByRole('textbox', { name: 'Твой перевод' })
const status = () => screen.getByRole('status')

describe('TranslateExercise', () => {
  it('показывает русскую фразу, подпись и поле без подсказок клавиатуры', async () => {
    stubApi()
    await renderCard()
    expect(screen.getByText('Переведи на английский')).toBeInTheDocument()
    const phrase = screen.getByText('Мы едим красные яблоки каждое утро.')
    expect(phrase).toHaveAttribute('lang', 'ru')
    const input = field()
    expect(input).toHaveAttribute('lang', 'en')
    expect(input).toHaveAttribute('spellcheck', 'false')
    expect(input).toHaveAttribute('autocapitalize', 'off')
    expect(input).toHaveAttribute('autocomplete', 'off')
    expect(input).toHaveAttribute('autocorrect', 'off')
    expect(input).toHaveFocus()
  })

  it('«Проверить» недоступна при пустом и пробельном поле', async () => {
    stubApi()
    const user = userEvent.setup()
    await renderCard()
    const check = screen.getByRole('button', { name: 'Проверить' })
    expect(check).toBeDisabled()
    await user.type(field(), '   ')
    expect(check).toBeDisabled()
    await user.type(field(), 'we')
    expect(check).toBeEnabled()
  })

  it('Enter отправляет обрезанный текст, Shift+Enter переносит строку', async () => {
    const { answers } = stubApi()
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), '  We eat red apples{Shift>}{Enter}{/Shift}every morning  ')
    expect(answers).toHaveLength(0)
    expect((field() as HTMLTextAreaElement).value).toContain('\n')
    await user.keyboard('{Enter}')
    await waitFor(() => expect(answers).toHaveLength(1))
    expect(answers[0].given).toBe('We eat red apples\nevery morning')
    expect(answers[0].exerciseId).toBe(1)
  })

  it('Esc очищает поле', async () => {
    stubApi()
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), 'we eat')
    await user.keyboard('{Escape}')
    expect((field() as HTMLTextAreaElement).value).toBe('')
  })

  it('верный ответ: «Верно», другие переводы, поле только для чтения, «Дальше»', async () => {
    stubApi(() =>
      jsonResponse(
        200,
        result({
          accepted: ['Every morning we eat red apples.', 'We have red apples every morning.'],
        }),
      ),
    )
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), 'we eat red apples every morning{Enter}')
    await within(status()).findByText('Верно')
    expect(within(status()).getByText(/Так тоже можно/)).toBeInTheDocument()
    expect(within(status()).getByText('Every morning we eat red apples.')).toHaveAttribute(
      'lang',
      'en',
    )
    expect(within(status()).queryByText(/Верный ответ/)).not.toBeInTheDocument()
    expect(field()).toHaveAttribute('readonly')
    expect(screen.getByRole('button', { name: 'Дальше' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Проверить' })).not.toBeInTheDocument()
  })

  it('опечатка: «Почти верно», подсвеченная пропущенная буква и «Правильно»', async () => {
    stubApi(() => jsonResponse(200, result({ nearMiss: MAIN })))
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), 'We eat red aples every morning.{Enter}')
    await within(status()).findByText(/Почти верно/)
    const mark = status().querySelector('mark')
    expect(mark).not.toBeNull()
    expect(mark?.textContent).toBe('p')
    expect(within(status()).getByText(/Правильно/).textContent).toContain(MAIN)
    expect(within(status()).queryByText('Неверно')).not.toBeInTheDocument()
  })

  it('неверный ответ: «Неверно» и «Верный ответ»', async () => {
    stubApi(() =>
      jsonResponse(
        200,
        result({
          correct: false,
          mistake: { status: 'open', streak: 0, nextDue: '2026-10-27T10:00:00Z' },
        }),
      ),
    )
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), 'they drink tea{Enter}')
    await within(status()).findByText('Неверно')
    expect(within(status()).getByText(/Верный ответ/).textContent).toContain(MAIN)
    expect(within(status()).getByText('Вернётся завтра')).toBeInTheDocument()
  })

  it('повтор ошибки помечен', async () => {
    stubApi(undefined, true)
    await renderCard()
    expect(screen.getByText('Повтор ошибки')).toBeInTheDocument()
  })

  it('движение: с клавиатуры выключено, мышью включено', async () => {
    stubApi()
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), 'we eat red apples every morning{Enter}')
    await within(status()).findByText('Верно')
    expect(document.querySelector('[data-motion]')).toHaveAttribute('data-motion', 'off')
  })

  it('движение мышью: включено', async () => {
    stubApi()
    const user = userEvent.setup()
    await renderCard()
    await user.type(field(), 'we eat red apples every morning')
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    await within(status()).findByText('Верно')
    expect(document.querySelector('[data-motion]')).toHaveAttribute('data-motion', 'on')
  })
})
