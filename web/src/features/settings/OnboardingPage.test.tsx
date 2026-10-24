import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { OnboardingPage } from './OnboardingPage'

afterEach(() => vi.unstubAllGlobals())

describe('OnboardingPage', () => {
  it('ведёт по трём шагам и после сохранения переходит на главную', async () => {
    const saved = { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 15, timezone: 'UTC' }
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse(200, saved))
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    renderAt('/onboarding', [
      { path: '/onboarding', element: <OnboardingPage /> },
      { path: '/', element: <p>Главная</p> },
    ])

    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    expect(document.title).toBe('Настройка — Verst')
    await user.click(
      within(screen.getByRole('group', { name: 'Текущий уровень' })).getByRole('radio', { name: /^B1/ }),
    )
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.click(screen.getByRole('button', { name: 'Начать' }))

    expect(await screen.findByText('Главная')).toBeInTheDocument()
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(fetchMock.mock.calls[0][1]?.method).toBe('PUT')
  })
})
