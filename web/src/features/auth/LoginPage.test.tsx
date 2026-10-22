import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { LoginPage } from './LoginPage'

const routes = [
  { path: '/login', element: <LoginPage /> },
  { path: '/', element: <p>Главная</p> },
]

function stubFetch(handler: (url: string, init?: RequestInit) => Response | Error) {
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    const r = handler(url, init)
    if (r instanceof Error) throw r
    return r
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

async function fill(login: string, password: string) {
  const user = userEvent.setup()
  if (login) await user.type(screen.getByLabelText('Логин'), login)
  if (password) await user.type(screen.getByLabelText('Пароль'), password)
  await user.click(screen.getByRole('button', { name: 'Войти' }))
  return user
}

afterEach(() => vi.unstubAllGlobals())

describe('LoginPage', () => {
  it('показывает заголовок, поля и кнопку', () => {
    renderAt('/login', routes)
    expect(screen.getByRole('heading', { name: /вход/i })).toBeInTheDocument()
    expect(screen.getByLabelText('Логин')).toHaveAttribute('autocomplete', 'username')
    expect(screen.getByLabelText('Пароль')).toHaveAttribute('type', 'password')
    expect(screen.getByLabelText('Пароль')).toHaveAttribute('autocomplete', 'current-password')
    expect(screen.getByRole('button', { name: 'Войти' })).toBeEnabled()
  })

  it('при успехе отправляет данные и уходит на главную', async () => {
    const fetchMock = stubFetch((url) =>
      url.endsWith('/auth/me')
        ? jsonResponse(200, { login: 'firimon' })
        : new Response(null, { status: 204 }),
    )
    renderAt('/login', routes)
    await fill('firimon', 'secret-password-1')
    expect(await screen.findByText('Главная')).toBeInTheDocument()
    const loginCall = fetchMock.mock.calls.find(([u]) => u === '/api/v1/auth/login')!
    expect(JSON.parse(loginCall[1]!.body as string)).toEqual({
      login: 'firimon',
      password: 'secret-password-1',
    })
  })

  it('неверный пароль: сообщение сервера, поля помечены ошибкой, остаёмся на форме', async () => {
    stubFetch(() => errorResponse(401, 'invalid_credentials', 'Неверный логин или пароль'))
    renderAt('/login', routes)
    await fill('firimon', 'wrong-password-1')
    expect(await screen.findByRole('alert')).toHaveTextContent('Неверный логин или пароль')
    expect(screen.getByLabelText('Логин')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText('Пароль')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('button', { name: 'Войти' })).toBeEnabled()
    expect(screen.queryByText('Главная')).not.toBeInTheDocument()
  })

  it.each([
    [429, 'too_many_attempts', 'Слишком много попыток, попробуй через минуту'],
    [503, 'busy', 'Сервер занят, повтори через несколько секунд'],
    [400, 'invalid_request', 'Проверь логин и пароль: логин 3–32 символа, пароль до 256 байт'],
  ])('ответ %i показывает понятное сообщение', async (status, code, message) => {
    stubFetch(() => errorResponse(status, code, message))
    renderAt('/login', routes)
    await fill('firimon', 'secret-password-1')
    expect(await screen.findByRole('alert')).toHaveTextContent(message)
  })

  it('потеря сети: сообщение про сервер, форма не зависает', async () => {
    stubFetch(() => new TypeError('Failed to fetch'))
    renderAt('/login', routes)
    await fill('firimon', 'secret-password-1')
    expect(await screen.findByRole('alert')).toHaveTextContent(/нет связи с сервером/i)
    expect(screen.getByRole('button', { name: 'Войти' })).toBeEnabled()
  })

  it('пустые поля: подсказка без запроса к серверу', async () => {
    const fetchMock = stubFetch(() => new Response(null, { status: 204 }))
    renderAt('/login', routes)
    await fill('', '')
    expect(await screen.findByRole('alert')).toHaveTextContent('Введи логин и пароль')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('пока идёт запрос, кнопка заблокирована и подписана «Входим…»', async () => {
    let release: (r: Response) => void = () => {}
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise<Response>((resolve) => (release = resolve))),
    )
    renderAt('/login', routes)
    await fill('firimon', 'secret-password-1')
    const button = await screen.findByRole('button', { name: 'Входим…' })
    expect(button).toBeDisabled()
    release(errorResponse(401, 'invalid_credentials', 'Неверный логин или пароль'))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Войти' })).toBeEnabled())
  })
})

describe('LoginPage: фокус после ошибки', () => {
  it('пустая форма: фокус на пустом логине', async () => {
    renderAt('/login', routes)
    await fill('', '')
    await screen.findByRole('alert')
    expect(screen.getByLabelText('Логин')).toHaveFocus()
  })

  it('неверный пароль: фокус на пароле, чтобы сразу набрать заново', async () => {
    stubFetch(() => errorResponse(401, 'invalid_credentials', 'Неверный логин или пароль'))
    renderAt('/login', routes)
    await fill('firimon', 'wrong-password-1')
    await screen.findByRole('alert')
    await waitFor(() => expect(screen.getByLabelText('Пароль')).toHaveFocus())
  })
})
