import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { TodayPage } from './TodayPage'

afterEach(() => vi.unstubAllGlobals())

const SETTINGS = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 90, timezone: 'Europe/Moscow' }

function renderToday(settings: Record<string, unknown> = SETTINGS) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) =>
      jsonResponse(200, url.endsWith('/settings') ? settings : { login: 'firimon' }),
    ),
  )
  return renderAt('/', [{ path: '/', element: <TodayPage /> }])
}

describe('TodayPage', () => {
  it('каждая карточка задачи — ссылка на свой раздел', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    const hrefs = within(screen.getByRole('region', { name: 'Задачи на сегодня' }))
      .getAllByRole('link')
      .map((a) => a.getAttribute('href'))
    expect(hrefs).toEqual(['/cards', '/exercises', '/mistakes'])
  })

  it('кнопка «Начать сессию» стоит раньше карточек в порядке чтения, а не в самом низу', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    const button = screen.getByRole('button', { name: 'Начать сессию' })
    const tasks = screen.getByRole('region', { name: 'Задачи на сегодня' })
    expect(button.compareDocumentPosition(tasks) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

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

  it('берёт цель и минуты из настроек и показывает прогноз: B1→B2 при 90 минутах — 6–8 месяцев', async () => {
    renderToday()
    const goal = await screen.findByText(/Цель: B1 → B2/)
    expect(goal).toHaveTextContent(/90\s+мин в день/)
    expect(screen.getByText(/6–8 месяцев/)).toBeInTheDocument()
  })

  it('если цель совпадает с уровнем, говорит, что она достигнута, и не выдумывает срок', async () => {
    renderToday({ ...SETTINGS, currentLevel: 'B2' })
    expect(await screen.findByText(/Цель: B2 → B2/)).toBeInTheDocument()
    expect(screen.getByText(/цель достигнута/i)).toBeInTheDocument()
    expect(screen.queryByText(/месяц/)).not.toBeInTheDocument()
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
