import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { formatDue } from './mistakes'
import { MistakesPage } from './MistakesPage'

// Часы закреплены: страница считает срок от реального времени, а данные собраны от NOW. Подменяется только Date, таймеры настоящие.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(NOW)
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

const NOW = new Date('2026-10-24T12:00:00Z')
const iso = (hours: number) => new Date(NOW.getTime() + hours * 3600_000).toISOString()

const mistake = (id: number, over: Record<string, unknown> = {}) => ({
  id,
  topic: 'grammar:articles',
  kind: 'gap',
  prompt: 'She is _____ engineer.',
  given: 'a',
  answer: 'an',
  streak: 1,
  nextDue: iso(-1),
  firstWrongAt: iso(-48),
  ...over,
})

function renderPage() {
  return renderAt('/mistakes', [
    { path: '/mistakes', element: <MistakesPage /> },
    { path: '/login', element: <p>Экран входа</p> },
  ])
}

describe('formatDue', () => {
  const local = (d: number, h: number, m = 0) => new Date(2026, 9, d, h, m)

  it('прошлое, настоящее и неверная дата: ждёт повтора', () => {
    expect(formatDue(iso(-5), NOW)).toBe('ждёт повтора')
    expect(formatDue(NOW.toISOString(), NOW)).toBe('ждёт повтора')
    expect(formatDue('не дата', NOW)).toBe('ждёт повтора')
  })
  it('позже в тот же календарный день: вернётся сегодня', () => {
    expect(formatDue(local(24, 12).toISOString(), local(24, 8))).toBe('вернётся сегодня')
  })
  it('следующая календарная дата: вернётся завтра, хоть и через 20 минут', () => {
    expect(formatDue(local(25, 0, 10).toISOString(), local(24, 23, 50))).toBe('вернётся завтра')
  })
  it('завтра в 12:00 при просмотре в 08:00 (28 часов): вернётся завтра, не «через 2 дн.»', () => {
    expect(formatDue(local(25, 12).toISOString(), local(24, 8))).toBe('вернётся завтра')
  })
  it('через три календарных дня: через 3 дн.', () => {
    expect(formatDue(local(27, 6).toISOString(), local(24, 23))).toBe('через 3 дн.')
    expect(formatDue(iso(72), NOW)).toBe('через 3 дн.')
  })
})

describe('MistakesPage', () => {
  it('показывает три числа, ошибки по темам, ответы и серию', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(200, {
          open: [
            mistake(1),
            mistake(2, { topic: 'skill:order', kind: 'order', prompt: 'Она инженер.', given: 'She an is engineer', answer: 'She is an engineer', streak: 0, nextDue: iso(72) }),
          ],
          counts: { open: 7, dueNow: 3, closedThisWeek: 2 },
        }),
      ),
    )
    renderPage()
    expect(await screen.findByRole('heading', { level: 1, name: 'Ошибки' })).toBeInTheDocument()
    expect(document.title).toBe('Ошибки — Verst')
    expect((await screen.findByText('Открыто')).closest('div')).toHaveTextContent('7')
    expect(screen.getByText('Ждут повтора сейчас').closest('div')).toHaveTextContent('3')
    expect(screen.getByText('Закрыто за неделю').closest('div')).toHaveTextContent('2')
    expect(screen.getByRole('heading', { level: 2, name: 'Артикли' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Порядок слов' })).toBeInTheDocument()
    const items = screen.getAllByRole('listitem')
    const first = within(items[0])
    expect(first.getByText('She is _____ engineer.')).toBeInTheDocument()
    expect(first.getByText(/Твой ответ:/)).toHaveTextContent('Твой ответ: a')
    expect(first.getByText(/Верно:/)).toHaveTextContent('Верно: an')
    expect(first.getByText('ждёт повтора')).toBeInTheDocument()
    expect(first.getByText(/1 из 2/)).toBeInTheDocument()
    const second = within(items[1])
    expect(second.getByText('Она инженер.')).toBeInTheDocument()
    expect(second.getByText('через 3 дн.')).toBeInTheDocument()
    expect(second.getByText(/0 из 2/)).toBeInTheDocument()
  })

  it('пустое состояние без чувства вины', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { open: [], counts: { open: 0, dueNow: 0, closedThisWeek: 0 } })))
    renderPage()
    expect(await screen.findByText('Пока ошибок нет. Ошибки из упражнений появятся здесь и вернутся через день')).toBeInTheDocument()
    expect(document.querySelector('[data-mood]')).toHaveAttribute('data-mood', 'happy')
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
  })

  it('ошибка загрузки: сообщение и «Повторить»', async () => {
    let n = 0
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        ++n === 1
          ? errorResponse(500, 'internal', 'Сервер ответил неожиданно.')
          : jsonResponse(200, { open: [], counts: { open: 0, dueNow: 0, closedThisWeek: 0 } }),
      ),
    )
    const user = userEvent.setup()
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('Сервер ответил неожиданно.')
    await user.click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByText(/Пока ошибок нет/)).toBeInTheDocument()
  })

  it('401 ведёт на вход', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errorResponse(401, 'unauthorized', 'Нужно войти.')))
    renderPage()
    expect(await screen.findByText('Экран входа')).toBeInTheDocument()
  })

  it('текст из базы выводится как текст, не как разметка', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(200, { open: [mistake(1, { given: '<b>x</b>' })], counts: { open: 1, dueNow: 1, closedThisWeek: 0 } }),
      ),
    )
    const { container } = renderPage()
    expect(await screen.findByText(/<b>x<\/b>/)).toBeInTheDocument()
    expect(container.querySelector('b')).toBeNull()
  })
})
