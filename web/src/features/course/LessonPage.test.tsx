import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { lessonFixture } from './fixtures'
import { LessonPage } from './LessonPage'

afterEach(() => vi.unstubAllGlobals())

function stub() {
  const completed: Record<string, unknown>[] = []
  const lessonCalls: number[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/course/complete')) {
        completed.push(JSON.parse(String(init?.body)))
        return jsonResponse(200, { lessonId: 'u1-l2', newWords: 2, cardsAdded: 2, seconds: 60, repeated: false, next: { id: 'u1-l3', title: 'Кто это?', number: 3, unit: 1, state: 'current', newWords: 5 } })
      }
      if (url.endsWith('/course/lesson')) {
        lessonCalls.push(1)
        return jsonResponse(200, lessonFixture)
      }
      return jsonResponse(200, {})
    }),
  )
  return Object.assign(completed, { lessonCalls })
}

const render = () =>
  renderAt('/lesson', [
    { path: '/lesson', element: <LessonPage /> },
    { path: '/path', element: <p>путь</p> },
  ])

describe('LessonPage', () => {
  it('проходит урок целиком: слова, текст, вопрос, задания с ошибкой и повтором, итог', async () => {
    const completed = stub()
    const user = userEvent.setup()
    render()

    // Слова
    expect(await screen.findByRole('heading', { name: 'how' })).toBeInTheDocument()
    expect(screen.getByText('Слово 1 из 2')).toBeInTheDocument()
    expect(screen.getByText('How', { selector: 'mark' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(screen.getByRole('heading', { name: 'fine' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Дальше' }))

    // Текст: нажатие на слово показывает перевод, имя — «имя»
    expect(screen.getByRole('heading', { name: 'Как дела?' })).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Hello' })[0])
    expect(screen.getByText('— привет')).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Anna' })[0])
    expect(screen.getByText('— имя')).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Перевод' })[0])
    expect(screen.getByText('Привет, Анна! Как дела?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Дальше' }))

    // Вопрос: «Проверить» недоступна до выбора
    expect(screen.getByRole('heading', { name: 'Как дела у Анны?' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Проверить' })).toBeDisabled()
    await user.keyboard('1')
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Верно!')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))

    // Пропуск: неверный ответ — показ верного и повтор в конце
    expect(screen.getByRole('img', { name: 'пропуск' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Fine/ }))
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Неверно')
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Правильно: How')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))

    // Порядок слов
    expect(screen.getByText('Собери предложение')).toBeInTheDocument()
    for (const w of ['I', 'am', 'fine']) await user.click(screen.getByRole('button', { name: w }))
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Верно!')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))

    // Пары: «Дальше» недоступна, пока не собраны все
    expect(screen.getByRole('heading', { name: 'Собери пары' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Дальше' })).toBeDisabled()
    for (const [en, ru] of [['how', 'как'], ['fine', 'хорошо'], ['hello', 'привет']]) {
      await user.click(screen.getByRole('button', { name: en }))
      await user.click(screen.getByRole('button', { name: ru }))
    }
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Все пары собраны')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))

    // Повтор задания с ошибкой
    expect(screen.getByText('Ещё раз')).toBeInTheDocument()
    await user.keyboard('1')
    await user.click(screen.getByRole('button', { name: /How/ }))
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    await user.click(screen.getByRole('button', { name: 'Завершить' }))

    // Итог: отправлен один раз, с числом ошибок
    expect(await screen.findByRole('heading', { name: 'Урок пройден' })).toBeInTheDocument()
    await waitFor(() => expect(completed).toHaveLength(1))
    expect(completed[0]).toMatchObject({ lessonId: 'u1-l2', mistakes: 1 })
    expect(await screen.findByRole('button', { name: 'Следующий урок: Кто это?' })).toBeEnabled()
    expect(screen.getByText('How are you?', { selector: 'h3' })).toBeInTheDocument()
    // Сохранение урока не перечитывает открытый урок (иначе итог подменился бы следующим уроком).
    expect(completed.lessonCalls).toHaveLength(1)
  })
})

describe('LessonPage: клавиатура и повтор', () => {
  it('Enter после выбора мышью нажимает «Проверить», а не вариант; на повторе нет обещания вернуть задание', async () => {
    stub()
    const user = userEvent.setup()
    render()
    await screen.findByRole('heading', { name: 'how' })
    for (let i = 0; i < 3; i++) await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.click(screen.getByRole('button', { name: /Хорошо/ }))
    await user.keyboard('{Enter}')
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Верно!')
    await user.keyboard('{Enter}')
    // Пропуск: ошибка мышью и Enter
    await user.click(screen.getByRole('button', { name: /Fine/ }))
    await user.keyboard('{Enter}')
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('вернётся в конце')
    await user.keyboard('{Enter}')
    for (const w of ['I', 'am', 'fine']) await user.click(screen.getByRole('button', { name: w }))
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    for (const [en, ru] of [['how', 'как'], ['fine', 'хорошо'], ['hello', 'привет']]) {
      await user.click(screen.getByRole('button', { name: en }))
      await user.click(screen.getByRole('button', { name: ru }))
    }
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(screen.getByText('Ещё раз')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Fine/ }))
    await user.click(screen.getByRole('button', { name: 'Проверить' }))
    expect(screen.getByRole('status', { name: 'Результат' })).toHaveTextContent('Правильно: How')
    expect(screen.getByRole('status', { name: 'Результат' })).not.toHaveTextContent('вернётся')
  })
})
