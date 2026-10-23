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

const EXAMPLES = {
  examples: [
    {
      en: { text: 'I like to run every morning.', author: 'CK', tatoebaId: 11 },
      ru: { text: 'Я люблю бегать по утрам.', author: 'marafon', tatoebaId: 22 },
      highlight: { start: 10, end: 13 },
    },
    {
      en: { text: 'Nothing matches here at all.', author: 'CM', tatoebaId: 12 },
      ru: { text: 'Ничего не совпадает.', author: 'ksenija', tatoebaId: 23 },
    },
  ],
}

// Отвечает на поиск слов и на примеры по адресу.
function stubWithExamples(examples: () => Response | Promise<Response>) {
  const fn = vi.fn(async (url: string) =>
    url.includes('/words/examples') ? examples() : jsonResponse(200, { words: [RUN] }),
  )
  vi.stubGlobal('fetch', fn)
  return fn
}

const exampleCalls = (fn: ReturnType<typeof vi.fn>) =>
  fn.mock.calls.map((c) => c[0] as string).filter((u) => u.includes('/words/examples'))

describe('примеры предложений', () => {
  it('кнопка «Примеры» подгружает примеры слова, подсвечивает форму и показывает атрибуцию Tatoeba', async () => {
    const fetchMock = stubWithExamples(() => jsonResponse(200, EXAMPLES))
    const user = userEvent.setup()
    renderPage()
    await search('run')
    const button = await screen.findByRole('button', { name: /примеры/i })
    expect(button).toHaveAttribute('aria-expanded', 'false')

    await user.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(exampleCalls(fetchMock)).toEqual(['/api/v1/words/examples?lemma=run&pos=verb'])

    const panel = within(document.getElementById(button.getAttribute('aria-controls')!)!)
    expect(await panel.findByText('Я люблю бегать по утрам.')).toBeInTheDocument()
    expect(panel.getByText('run').tagName).toBe('MARK')
    expect(panel.getByText('Ничего не совпадает.')).toBeInTheDocument()
    // Без подсветки предложение всё равно показывается целиком.
    expect(panel.getByText('Nothing matches here at all.')).toBeInTheDocument()

    // Атрибуция по лицензии CC BY: номер и автор обеих сторон, ссылка на tatoeba.org.
    const link = panel.getByRole('link', { name: '#11' })
    expect(link).toHaveAttribute('href', 'https://tatoeba.org/en/sentences/show/11')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'))
    expect(panel.getByRole('link', { name: '#22' })).toHaveAttribute('href', 'https://tatoeba.org/en/sentences/show/22')
    expect(panel.getByText(/CK/)).toBeInTheDocument()
    expect(panel.getByText(/marafon/)).toBeInTheDocument()
  })

  it('повторное нажатие сворачивает, а новое раскрытие берёт примеры из кэша', async () => {
    const fetchMock = stubWithExamples(() => jsonResponse(200, EXAMPLES))
    const user = userEvent.setup()
    renderPage()
    await search('run')
    const button = await screen.findByRole('button', { name: /примеры/i })
    await user.click(button)
    await screen.findByText('Я люблю бегать по утрам.')
    await user.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Я люблю бегать по утрам.')).not.toBeInTheDocument()
    await user.click(button)
    expect(await screen.findByText('Я люблю бегать по утрам.')).toBeInTheDocument()
    expect(exampleCalls(fetchMock)).toHaveLength(1)
  })

  it('не запрашивает примеры, пока их не раскрыли', async () => {
    const fetchMock = stubWithExamples(() => jsonResponse(200, EXAMPLES))
    renderPage()
    await search('run')
    await screen.findByRole('button', { name: /примеры/i })
    expect(exampleCalls(fetchMock)).toHaveLength(0)
  })

  it('если примеров нет, говорит об этом', async () => {
    stubWithExamples(() => jsonResponse(200, { examples: [] }))
    renderPage()
    await search('run')
    await userEvent.setup().click(await screen.findByRole('button', { name: /примеры/i }))
    expect(await screen.findByText(/примеров нет/i)).toBeInTheDocument()
  })

  it('пока грузятся, показывает ожидание', async () => {
    stubWithExamples(() => new Promise<Response>(() => {}))
    renderPage()
    await search('run')
    await userEvent.setup().click(await screen.findByRole('button', { name: /примеры/i }))
    expect(await screen.findByText(/загружаем примеры/i)).toBeInTheDocument()
  })

  it('при сбое показывает ошибку и даёт повторить', async () => {
    let ok = false
    stubWithExamples(() => (ok ? jsonResponse(200, EXAMPLES) : errorResponse(500, 'internal', 'Внутренняя ошибка')))
    renderPage()
    await search('run')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /примеры/i }))
    expect(await screen.findByText('Внутренняя ошибка')).toBeInTheDocument()
    ok = true
    await user.click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByText('Я люблю бегать по утрам.')).toBeInTheDocument()
  })

  it('слово с косой чертой передаётся в запросе как есть, но экранировано', async () => {
    const fetchMock = vi.fn(async (url: string) =>
      url.includes('/words/examples')
        ? jsonResponse(200, { examples: [] })
        : jsonResponse(200, { words: [{ lemma: 'cafe/café', pos: 'noun', level: 'A1', levels: [{ source: 'cefrj', level: 'A1', notes: [] }] }] }),
    )
    vi.stubGlobal('fetch', fetchMock)
    renderPage()
    await search('cafe')
    await userEvent.setup().click(await screen.findByRole('button', { name: /примеры/i }))
    await screen.findByText(/примеров нет/i)
    expect(exampleCalls(fetchMock)).toEqual(['/api/v1/words/examples?lemma=cafe%2Fcaf%C3%A9&pos=noun'])
  })
})

