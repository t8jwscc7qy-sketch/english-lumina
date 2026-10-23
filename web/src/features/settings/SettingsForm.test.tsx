import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Settings } from '@/api/client'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { toast } from 'sonner'
import { SettingsForm } from './SettingsForm'

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const saved: Settings = {
  currentLevel: 'B1',
  targetLevel: 'B2',
  dailyMinutes: 90,
  timezone: 'Europe/Moscow',
}

beforeEach(() => vi.mocked(toast.success).mockClear())
afterEach(() => vi.unstubAllGlobals())

function render(props: { initial: Settings | null; mode: 'onboarding' | 'settings'; onSaved?: () => void }) {
  const onSaved = props.onSaved ?? vi.fn()
  renderAt('/', [
    { path: '/', element: <SettingsForm initial={props.initial} mode={props.mode} onSaved={onSaved} /> },
  ])
  return { onSaved }
}

function stubPut(respond: () => Response | Promise<Response>) {
  const fn = vi.fn(async (_url: string, _init?: RequestInit) => respond())
  vi.stubGlobal('fetch', fn)
  return fn
}

const group = (name: string) => within(screen.getByRole('group', { name }))

describe('онбординг', () => {
  it('нельзя уйти с первого шага, не выбрав уровень', () => {
    render({ initial: null, mode: 'onboarding' })
    expect(screen.getByRole('button', { name: 'Дальше' })).toBeDisabled()
  })

  it('три шага: уровень → цель → минуты; цель сама встаёт на следующий уровень', async () => {
    const user = userEvent.setup()
    render({ initial: null, mode: 'onboarding' })

    await user.click(group('Текущий уровень').getByRole('radio', { name: /^B1/ }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(group('Цель').getByRole('radio', { name: /^B2/ })).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(screen.getByRole('button', { name: 'Начать' })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Назад' }))
    expect(group('Цель').getByRole('radio', { name: /^B2/ })).toBeChecked()
  })

  it('счётчик пересчитывается: B1→B2 при 90 минутах — 6–8 месяцев', async () => {
    const user = userEvent.setup()
    render({ initial: null, mode: 'onboarding' })
    expect(screen.getByRole('status')).toHaveTextContent(/выбери уровень/i)

    await user.click(group('Текущий уровень').getByRole('radio', { name: /^B1/ }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    const minutes = screen.getByLabelText(/своё число минут/i)
    await user.clear(minutes)
    await user.type(minutes, '90')
    expect(screen.getByRole('status')).toHaveTextContent('6–8 месяцев')
  })

  it('минуты вне 5–240 не дают сохранить и прячут прогноз', async () => {
    const user = userEvent.setup()
    render({ initial: saved, mode: 'settings' })
    const minutes = screen.getByLabelText(/своё число минут/i)
    await user.clear(minutes)
    await user.type(minutes, '241')
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()
    expect(screen.getByRole('status')).not.toHaveTextContent('месяц')
  })
})

describe('двойной клик в онбординге', () => {
  it('по «Дальше» на шаге 2 не сохраняет настройки и не перескакивает шаг 3', async () => {
    const user = userEvent.setup()
    const put = stubPut(() => jsonResponse(200, saved))
    render({ initial: null, mode: 'onboarding' })
    await user.click(group('Текущий уровень').getByRole('radio', { name: /^B1/ }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.dblClick(screen.getByRole('button', { name: 'Дальше' }))
    expect(put).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Начать' })).toBeEnabled()
    expect(screen.getByLabelText(/своё число минут/i)).toBeInTheDocument()
  })
})

describe('настройки', () => {
  it('уровень выше цели поднимает цель', async () => {
    const user = userEvent.setup()
    render({ initial: saved, mode: 'settings' })
    await user.click(group('Текущий уровень').getByRole('radio', { name: /^C1/ }))
    expect(group('Цель').getByRole('radio', { name: /^C1/ })).toBeChecked()
    expect(group('Цель').getByRole('radio', { name: /^B2/ })).toBeDisabled()
  })

  it('сохраняет PUT-ом, зовёт onSaved один раз и показывает уведомление', async () => {
    const user = userEvent.setup()
    const put = stubPut(() => jsonResponse(200, saved))
    const { onSaved } = render({ initial: saved, mode: 'settings' })
    await user.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    const [url, init = {}] = put.mock.calls[0]
    expect(url).toBe('/api/v1/settings')
    expect(init.method).toBe('PUT')
    expect(JSON.parse(init.body as string)).toEqual(saved)
    expect(toast.success).toHaveBeenCalledTimes(1)
  })

  it('двойной клик по «Сохранить» отправляет один запрос', async () => {
    const user = userEvent.setup()
    const put = stubPut(
      () => new Promise<Response>((resolve) => setTimeout(() => resolve(jsonResponse(200, saved)), 50)),
    )
    const { onSaved } = render({ initial: saved, mode: 'settings' })
    await user.dblClick(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(put).toHaveBeenCalledTimes(1)
  })

  it('ошибка сервера показывается, введённое остаётся, onSaved не зовётся', async () => {
    const user = userEvent.setup()
    stubPut(() => errorResponse(400, 'invalid_request', 'Дата цели уже прошла'))
    const { onSaved } = render({ initial: saved, mode: 'settings' })
    await user.click(screen.getByRole('button', { name: 'Сохранить' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Дата цели уже прошла')
    expect(screen.getByLabelText(/своё число минут/i)).toHaveValue(90)
    expect(onSaved).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeEnabled()
  })

  it('в онбординге уведомление не показывается', async () => {
    const user = userEvent.setup()
    stubPut(() => jsonResponse(200, saved))
    const { onSaved } = render({ initial: null, mode: 'onboarding' })
    await user.click(group('Текущий уровень').getByRole('radio', { name: /^B1/ }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await user.click(screen.getByRole('button', { name: 'Начать' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(toast.success).not.toHaveBeenCalled()
  })
})
