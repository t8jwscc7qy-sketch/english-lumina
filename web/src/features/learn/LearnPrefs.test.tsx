import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { LearnPrefs } from './LearnPrefs'

afterEach(() => vi.unstubAllGlobals())

function stub() {
  const puts: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'PUT') {
        puts.push(JSON.parse(String(init.body)))
        return jsonResponse(200, puts.at(-1))
      }
      return jsonResponse(200, { newWordsMode: 'auto', newWordsPerDay: 12, campCoverage: 0.95 })
    }),
  )
  return puts
}

const render = () => renderAt('/', [{ path: '/', element: <LearnPrefs /> }])

describe('LearnPrefs', () => {
  it('показывает значения по умолчанию и сохраняет изменённые', async () => {
    const puts = stub()
    const user = userEvent.setup()
    render()
    expect(await screen.findByLabelText('Не больше слов в день')).toHaveValue('12')
    expect(screen.getByLabelText(/Покрытие текста/)).toHaveValue('95')
    await user.click(screen.getByRole('radio', { name: 'Столько, сколько я укажу' }))
    const perDay = screen.getByLabelText('Слов в день')
    await user.clear(perDay)
    await user.type(perDay, '9')
    await user.click(screen.getByRole('button', { name: 'Применить' }))
    expect(await screen.findByText('Сохранено')).toBeInTheDocument()
    expect(puts[0]).toEqual({ newWordsMode: 'manual', newWordsPerDay: 9, campCoverage: 0.95 })
  })

  it('значения вне границ блокируют сохранение', async () => {
    const puts = stub()
    const user = userEvent.setup()
    render()
    const perDay = await screen.findByLabelText('Не больше слов в день')
    await user.clear(perDay)
    await user.type(perDay, '31')
    expect(screen.getByRole('button', { name: 'Применить' })).toBeDisabled()
    await user.clear(perDay)
    await user.type(perDay, '10')
    const cov = screen.getByLabelText(/Покрытие текста/)
    await user.clear(cov)
    await user.type(cov, '100')
    expect(screen.getByRole('button', { name: 'Применить' })).toBeDisabled()
    expect(puts).toHaveLength(0)
  })
})
