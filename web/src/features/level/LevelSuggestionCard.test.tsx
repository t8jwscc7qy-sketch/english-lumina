import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { LevelSuggestionCard } from './LevelSuggestionCard'

afterEach(() => vi.unstubAllGlobals())

const SETTINGS = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 30, timezone: 'UTC' }
const UP = { suggestion: { direction: 'up', from: 'B1', to: 'B2', correct: 28, total: 30 } }
const DOWN = { suggestion: { direction: 'down', from: 'B1', to: 'A2', correct: 10, total: 30 } }

interface Calls {
  put: unknown[]
  dismissed: number
}

function stub(suggestion: () => Response, opts: { put?: () => Response; dismiss?: () => Response; settings?: object } = {}) {
  const calls: Calls = { put: [], dismissed: 0 }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/level/suggestion/dismiss')) {
        calls.dismissed++
        return opts.dismiss ? opts.dismiss() : new Response(null, { status: 204 })
      }
      if (url.endsWith('/level/suggestion')) return suggestion()
      if (url.endsWith('/settings') && init?.method === 'PUT') {
        calls.put.push(JSON.parse(String(init.body)))
        return opts.put ? opts.put() : jsonResponse(200, JSON.parse(String(init.body)))
      }
      if (url.endsWith('/settings')) return jsonResponse(200, opts.settings ?? SETTINGS)
      return jsonResponse(200, {})
    }),
  )
  return calls
}

const renderCard = () => renderAt('/', [{ path: '/', element: <LevelSuggestionCard /> }])

describe('LevelSuggestionCard', () => {
  it('повышение: цифры, процент и две кнопки; ничего не меняется без нажатия', async () => {
    const calls = stub(() => jsonResponse(200, UP))
    renderCard()
    expect(await screen.findByRole('heading', { name: 'Пора на уровень выше?' })).toBeInTheDocument()
    expect(screen.getByText(/последние 30 ответов на B1 ты ответил верно 28 \(93%\)/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Перейти на B2' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Пока нет' })).toBeInTheDocument()
    expect(screen.getByText(/Решать тебе/)).toBeInTheDocument()
    expect(calls.put).toHaveLength(0)
    expect(calls.dismissed).toBe(0)
  })

  it('понижение: мягкая формулировка и кнопка «Оставить B1»', async () => {
    stub(() => jsonResponse(200, DOWN))
    renderCard()
    expect(await screen.findByRole('heading', { name: 'Может, закрепить A2?' })).toBeInTheDocument()
    expect(screen.getByText(/верно 10 \(33%\)/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Перейти на A2' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Оставить B1' })).toBeInTheDocument()
  })

  it('предложения нет — карточки нет', async () => {
    stub(() => jsonResponse(200, {}))
    const { container } = renderCard()
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(container).toBeEmptyDOMElement()
  })

  it('предложение не загрузилось — карточки и ошибки нет, страница не страдает', async () => {
    stub(() => errorResponse(500, 'internal', 'Внутренняя ошибка'))
    const { container } = renderCard()
    await new Promise((r) => setTimeout(r, 30))
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('«Перейти на B2» сохраняет новый уровень и убирает карточку', async () => {
    const calls = stub(() => jsonResponse(200, UP))
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByRole('button', { name: 'Перейти на B2' }))
    await waitFor(() => expect(calls.put).toEqual([{ ...SETTINGS, currentLevel: 'B2', targetLevel: 'B2' }]))
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Пора на уровень выше?' })).not.toBeInTheDocument())
  })

  it('повышение выше цели честно говорит, что цель поднимется, и поднимает её', async () => {
    const calls = stub(() => jsonResponse(200, { suggestion: { direction: 'up', from: 'B1', to: 'B2', correct: 29, total: 30 } }), {
      settings: { ...SETTINGS, targetLevel: 'B1' },
    })
    const user = userEvent.setup()
    renderCard()
    expect(await screen.findByText(/Цель сейчас B1: она тоже станет B2/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Перейти на B2' }))
    await waitFor(() => expect(calls.put[0]).toMatchObject({ currentLevel: 'B2', targetLevel: 'B2' }))
  })

  it('понижение не трогает цель', async () => {
    const calls = stub(() => jsonResponse(200, DOWN))
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByRole('button', { name: 'Перейти на A2' }))
    await waitFor(() => expect(calls.put[0]).toMatchObject({ currentLevel: 'A2', targetLevel: 'B2' }))
  })

  it('«Пока нет» записывает отказ и прячет карточку, настройки не трогает', async () => {
    const calls = stub(() => jsonResponse(200, UP))
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByRole('button', { name: 'Пока нет' }))
    await waitFor(() => expect(calls.dismissed).toBe(1))
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Пора на уровень выше?' })).not.toBeInTheDocument())
    expect(calls.put).toHaveLength(0)
  })

  it('если сохранить не вышло, карточка остаётся с сообщением', async () => {
    stub(() => jsonResponse(200, UP), { put: () => errorResponse(500, 'internal', 'Внутренняя ошибка') })
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByRole('button', { name: 'Перейти на B2' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('heading', { name: 'Пора на уровень выше?' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Перейти на B2' })).toBeEnabled()
  })

  it('если отказ не записался, карточка остаётся с сообщением', async () => {
    stub(() => jsonResponse(200, UP), { dismiss: () => errorResponse(500, 'internal', 'Внутренняя ошибка') })
    const user = userEvent.setup()
    renderCard()
    await user.click(await screen.findByRole('button', { name: 'Пока нет' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('heading', { name: 'Пора на уровень выше?' })).toBeInTheDocument()
  })
})
