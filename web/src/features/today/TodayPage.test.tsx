import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { TodayPage } from './TodayPage'

afterEach(() => vi.unstubAllGlobals())

function renderToday() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => jsonResponse(200, { login: 'firimon' })),
  )
  return renderAt('/', [{ path: '/', element: <TodayPage /> }])
}

describe('TodayPage', () => {
  it('заголовки идут по порядку: h1, затем h2 для разделов, h3 только внутри них', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    const levels = screen.getAllByRole('heading').map((h) => Number(h.tagName[1]))
    for (let i = 1; i < levels.length; i++) expect(levels[i] - levels[i - 1]).toBeLessThanOrEqual(1)
  })

  it('единицы не отрываются от чисел: «4 из 10 мин» с неразрывными пробелами', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    // getByText сам сворачивает неразрывные пробелы, поэтому сверяем сырой textContent.
    const el = screen.getByText(/^4\s+из\s+10\s+мин$/)
    expect(el.textContent).toBe('4\u00a0из\u00a010\u00a0мин')
  })

  it('приветствует по имени и называет цель', async () => {
    renderToday()
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Привет, firimon' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/Цель: B1 → B2/)).toBeInTheDocument()
  })

  it('честно предупреждает, что данные демонстрационные', async () => {
    renderToday()
    expect(await screen.findByRole('note')).toHaveTextContent(/демо-данные/i)
  })

  it('показывает путь по уровням', async () => {
    renderToday()
    expect(await screen.findByRole('list', { name: 'Путь по уровням' })).toBeInTheDocument()
  })

  it('показывает три задачи дня с числами', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    const group = screen.getByRole('region', { name: 'Задачи на сегодня' })
    expect(within(group).getByText('Карточки к повторению')).toBeInTheDocument()
    expect(within(group).getByText('Упражнения на слабые темы')).toBeInTheDocument()
    expect(within(group).getByText('Работа над ошибками')).toBeInTheDocument()
    expect(within(group).getAllByRole('article')).toHaveLength(3)
  })

  it('показывает серию и минимальный день с полоской', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    expect(screen.getByText('Серия')).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: 'Минимальный день' })).toBeInTheDocument()
  })

  it('кнопка «Начать сессию» пока недоступна и объясняет почему', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    const button = screen.getByRole('button', { name: 'Начать сессию' })
    expect(button).toBeDisabled()
    expect(screen.getByText(/словарь и упражнения ещё не подключены/i)).toBeInTheDocument()
  })
})
