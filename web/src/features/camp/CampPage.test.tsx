import { screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse, renderAt } from '@/test/render'
import { STATUS } from '@/features/learn/fixtures'
import { CampPage } from './CampPage'

afterEach(() => vi.unstubAllGlobals())

function stub(status: object) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.endsWith('/learn/status')) return jsonResponse(200, status)
      if (url.endsWith('/learn/preferences')) return jsonResponse(200, { newWordsMode: 'auto', newWordsPerDay: 12, campCoverage: 0.95 })
      return jsonResponse(200, {})
    }),
  )
}

const render = () => renderAt('/camp', [{ path: '/camp', element: <CampPage /> }])

describe('CampPage', () => {
  it('закрыт: сколько слов осталось и куда идти', async () => {
    stub({ ...STATUS, campWords: 12, campMinWords: 50, campOpen: false })
    render()
    expect(await screen.findByRole('heading', { name: 'Привал откроется после 50 слов' })).toBeInTheDocument()
    expect(screen.getByText('Изучено слов: 12 из 50')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Знакомство' })).toHaveAttribute('href', '/learn')
    expect(screen.getByRole('link', { name: 'Быстрый тест' })).toHaveAttribute('href', '/learn/placement')
    expect(screen.queryByRole('link', { name: 'Упражнения Привала' })).not.toBeInTheDocument()
  })

  it('открыт: счётчик, покрытие и ссылка на упражнения режима', async () => {
    stub({ ...STATUS, campWords: 73, campMinWords: 50, campOpen: true })
    render()
    expect(await screen.findByRole('heading', { name: 'Привал открыт' })).toBeInTheDocument()
    expect(screen.getByText('Изучено слов: 73')).toBeInTheDocument()
    expect(await screen.findByText(/не меньше 95% слов/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Упражнения Привала' })).toHaveAttribute('href', '/exercises?mode=camp')
  })
})
