import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { ProgressPage } from './ProgressPage'

afterEach(() => vi.unstubAllGlobals())

const SETTINGS = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 30, timezone: 'Europe/Moscow' }

function overview(over: Record<string, unknown> = {}) {
  const daily = Array.from({ length: 90 }, (_, i) => {
    const d = new Date(2026, 6, 28 + i)
    const day = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    return { day, minutes: 0, cumulativeHours: 0 }
  })
  return { daysSinceStart: 0, studiedMinutes: 0, paceMinutesPerDay: 0, paceReady: false, daily, ...over }
}

const rating = (topic: string, r: number, attempts = 10, deviation = 80) => ({ topic, rating: r, deviation, attempts })

type Opts = {
  settings?: Record<string, unknown>
  overview?: () => Response
  topics?: ReturnType<typeof rating>[]
}

function renderPage(opts: Opts = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.endsWith('/progress/overview')) return opts.overview ? opts.overview() : jsonResponse(200, overview())
      if (url.endsWith('/exercises/topics')) return jsonResponse(200, { topics: opts.topics ?? [] })
      if (url.endsWith('/settings')) return jsonResponse(200, opts.settings ?? SETTINGS)
      return jsonResponse(200, { login: 'firimon' })
    }),
  )
  return renderAt('/progress', [
    { path: '/progress', element: <ProgressPage /> },
    { path: '/login', element: <p>Экран входа</p> },
    { path: '/settings', element: <p>Настройки</p> },
  ])
}

const ready = (over: Record<string, unknown> = {}) =>
  () => jsonResponse(200, overview({ startedOn: '2026-10-05', daysSinceStart: 21, studiedMinutes: 600, paceMinutesPerDay: 30, paceReady: true, ...over }))

describe('ProgressPage: прогноз', () => {
  it('заголовок «Прогресс»', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Прогресс' })).toBeInTheDocument()
  })

  it('нет занятий: «Собираю данные», счётчик суток и пустой график', async () => {
    renderPage()
    expect(await screen.findByText(/собираю данные/i)).toBeInTheDocument()
    expect(screen.getByText(/0 из 14 суток/)).toBeInTheDocument()
    expect(screen.getByText('Начни сессию, и здесь появится линия')).toBeInTheDocument()
    expect(screen.queryByTestId('hours-line')).not.toBeInTheDocument()
  })

  it('мало суток занятий: «5 из 14 суток»', async () => {
    renderPage({ overview: () => jsonResponse(200, overview({ startedOn: '2026-10-21', daysSinceStart: 5, studiedMinutes: 95 })) })
    expect(await screen.findByText(/5 из 14 суток/)).toBeInTheDocument()
    expect(screen.queryByText(/примерно/)).not.toBeInTheDocument()
  })

  it('оценка по реальному темпу: срок, даты и «что изменить»', async () => {
    renderPage({ overview: ready() })
    expect(await screen.findByText(/До B2 примерно/)).toBeInTheDocument()
    expect(screen.getByText(/к \d{1,2} [а-я]+( \d{4})? – \d{1,2} [а-я]+( \d{4})?/)).toBeInTheDocument()
    expect(screen.getByText(/Что изменить: \+10 минут в день сократят срок примерно на \d+ /)).toBeInTheDocument()
    expect(screen.getByText(/считаются только минуты в сервисе/i)).toBeInTheDocument()
  })

  it('темп готов, но 0: «Сейчас занятий нет», даты нет', async () => {
    renderPage({ overview: ready({ paceMinutesPerDay: 0 }) })
    expect(await screen.findByText(/Сейчас занятий нет/)).toBeInTheDocument()
    expect(screen.queryByText(/примерно/)).not.toBeInTheDocument()
  })

  it('очень низкий темп: «срок больше 10 лет», даты и подсказки нет', async () => {
    renderPage({ overview: ready({ paceMinutesPerDay: 0.1, studiedMinutes: 20 }) })
    expect(await screen.findByText(/срок больше 10 лет/i)).toBeInTheDocument()
    expect(screen.queryByText(/примерно/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Что изменить/)).not.toBeInTheDocument()
  })

  it('набрано больше нужных часов: без даты', async () => {
    renderPage({ overview: ready({ studiedMinutes: 25000 }) })
    expect(await screen.findByText(/Набрано больше нужных часов/)).toBeInTheDocument()
    expect(screen.queryByText(/До B2 примерно/)).not.toBeInTheDocument()
  })

  it('цель не выше уровня: «Цель достигнута» и ссылка в настройки', async () => {
    renderPage({ settings: { ...SETTINGS, currentLevel: 'B2' }, overview: ready() })
    expect(await screen.findByText(/цель достигнута/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /выбери цель повыше/i })).toHaveAttribute('href', '/settings')
  })
})

