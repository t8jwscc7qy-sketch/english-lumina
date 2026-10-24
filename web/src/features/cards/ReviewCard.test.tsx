import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { QueueCard } from '@/api/client'
import { jsonResponse, renderAt } from '@/test/render'
import { ReviewCard } from './ReviewCard'

afterEach(() => vi.unstubAllGlobals())

const card: QueueCard = {
  cardId: 7,
  lemma: 'run',
  pos: 'verb',
  level: 'B2',
  state: 0,
  intervals: { again: 60, hard: 360, good: 600, easy: 345_600 },
  sentence: {
    en: { text: 'I like to run every morning.', author: 'CK', tatoebaId: 11 },
    ru: { text: 'Я люблю бегать по утрам.', author: 'marafon', tatoebaId: 22 },
    gap: { start: 10, end: 13 },
  },
}

function setup(onRated = vi.fn()) {
  const reviews: { cardId: number; rating: number; key: string }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/cards/review')) {
        reviews.push(JSON.parse(init?.body as string))
        return jsonResponse(200, { dueCount: 0, totalCount: 3 })
      }
      throw new Error(`неожиданный запрос ${url}`)
    }),
  )
  renderAt('/x', [{ path: '/x', element: <ReviewCard card={card} onRated={onRated} /> }])
  return { reviews, onRated }
}

describe('ReviewCard', () => {
  it('Пробел открывает ответ, клавиша 3 оценивает и вызывает onRated ровно один раз', async () => {
    const { reviews, onRated } = setup()
    const user = userEvent.setup()
    await screen.findByLabelText('Карточка')
    await user.keyboard(' ')
    await screen.findByText('run')
    await user.keyboard('3')
    await waitFor(() => expect(onRated).toHaveBeenCalledTimes(1))
    expect(reviews).toHaveLength(1)
    expect(reviews[0]).toMatchObject({ cardId: 7, rating: 3 })
    await user.keyboard('3') // карточка уже оценена: повторных запросов нет
    expect(reviews).toHaveLength(1)
  })

  it('до открытия ответа цифры не оценивают и onRated не вызывается', async () => {
    const { reviews, onRated } = setup()
    const user = userEvent.setup()
    await screen.findByLabelText('Карточка')
    await user.keyboard('3')
    expect(reviews).toHaveLength(0)
    expect(onRated).not.toHaveBeenCalled()
  })
})
