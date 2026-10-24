import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { AppShell } from './AppShell'

const routes = [
  { path: '/login', element: <p>Форма входа</p> },
  { path: '/', element: <AppShell /> },
]

function stubFetch() {
  const fn = vi.fn(async (url: string) =>
    url.endsWith('/auth/me')
      ? jsonResponse(200, { login: 'firimon' })
      : new Response(null, { status: 204 }),
  )
  vi.stubGlobal('fetch', fn)
  return fn
}

beforeEach(() => localStorage.clear())
afterEach(() => {
  vi.unstubAllGlobals()
  document.documentElement.classList.remove('dark')
})

describe('AppShell', () => {
  it('первой в порядке фокуса идёт ссылка «Перейти к содержимому» на <main>', async () => {
    stubFetch()
    renderAt('/', routes)
    const skip = await screen.findByRole('link', { name: 'Перейти к содержимому' })
    expect(skip).toHaveAttribute('href', '#content')
    expect(screen.getByRole('main')).toHaveAttribute('id', 'content')
  })

  it('показывает навигацию с восемью разделами', async () => {
    stubFetch()
    renderAt('/', routes)
    const nav = await screen.findByRole('navigation', { name: 'Разделы' })
    for (const name of ['Сегодня', 'Карточки', 'Упражнения', 'Слова', 'Ошибки', 'Прогресс', 'Настройки', 'Источники']) {
      expect(nav).toHaveTextContent(name)
    }
  })

  it('показывает имя пользователя', async () => {
    stubFetch()
    renderAt('/', routes)
    expect(await screen.findByText('firimon')).toBeInTheDocument()
  })

  it('кнопка «Выйти» шлёт выход и ведёт на экран входа', async () => {
    const fetchMock = stubFetch()
    renderAt('/', routes)
    await userEvent.setup().click(await screen.findByRole('button', { name: 'Выйти' }))
    expect(await screen.findByText('Форма входа')).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/logout',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('если сервер не ответил на выход, пользователь всё равно оказывается на входе', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (url.endsWith('/auth/me')) return jsonResponse(200, { login: 'firimon' })
        throw new TypeError('Failed to fetch')
      }),
    )
    renderAt('/', routes)
    await userEvent.setup().click(await screen.findByRole('button', { name: 'Выйти' }))
    expect(await screen.findByText('Форма входа')).toBeInTheDocument()
  })

  it('переключатель темы ставит и снимает класс dark и сохраняет выбор', async () => {
    stubFetch()
    renderAt('/', routes)
    const user = userEvent.setup()
    const toggle = await screen.findByRole('button', { name: /тёмная тема/i })
    await user.click(toggle)
    await waitFor(() => expect(document.documentElement).toHaveClass('dark'))
    expect(localStorage.getItem('verst-theme')).toBe('dark')
    await user.click(screen.getByRole('button', { name: /светлая тема/i }))
    await waitFor(() => expect(document.documentElement).not.toHaveClass('dark'))
    expect(localStorage.getItem('verst-theme')).toBe('light')
  })
})
