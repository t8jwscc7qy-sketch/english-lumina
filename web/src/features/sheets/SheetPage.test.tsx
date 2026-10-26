import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { SHEET } from './fixtures'
import { SheetPage } from './SheetPage'

afterEach(() => vi.unstubAllGlobals())

function stub(sheet: () => Response) {
  const calls: { url: string; method: string; body: unknown }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (url.endsWith('/sheets/open')) return sheet()
      if (url.endsWith('/sheets')) return jsonResponse(200, { sheets: [] })
      return jsonResponse(200, {})
    }),
  )
  return calls
}

const render = () => renderAt('/sheets/irregular-verbs', [{ path: '/sheets/:id', element: <SheetPage /> }])

describe('SheetPage', () => {
  it('показывает блоки, плашку черновика и источники', async () => {
    const calls = stub(() => jsonResponse(200, SHEET))
    const { container } = render()
    expect(await screen.findByRole('heading', { name: 'Неправильные глаголы' })).toBeInTheDocument()
    expect(screen.getByText(/Черновик: проверь/)).toBeInTheDocument()
    expect(screen.getByText(/Источники: собственные знания автора/)).toBeInTheDocument()
    expect(screen.getByText('Нужно запомнить.')).toBeInTheDocument()
    expect(screen.getByText('I went')).toBeInTheDocument()
    expect(container.querySelector('strong')).toHaveTextContent('wrote')
    expect(calls.find((c) => c.url.endsWith('/sheets/open'))?.body).toEqual({ id: 'irregular-verbs' })
  })

  it('«Закрепить» шлёт PUT и меняет кнопку', async () => {
    const calls = stub(() => jsonResponse(200, SHEET))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: '☆ Закрепить' }))
    expect(await screen.findByRole('button', { name: '★ Закреплено' })).toHaveAttribute('aria-pressed', 'true')
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ id: 'irregular-verbs', favorite: true })
  })

  it('«Я проверил» отмечает шпаргалку проверенной', async () => {
    const calls = stub(() => jsonResponse(200, SHEET))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: 'Я проверил' }))
    expect(await screen.findByText('✓ Проверено')).toBeInTheDocument()
    expect(screen.queryByText(/Черновик: проверь/)).not.toBeInTheDocument()
    expect(calls.some((c) => c.url.endsWith('/sheets/review'))).toBe(true)
  })

  it('«Печать» вызывает window.print', async () => {
    stub(() => jsonResponse(200, SHEET))
    const print = vi.spyOn(window, 'print').mockImplementation(() => {})
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: 'Печать' }))
    expect(print).toHaveBeenCalledOnce()
  })

  it('нет такой шпаргалки — спокойное сообщение', async () => {
    stub(() => errorResponse(404, 'not_found', 'Такой шпаргалки нет'))
    render()
    expect(await screen.findByRole('heading', { name: 'Такой шпаргалки нет' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('тексты из данных выводятся только текстом', async () => {
    stub(() => jsonResponse(200, { ...SHEET, title: '<img src=x onerror=alert(1)>', blocks: [{ type: 'text', text: '<b>x</b>' }] }))
    const { container } = render()
    await screen.findByRole('heading', { level: 1 })
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
  })
})