describe('ProgressPage: часы, темп и карта тем', () => {
  it('полоса «Набрано … из …» и график скрыт от читалок, таблица по дням в details', async () => {
    renderPage({ overview: ready() })
    const bar = await screen.findByRole('progressbar', { name: 'Набрано часов' })
    expect(bar).toHaveAttribute('aria-valuenow', '10')
    expect(screen.getByText(/10 ч из 263–350 ч/)).toBeInTheDocument()
    const details = screen.getByText('Данные по дням').closest('details')!
    expect(within(details).getAllByRole('row')).toHaveLength(91) // шапка + 90 суток
    expect(screen.getByTestId('hours-line')).toBeInTheDocument()
  })

  it('темп: готов и не готов', async () => {
    const { unmount } = renderPage({ overview: ready() })
    expect(await screen.findByText(/Средний темп за 14 дней: 30 мин в день/)).toBeInTheDocument()
    expect(screen.getByText(/Занимаешься с 5 октября/)).toBeInTheDocument()
    expect(screen.getByText(/Набрано: 10 ч/)).toBeInTheDocument()
    unmount()
    renderPage({ overview: () => jsonResponse(200, overview({ startedOn: '2026-10-21', daysSinceStart: 5, studiedMinutes: 95 })) })
    expect(await screen.findByText(/Темп покажу после 14 суток занятий/)).toBeInTheDocument()
  })

  it('карта тем по возрастанию рейтинга; «слабая тема» текстом у пяти самых слабых надёжных', async () => {
    const topics = [
      rating('grammar:future', 1500), rating('grammar:articles', 1380), rating('pos:verb', 1450), rating('grammar:modals', 1300),
      rating('level:B1', 1600), rating('grammar:prepositions', 1250), rating('grammar:past_simple', 1420, 2),
    ]
    renderPage({ topics })
    const list = await screen.findByRole('list', { name: 'Карта тем' })
    const names = within(list).getAllByRole('listitem').map((li) => li.textContent)
    expect(names[0]).toContain('Предлоги')
    expect(names[names.length - 1]).toContain('Уровень B1')
    expect(within(list).getAllByText('слабая тема')).toHaveLength(5)
    const few = within(list).getAllByRole('listitem').find((li) => li.textContent?.includes('Прошедшее простое'))!
    expect(few).toHaveTextContent('мало ответов')
    expect(few).not.toHaveTextContent('слабая тема')
  })

  it('нет тем: подсказка вместо списка', async () => {
    renderPage({ topics: [] })
    expect(await screen.findByText(/Пока нет данных — ответь на несколько упражнений/)).toBeInTheDocument()
  })
})

describe('ProgressPage: ошибки', () => {
  it('ошибка загрузки: сообщение и «Повторить», который перезапрашивает данные', async () => {
    let calls = 0
    renderPage({ overview: () => (calls++ === 0 ? errorResponse(500, 'internal', 'Внутренняя ошибка') : jsonResponse(200, overview())) })
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByText(/собираю данные/i)).toBeInTheDocument()
    await waitFor(() => expect(calls).toBe(2))
  })

  it('401 ведёт на вход', async () => {
    renderPage({ overview: () => errorResponse(401, 'unauthorized', 'Нужен вход') })
    expect(await screen.findByText('Экран входа')).toBeInTheDocument()
  })
})
