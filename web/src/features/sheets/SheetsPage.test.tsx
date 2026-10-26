import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { PinnedSheets } from './PinnedSheets'
import { SUMMARIES } from './fixtures'
import { SheetsPage } from './SheetsPage'
import { filterSheets, pinned } from './sheets'

afterEach(() => vi.unstubAllGlobals())

const stub = (r: () => Response) => vi.stubGlobal('fetch', vi.fn(async () => r()))
const render = () => renderAt('/sheets', [{ path: '/sheets', element: <SheetsPage /> }])

describe('SheetsPage', () => {
  it('закреплённые сверху (недавно открытые первыми), остальные ниже', async () => {
    stub(() => jsonResponse(200, { sheets: SUMMARIES }))
    render()
    expect(await screen.findByRole('heading', { name: 'Закреплённое' })).toBeInTheDocument()
    const links = screen.getAllByRole('link').map((l) => l.textContent)
    expect(links[0]).toContain('Артикли')
    expect(links[1]).toContain('Глагол to be')
    expect(screen.getByRole('heading', { name: 'Остальные' })).toBeInTheDocument()
    expect(screen.getByText('слабых: 2')).toBeInTheDocument()
    expect(screen.getByText('✓ проверено')).toBeInTheDocument()
  })

  it('поиск и теги', async () => {
    stub(() => jsonResponse(200, { sheets: SUMMARIES }))
    const user = userEvent.setup()
    render()
    await screen.findByRole('searchbox', { name: 'Поиск шпаргалки' })
    await user.type(screen.getByRole('searchbox', { name: 'Поиск шпаргалки' }), 'артикл')
    expect(screen.getAllByRole('link')).toHaveLength(1)
    await user.clear(screen.getByRole('searchbox', { name: 'Поиск шпаргалки' }))
    await user.click(screen.getByRole('button', { name: 'таблицы' }))
    expect(screen.getAllByRole('link')).toHaveLength(1)
    await user.type(screen.getByRole('searchbox', { name: 'Поиск шпаргалки' }), 'zzz')
    expect(screen.getByText(/Ничего не нашлось/)).toBeInTheDocument()
  })

  it('ошибка загрузки — сообщение и «Повторить»', async () => {
    stub(() => errorResponse(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    render()
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
  })
})

describe('PinnedSheets', () => {
  it('до трёх закреплённых; нет закреплённых — блока нет', async () => {
    stub(() => jsonResponse(200, { sheets: SUMMARIES }))
    const { unmount } = renderAt('/', [{ path: '/', element: <PinnedSheets /> }])
    expect(await screen.findByRole('heading', { name: 'Закреплённое' })).toBeInTheDocument()
    expect(screen.getAllByRole('link')).toHaveLength(2)
    unmount()
    stub(() => jsonResponse(200, { sheets: SUMMARIES.map((s) => ({ ...s, favorite: false })) }))
    renderAt('/', [{ path: '/', element: <PinnedSheets /> }])
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('heading', { name: 'Закреплённое' })).not.toBeInTheDocument()
  })
})

describe('filterSheets / pinned', () => {
  it('поиск без учёта регистра по названию, описанию и тегам', () => {
    expect(filterSheets(SUMMARIES, 'ГЛАГОЛ', null).map((s) => s.id)).toEqual(['to-be', 'irregular-verbs'])
    expect(filterSheets(SUMMARIES, '', 'артикли').map((s) => s.id)).toEqual(['articles'])
    expect(filterSheets(SUMMARIES, '  ', null)).toHaveLength(3)
  })
  it('закреплённые: недавно открытые первыми', () => {
    expect(pinned(SUMMARIES).map((s) => s.id)).toEqual(['articles', 'to-be'])
  })
})
