import { screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { SettingsPage } from './SettingsPage'

afterEach(() => vi.unstubAllGlobals())

describe('SettingsPage', () => {
  it('подставляет сохранённые значения', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(200, { currentLevel: 'B1', targetLevel: 'C1', dailyMinutes: 45, timezone: 'Europe/Moscow' }),
      ),
    )
    renderAt('/settings', [{ path: '/settings', element: <SettingsPage /> }])

    expect(await screen.findByRole('heading', { level: 1, name: 'Настройки' })).toBeInTheDocument()
    expect(document.title).toBe('Настройки — Verst')
    const current = await screen.findByRole('group', { name: 'Текущий уровень' })
    expect(within(current).getByRole('radio', { name: /^B1/ })).toBeChecked()
    expect(within(screen.getByRole('group', { name: 'Цель' })).getByRole('radio', { name: /^C1/ })).toBeChecked()
    expect(screen.getByLabelText(/своё число минут/i)).toHaveValue(45)
    expect(screen.getByLabelText('Часовой пояс')).toHaveValue('Europe/Moscow')
  })
})
