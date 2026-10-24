import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { RedirectIfAuthed, RedirectIfOnboarded, RequireAuth, RequireOnboarded } from './guards'

const routes = [
  {
    path: '/login',
    element: (
      <RedirectIfAuthed>
        <p>Форма входа</p>
      </RedirectIfAuthed>
    ),
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <p>Секретная страница</p>
      </RequireAuth>
    ),
  },
]

afterEach(() => vi.unstubAllGlobals())

function stub(fn: () => Response | Promise<Response> | Error) {
  const mock = vi.fn(async () => {
    const r = await fn()
    if (r instanceof Error) throw r
    return r
  })
  vi.stubGlobal('fetch', mock)
  return mock
}

describe('RequireAuth', () => {
  it('вошедшему показывает страницу', async () => {
    stub(() => jsonResponse(200, { login: 'firimon' }))
    renderAt('/', routes)
    expect(await screen.findByText('Секретная страница')).toBeInTheDocument()
  })

  it('невошедшего отправляет на вход', async () => {
    stub(() => errorResponse(401, 'unauthorized', 'Нужно войти'))
    renderAt('/', routes)
    expect(await screen.findByText('Форма входа')).toBeInTheDocument()
    expect(screen.queryByText('Секретная страница')).not.toBeInTheDocument()
  })

  it('пока проверяет сессию, говорит об этом и не мигает страницей', () => {
    stub(() => new Promise<Response>(() => {}) as unknown as Response)
    renderAt('/', routes)
    expect(screen.getByRole('status')).toHaveTextContent(/проверяем/i)
    expect(screen.queryByText('Секретная страница')).not.toBeInTheDocument()
  })

  it('если сервер недоступен, показывает ошибку с кнопкой «Повторить», а не белый экран', async () => {
    let up = false
    stub(() => (up ? jsonResponse(200, { login: 'firimon' }) : new TypeError('Failed to fetch')))
    renderAt('/', routes)
    expect(await screen.findByRole('alert')).toHaveTextContent(/нет связи с сервером/i)
    up = true
    await userEvent.setup().click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByText('Секретная страница')).toBeInTheDocument()
  })
})

describe('RedirectIfAuthed', () => {
  it('вошедшего с экрана входа уводит на главную', async () => {
    stub(() => jsonResponse(200, { login: 'firimon' }))
    renderAt('/login', routes)
    expect(await screen.findByText('Секретная страница')).toBeInTheDocument()
  })

  it('невошедшему показывает форму входа', async () => {
    stub(() => errorResponse(401, 'unauthorized', 'Нужно войти'))
    renderAt('/login', routes)
    expect(await screen.findByText('Форма входа')).toBeInTheDocument()
  })

  it('если сервер недоступен, форму входа всё равно показываем (она сама сообщит об ошибке)', async () => {
    stub(() => new TypeError('Failed to fetch'))
    renderAt('/login', routes)
    expect(await screen.findByText('Форма входа')).toBeInTheDocument()
  })
})

const onboardingRoutes = [
  {
    path: '/onboarding',
    element: (
      <RedirectIfOnboarded>
        <p>Онбординг</p>
      </RedirectIfOnboarded>
    ),
  },
  {
    path: '/',
    element: (
      <RequireOnboarded>
        <p>Рабочий раздел</p>
      </RequireOnboarded>
    ),
  },
]

const savedSettings = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 90, timezone: 'UTC' }
const needOnboarding = () => errorResponse(404, 'onboarding_required', 'Сначала настрой цель и уровень')

describe('RequireOnboarded', () => {
  it('с настройками показывает раздел', async () => {
    stub(() => jsonResponse(200, savedSettings))
    renderAt('/', onboardingRoutes)
    expect(await screen.findByText('Рабочий раздел')).toBeInTheDocument()
  })

  it('без настроек отправляет на онбординг', async () => {
    stub(needOnboarding)
    renderAt('/', onboardingRoutes)
    expect(await screen.findByText('Онбординг')).toBeInTheDocument()
    expect(screen.queryByText('Рабочий раздел')).not.toBeInTheDocument()
  })

  it('пока проверяет настройки, говорит об этом', () => {
    stub(() => new Promise<Response>(() => {}) as unknown as Response)
    renderAt('/', onboardingRoutes)
    expect(screen.getByRole('status')).toHaveTextContent(/проверяем настройки/i)
  })

  it('сбой сервера не отправляет в онбординг: ошибка и «Повторить»', async () => {
    let up = false
    stub(() => (up ? jsonResponse(200, savedSettings) : errorResponse(500, 'internal', 'Внутренняя ошибка')))
    renderAt('/', onboardingRoutes)
    expect(await screen.findByRole('alert')).toHaveTextContent(/внутренняя ошибка/i)
    expect(screen.queryByText('Онбординг')).not.toBeInTheDocument()
    up = true
    await userEvent.setup().click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByText('Рабочий раздел')).toBeInTheDocument()
  })
})

describe('RedirectIfOnboarded', () => {
  it('с настройками уводит с онбординга на главную', async () => {
    stub(() => jsonResponse(200, savedSettings))
    renderAt('/onboarding', onboardingRoutes)
    expect(await screen.findByText('Рабочий раздел')).toBeInTheDocument()
  })

  it('без настроек показывает онбординг', async () => {
    stub(needOnboarding)
    renderAt('/onboarding', onboardingRoutes)
    expect(await screen.findByText('Онбординг')).toBeInTheDocument()
  })
})
