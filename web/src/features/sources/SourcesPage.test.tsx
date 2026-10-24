import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { SourcesPage } from './SourcesPage'

afterEach(() => vi.unstubAllGlobals())

const CEFRJ = {
  name: 'cefrj',
  version: '1.5',
  license: 'Бесплатно для исследований и коммерции с указанием источника',
  url: 'https://raw.githubusercontent.com/openlanguageprofiles/olp-en-cefrj/abc/cefrj.csv',
  attribution: 'The CEFR-J Wordlist Version 1.5. Compiled by Yukio Tono, Tokyo University of Foreign Studies.',
  rowCount: 7799,
  importedAt: '2026-10-23T12:00:00Z',
}

function renderPage() {
  return renderAt('/sources', [{ path: '/sources', element: <SourcesPage /> }])
}

describe('SourcesPage', () => {
  it('показывает источник: название, лицензию, атрибуцию дословно, ссылку, число строк и дату', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { sources: [CEFRJ] })))
    renderPage()

    expect(await screen.findByRole('heading', { level: 1, name: 'Источники и лицензии' })).toBeInTheDocument()
    expect(document.title).toBe('Источники и лицензии — Verst')
    const card = within(await screen.findByRole('article', { name: /cefrj/ }))
    expect(card.getByText('версия 1.5')).toBeInTheDocument()
    expect(card.getByText(CEFRJ.license)).toBeInTheDocument()
    expect(card.getByText(CEFRJ.attribution)).toBeInTheDocument()
    expect(card.getByText(/7\s799/)).toBeInTheDocument()
    expect(card.getByText(/23 октября 2026/)).toBeInTheDocument()
    const link = card.getByRole('link', { name: /raw\.githubusercontent\.com/ })
    expect(link).toHaveAttribute('href', CEFRJ.url)
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'))
    expect(link).toHaveAttribute('target', '_blank')
  })

  it('несколько источников идут отдельными блоками в порядке ответа', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(200, { sources: [CEFRJ, { ...CEFRJ, name: 'tatoeba', version: '2026-10' }] })),
    )
    renderPage()
    const articles = await screen.findAllByRole('article')
    expect(articles).toHaveLength(2)
    expect(articles[0]).toHaveAccessibleName(/cefrj/)
    expect(articles[1]).toHaveAccessibleName(/tatoeba/)
  })

  it('если источников нет, подсказывает, как их загрузить', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { sources: [] })))
    renderPage()
    expect(await screen.findByText(/go run \.\/cmd\/import cefrj/)).toBeInTheDocument()
    expect(screen.queryByRole('article')).not.toBeInTheDocument()
  })

  it('пока грузится, говорит об этом', () => {
    vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(() => {})))
    renderPage()
    expect(screen.getByRole('status')).toHaveTextContent(/загружаем/i)
  })

  it('при сбое показывает ошибку и даёт повторить', async () => {
    let ok = false
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => (ok ? jsonResponse(200, { sources: [CEFRJ] }) : errorResponse(500, 'internal', 'Внутренняя ошибка'))),
    )
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    ok = true
    await userEvent.setup().click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByRole('article', { name: /cefrj/ })).toBeInTheDocument()
  })

  it('текст атрибуции и ссылка не превращаются в разметку (никакого HTML из данных)', async () => {
    const evil = { ...CEFRJ, attribution: '<img src=x onerror=alert(1)>', license: '<b>x</b>', url: 'javascript:alert(1)' }
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { sources: [evil] })))
    const { container } = renderPage()
    expect(await screen.findByText('<img src=x onerror=alert(1)>')).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    // Ссылкой становится только https-адрес: javascript: и прочее показываем текстом.
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
