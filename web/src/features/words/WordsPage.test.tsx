import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { WordsPage } from './WordsPage'

afterEach(() => vi.unstubAllGlobals())

const RUN = {
  lemma: 'run',
  pos: 'verb',
  level: 'A1',
  levels: [{ source: 'cefrj', level: 'A1', notes: [] }],
}
const COMPLEXITY = {
  lemma: 'complexity',
  pos: 'noun',
  level: 'B2',
  levels: [
    { source: 'cefrj', level: 'B2', notes: [] },
    { source: 'octanove', level: 'C1', notes: ['сложность', 'запутанность'] },
  ],
}

function stub(respond: (q: string) => Response | Promise<Response>) {
  const fn = vi.fn(async (url: string) => respond(new URL(url, 'http://x').searchParams.get('q') ?? ''))
  vi.stubGlobal('fetch', fn)
  return fn
}

const list = (...words: unknown[]) => jsonResponse(200, { words })

function renderPage() {
  return renderAt('/words', [{ path: '/words', element: <WordsPage /> }])
}

async function search(text: string) {
  await userEvent.setup().type(screen.getByRole('searchbox', { name: 'Слово' }), text)
}

describe('WordsPage', () => {
  it('до ввода показывает подсказку и ничего не запрашивает', () => {
    const fetchMock = stub(() => list())
    renderPage()
    expect(screen.getByRole('heading', { level: 1, name: 'Слова' })).toBeInTheDocument()
    expect(document.title).toBe('Слова — Verst')
    expect(screen.getByRole('searchbox', { name: 'Слово' })).toHaveFocus()
    expect(screen.getByText(/введи начало слова/i)).toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('честно говорит, что примеров предложений пока нет', () => {
    stub(() => list())
    renderPage()
    expect(screen.getByText(/примеры предложений появятся/i)).toBeInTheDocument()
  })

  it('показывает слово, часть речи, уровень и источник', async () => {
    stub(() => list(RUN))
    renderPage()
    await search('run')
    const item = within(await screen.findByRole('listitem'))
    expect(item.getByText('run')).toBeInTheDocument()
    expect(item.getByText('verb')).toBeInTheDocument()
    expect(item.getByLabelText('Уровень A1')).toBeInTheDocument()
    expect(item.getByText('CEFR-J A1')).toBeInTheDocument()
  })

  it('у слова с двумя оценками показывает уровень слова и обе оценки с заметками', async () => {
    stub(() => list(COMPLEXITY))
    renderPage()
    await search('compl')
    const item = within(await screen.findByRole('listitem'))
    expect(item.getByLabelText('Уровень B2')).toBeInTheDocument()
    expect(item.getByText('CEFR-J B2 · Octanove C1')).toBeInTheDocument()
    expect(item.getByText('сложность; запутанность')).toBeInTheDocument()
  })

  it('быстрый набор отправляет один запрос с итоговым текстом', async () => {
    const fetchMock = stub(() => list(RUN))
    renderPage()
    await search('run')
    await screen.findByRole('listitem')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/words?q=run')
  })

  it('считает найденное в строке состояния', async () => {
    stub(() => list(RUN, COMPLEXITY))
    renderPage()
    await search('r')
    expect(await screen.findByText('Найдено: 2')).toBe(await screen.findByRole('status'))
  })

  it('если ничего нет, говорит об этом и что поиск идёт по началу слова', async () => {
    stub(() => list())
    renderPage()
    await search('zzz')
    expect(await screen.findByText(/ничего не найдено/i)).toHaveTextContent('zzz')
    expect(screen.getByText(/по началу слова/i)).toBeInTheDocument()
  })

  it('очистка поля возвращает подсказку', async () => {
    stub(() => list(RUN))
    renderPage()
    await search('run')
    await screen.findByRole('listitem')
    await userEvent.setup().clear(screen.getByRole('searchbox', { name: 'Слово' }))
    await waitFor(() => expect(screen.queryByRole('listitem')).not.toBeInTheDocument())
    expect(screen.getByText(/введи начало слова/i)).toBeInTheDocument()
  })

  it('при сбое показывает ошибку и даёт повторить', async () => {
    let ok = false
    stub(() => (ok ? list(RUN) : errorResponse(500, 'internal', 'Внутренняя ошибка')))
    renderPage()
    await search('run')
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    ok = true
    await userEvent.setup().click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByRole('listitem')).toBeInTheDocument()
  })

  it('слово из данных выводится текстом, а не разметкой', async () => {
    stub(() => list({ ...RUN, lemma: '<img src=x onerror=alert(1)>' }))
    const { container } = renderPage()
    await search('x')
    expect(await screen.findByText('<img src=x onerror=alert(1)>')).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
  })
})
