import { screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { STATUS } from './fixtures'
import { LearnCard } from './LearnCard'

afterEach(() => vi.unstubAllGlobals())

const render = () => renderAt('/', [{ path: '/', element: <LearnCard /> }])
const stub = (r: () => Response) => vi.stubGlobal('fetch', vi.fn(async () => r()))

describe('LearnCard', () => {
  it('показывает число слов сегодня, причину и две ссылки', async () => {
    stub(() => jsonResponse(200, STATUS))
    render()
    expect(await screen.findByRole('heading', { name: 'Знакомство' })).toBeInTheDocument()
    expect(screen.getByText('Новых слов сегодня: 1 из 5')).toBeInTheDocument()
    expect(screen.getByText('Столько слов поместится в твои минуты')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Начать знакомство' })).toHaveAttribute('href', '/learn')
    expect(screen.getByRole('link', { name: 'Быстрый тест' })).toHaveAttribute('href', '/learn/placement')
  })

  it('лимит исчерпан — без «Начать знакомство»', async () => {
    stub(() => jsonResponse(200, { ...STATUS, remaining: 0, newToday: 5 }))
    render()
    await screen.findByRole('heading', { name: 'Знакомство' })
    expect(screen.queryByRole('link', { name: 'Начать знакомство' })).not.toBeInTheDocument()
  })

  it('нет базы знаний или ошибка — карточки нет', async () => {
    stub(() => jsonResponse(200, { ...STATUS, kbReady: false }))
    const { unmount } = render()
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('heading', { name: 'Знакомство' })).not.toBeInTheDocument()
    unmount()
    stub(() => errorResponse(500, 'internal', 'x'))
    render()
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('heading', { name: 'Знакомство' })).not.toBeInTheDocument()
  })
})
