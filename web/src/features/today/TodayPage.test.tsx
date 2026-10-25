import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { TodayPage } from './TodayPage'

afterEach(() => vi.unstubAllGlobals())

const SETTINGS = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 90, timezone: 'Europe/Moscow' }

const PROGRESS = {
  minutes: 4,
  minimumDay: 10,
  goalMinutes: 15,
  doneToday: false,
  streakDays: 3,
  today: { answers: 5, correct: 4, cardsReviewed: 2, mistakesOpened: 1, mistakesClosed: 0 },
}

function renderToday(
  settings: Record<string, unknown> = SETTINGS,
  queue: () => Response = () => jsonResponse(200, { dueCount: 7, totalCount: 12 }),
  progress: () => Response = () => jsonResponse(200, PROGRESS),
) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.endsWith('/cards/queue')) return queue()
      if (url.endsWith('/progress/today')) return progress()
      return jsonResponse(200, url.endsWith('/settings') ? settings : { login: 'firimon' })
    }),
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
    const button = await screen.findByRole('link', { name: 'Продолжить сессию' })
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
    const el = await screen.findByText(/^4\s+из\s+10\s+мин$/)
    expect(el.textContent).toBe('4\u00a0из\u00a010\u00a0мин')
  })

  it('минуты с долей показываются через запятую: «4,3 из 10 мин»', async () => {
    renderToday(SETTINGS, undefined, () => jsonResponse(200, { ...PROGRESS, minutes: 4.3 }))
    const el = await screen.findByText(/^4,3\s+из\s+10\s+мин$/)
    expect(el).toBeInTheDocument()
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

  it('честно предупреждает, что путь и числа упражнений и ошибок пока демонстрационные', async () => {
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
    const links = within(group).getAllByRole('link')
    expect(links).toHaveLength(3)
    // У ссылки-плитки есть доступное имя (раньше article внутри ссылки оставлял его пустым в настоящем браузере).
    for (const link of links) expect(link).toHaveAccessibleName(/.+/)
  })

  it('показывает настоящую серию и минимальный день с полоской', async () => {
    renderToday()
    await screen.findByRole('heading', { level: 1 })
    expect(screen.getByText('Серия')).toBeInTheDocument()
    expect(await screen.findByText('3 дня')).toBeInTheDocument()
    const bar = screen.getByRole('progressbar', { name: 'Минимальный день' })
    expect(bar).toHaveAttribute('aria-valuenow', '4')
    expect(bar).toHaveAttribute('aria-valuemax', '10')
  })

  it('склоняет «день»: 1 день, 3 дня, 5 дней', async () => {
    for (const [n, text] of [[1, '1 день'], [3, '3 дня'], [5, '5 дней']] as const) {
      const { unmount } = renderToday(SETTINGS, undefined, () => jsonResponse(200, { ...PROGRESS, streakDays: n }))
      expect(await screen.findByText(text)).toBeInTheDocument()
      unmount()
    }
  })

  it('называет день, который спасла заморозка', async () => {
    renderToday(SETTINGS, undefined, () => jsonResponse(200, { ...PROGRESS, freezeUsedOn: '2026-10-24' }))
    expect(await screen.findByText(/заморозка спасла 24 октября/i)).toBeInTheDocument()
  })

  it('без заморозки пометки нет', async () => {
    renderToday()
    await screen.findByText('3 дня')
    expect(screen.queryByText(/заморозка/i)).not.toBeInTheDocument()
  })

  it('кнопка сессии ведёт на /session и называется по прогрессу: нет минут, есть минуты, минимум выполнен', async () => {
    const cases = [
      [{ minutes: 0, doneToday: false }, 'Начать сессию'],
      [{ minutes: 4, doneToday: false }, 'Продолжить сессию'],
      [{ minutes: 10, doneToday: true }, 'Позаниматься ещё'],
    ] as const
    for (const [patch, name] of cases) {
      const { unmount } = renderToday(SETTINGS, undefined, () => jsonResponse(200, { ...PROGRESS, ...patch }))
      const link = await screen.findByRole('link', { name })
      expect(link).toHaveAttribute('href', '/session')
      unmount()
    }
  })

  it('если прогресс не загрузился: сообщение и «Повторить», страница работает, сессию начать можно', async () => {
    renderToday(SETTINGS, undefined, () => errorResponse(500, 'internal', 'Внутренняя ошибка'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Начать сессию' })).toHaveAttribute('href', '/session')
  })
})

describe('плитка «Карточки к повторению»', () => {
  it('показывает настоящее число из очереди и ведёт в раздел карточек', async () => {
    renderToday()
    const tasks = within(await screen.findByRole('region', { name: 'Задачи на сегодня' }))
    const link = tasks.getByRole('link', { name: /карточки к повторению/i })
    expect(link).toHaveAttribute('href', '/cards')
    await waitFor(() => expect(link).toHaveTextContent('7'))
  })

  it('если очередь не загрузилась, плитка показывает прочерк, а экран не падает', async () => {
    renderToday(SETTINGS, () => errorResponse(500, 'internal', 'Внутренняя ошибка'))
    const tasks = within(await screen.findByRole('region', { name: 'Задачи на сегодня' }))
    const link = tasks.getByRole('link', { name: /карточки к повторению/i })
    await waitFor(() => expect(link).toHaveTextContent('—'))
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
  })

  it('нулевое число показывается нулём, а не прочерком', async () => {
    renderToday(SETTINGS, () => jsonResponse(200, { dueCount: 0, totalCount: 3 }))
    const tasks = within(await screen.findByRole('region', { name: 'Задачи на сегодня' }))
    const link = tasks.getByRole('link', { name: /карточки к повторению/i })
    await waitFor(() => expect(link).toHaveTextContent('0'))
    expect(link).not.toHaveTextContent('—')
  })
})

describe('лисёнок и его реплика', () => {
  afterEach(() => vi.useRealTimers())

  const withHour = (hour: number) => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 25, hour, 0, 0))
  }
  const mood = (container: HTMLElement) => ({
    happy: !!container.querySelector('ellipse[fill="var(--coral-soft)"]'),
    sleeping: !!container.querySelector('.fox-zzz'),
    thinking: !!container.querySelector('g[stroke="var(--fox-dark)"] path'),
  })

  it('минимум выполнен: радуется', async () => {
    withHour(14)
    const { container } = renderToday(SETTINGS, undefined, () =>
      jsonResponse(200, { ...PROGRESS, minutes: 12, doneToday: true }),
    )
    expect(await screen.findByText(/минимум на сегодня выполнен/i)).toBeInTheDocument()
    expect(mood(container).happy).toBe(true)
  })

  it('ночью спит, если минимум ещё не выполнен', async () => {
    withHour(3)
    const { container } = renderToday()
    expect(await screen.findByText(/уже поздно/i)).toBeInTheDocument()
    expect(mood(container).sleeping).toBe(true)
  })

  it('нет серии и минут: думает и зовёт начать с малого', async () => {
    withHour(14)
    const { container } = renderToday(SETTINGS, undefined, () =>
      jsonResponse(200, { ...PROGRESS, minutes: 0, streakDays: 0 }),
    )
    expect(await screen.findByText(/начнём с малого/i)).toBeInTheDocument()
    expect(mood(container).thinking).toBe(true)
  })

  it('днём в процессе: называет, сколько осталось до минимума', async () => {
    withHour(14)
    renderToday()
    expect(await screen.findByText(/ещё 6\s+мин/)).toBeInTheDocument()
  })

  it('пока прогресс не загрузился, реплики нет', async () => {
    withHour(14)
    renderToday(SETTINGS, undefined, () => new Promise<Response>(() => {}) as unknown as Response)
    await screen.findByRole('heading', { level: 1 })
    expect(screen.queryByText(/до минимума|начнём с малого|уже поздно/i)).not.toBeInTheDocument()
  })
})