describe('кнопка «В карточки»', () => {
  const word = (inCards: boolean) => ({ ...RUN, inCards })

  /** Поиск слов отвечает по очереди (после добавления слово приходит уже с inCards), POST /cards — как задано. */
  function stubCards(opts: { add?: () => Response | Promise<Response>; searches?: boolean[] }) {
    const searches = [...(opts.searches ?? [false])]
    const fn = vi.fn(async (url: string, init?: RequestInit) => {
      if (url === '/api/v1/cards' && init?.method === 'POST') return opts.add ? opts.add() : jsonResponse(201, { cardId: 1, created: true })
      const inCards = searches.length > 1 ? searches.shift()! : searches[0]
      return jsonResponse(200, { words: [word(inCards)] })
    })
    vi.stubGlobal('fetch', fn)
    return fn
  }
  const posts = (fn: ReturnType<typeof vi.fn>) => fn.mock.calls.filter((c) => c[0] === '/api/v1/cards' && c[1]?.method === 'POST')
  const gets = (fn: ReturnType<typeof vi.fn>) => fn.mock.calls.filter((c) => String(c[0]).startsWith('/api/v1/words?q='))

  it('слово без карточки: кнопка отправляет слово и часть речи, потом показывает «В карточках»', async () => {
    const fn = stubCards({ searches: [false, true] })
    const user = userEvent.setup()
    renderPage()
    await search('run')
    await user.click(await screen.findByRole('button', { name: 'В карточки' }))
    const done = await screen.findByRole('button', { name: /в карточках/i })
    expect(done).toBeDisabled()
    expect(JSON.parse(posts(fn)[0][1].body as string)).toEqual({ lemma: 'run', pos: 'verb' })
  })

  it('после добавления поиск обновляется, и признак приходит с сервера', async () => {
    const fn = stubCards({ searches: [false, true] })
    renderPage()
    await search('run')
    await userEvent.setup().click(await screen.findByRole('button', { name: 'В карточки' }))
    await waitFor(() => expect(gets(fn).length).toBeGreaterThanOrEqual(2))
    expect(await screen.findByRole('button', { name: /в карточках/i })).toBeDisabled()
  })

  it('слово, которое уже в карточках, сразу неактивно', async () => {
    const fn = stubCards({ searches: [true] })
    renderPage()
    await search('run')
    expect(await screen.findByRole('button', { name: /в карточках/i })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'В карточки' })).not.toBeInTheDocument()
    expect(posts(fn)).toHaveLength(0)
  })

  it('409 no_examples показывает объяснение, кнопка остаётся доступной', async () => {
    stubCards({ add: () => errorResponse(409, 'no_examples', 'Для этого слова нет подходящих примеров, карточку не собрать') })
    renderPage()
    await search('run')
    await userEvent.setup().click(await screen.findByRole('button', { name: 'В карточки' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('нет подходящих примеров')
    expect(screen.getByRole('button', { name: 'В карточки' })).toBeEnabled()
  })

  it('другая ошибка: сообщение и повтор', async () => {
    let calls = 0
    const fn = stubCards({ add: () => (++calls === 1 ? errorResponse(500, 'internal', 'Внутренняя ошибка') : jsonResponse(201, { cardId: 1, created: true })), searches: [false, true] })
    const user = userEvent.setup()
    renderPage()
    await search('run')
    await user.click(await screen.findByRole('button', { name: 'В карточки' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    await user.click(screen.getByRole('button', { name: 'В карточки' }))
    expect(await screen.findByRole('button', { name: /в карточках/i })).toBeDisabled()
    expect(posts(fn)).toHaveLength(2)
  })

  it('двойной клик отправляет один запрос', async () => {
    const fn = stubCards({
      add: () => new Promise<Response>((resolve) => setTimeout(() => resolve(jsonResponse(201, { cardId: 1, created: true })), 50)),
      searches: [false, true],
    })
    renderPage()
    await search('run')
    await userEvent.setup().dblClick(await screen.findByRole('button', { name: 'В карточки' }))
    await screen.findByRole('button', { name: /в карточках/i })
    expect(posts(fn)).toHaveLength(1)
  })
})
