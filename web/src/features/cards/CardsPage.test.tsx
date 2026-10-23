import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { CardsPage } from './CardsPage'

afterEach(() => vi.unstubAllGlobals())

const card = (id: number, lemma = 'run', sentence: unknown = SENTENCE) => ({
  cardId: id,
  lemma,
  pos: 'verb',
  level: 'B2',
  state: 0,
  intervals: { again: 60, hard: 360, good: 600, easy: 345_600 },
  ...(sentence ? { sentence } : {}),
})

const SENTENCE = {
  en: { text: 'I like to run every morning.', author: 'CK', tatoebaId: 11 },
  ru: { text: 'Я люблю бегать по утрам.', author: 'marafon', tatoebaId: 22 },
  gap: { start: 10, end: 13 },
}
const SENTENCE2 = {
  en: { text: 'We go to school together.', author: 'CM', tatoebaId: 33 },
  ru: { text: 'Мы ходим в школу вместе.', author: 'ksenija', tatoebaId: 44 },
  gap: { start: 3, end: 5 },
}

type Queue = { dueCount: number; totalCount: number; card?: ReturnType<typeof card> }
const queue = (dueCount: number, c?: ReturnType<typeof card>): Queue => ({ dueCount, totalCount: dueCount + 3, ...(c ? { card: c } : {}) })

/** Подмена API: очередь по порядку ответов и журнал оценок. */
function stubApi(opts: { queue: () => Response; review?: (body: { cardId: number; rating: number; key: string }) => Response | Promise<Response> }) {
  const reviews: { cardId: number; rating: number; key: string }[] = []
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/cards/queue')) return opts.queue()
    if (url.endsWith('/cards/review')) {
      const body = JSON.parse(init?.body as string)
      reviews.push(body)
      return opts.review ? opts.review(body) : jsonResponse(200, queue(0))
    }
    throw new Error(`неожиданный запрос ${url}`)
  })
  vi.stubGlobal('fetch', fn)
  return { fn, reviews }
}

// Пропуск лежит отдельным элементом внутри абзаца, поэтому текст предложения сверяем по textContent абзаца.
const findSentence = (text: string) => screen.findByText((_, el) => el?.tagName === 'P' && el.textContent === text)

function renderPage() {
  return renderAt('/cards', [
    {
      path: '/cards',
      element: (
        <>
          {/* Элементы оболочки (меню, кнопки): клавиши Пробел и Enter на них не должны открывать ответ. */}
          <a href="#menu">Меню</a>
          <button type="button">Тема</button>
          <CardsPage />
        </>
      ),
    },
    { path: '/words', element: <p>Раздел слов</p> },
  ])
}

describe('CardsPage: клавиатура и фокус', () => {
  it.each([['Меню', 'ссылке'], ['Тема', 'кнопке']])('Пробел и Enter на %s (%s оболочки) не открывают ответ', async (name) => {
    stubApi({ queue: () => jsonResponse(200, queue(1, card(7))) })
    const user = userEvent.setup()
    renderPage()
    await findSentence('I like to _____ every morning.')
    const target = screen.getByRole(name === 'Меню' ? 'link' : 'button', { name })
    target.focus()
    await user.keyboard(' ')
    await user.keyboard('{Enter}')
    expect(screen.getByRole('button', { name: /показать ответ/i })).toBeInTheDocument()
    expect(screen.queryByText('run')).not.toBeInTheDocument()
  })

  it('на самой кнопке «Показать ответ» Пробел и Enter по-прежнему открывают ответ', async () => {
    stubApi({ queue: () => jsonResponse(200, queue(1, card(7))) })
    const user = userEvent.setup()
    renderPage()
    ;(await screen.findByRole('button', { name: /показать ответ/i })).focus()
    await user.keyboard('{Enter}')
    expect(await screen.findByText('run')).toHaveProperty('tagName', 'MARK')
  })
})

describe('CardsPage', () => {
  it('пока грузится, говорит об этом; заголовок страницы «Карточки — Verst»', () => {
    stubApi({ queue: () => new Promise<Response>(() => {}) as unknown as Response })
    renderPage()
    expect(screen.getByRole('status')).toHaveTextContent(/загружаем/i)
    expect(screen.getByRole('heading', { level: 1, name: 'Карточки' })).toBeInTheDocument()
    expect(document.title).toBe('Карточки — Verst')
  })

  it('если повторять нечего, пишет «На сегодня всё» и зовёт добавить слова', async () => {
    stubApi({ queue: () => jsonResponse(200, queue(0)) })
    renderPage()
    expect(await screen.findByText('На сегодня всё')).toBeInTheDocument()
    expect(screen.getByText(/всего карточек: 3/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /слова/i })).toHaveAttribute('href', '/words')
  })

  it('лицо карточки: предложение с пропуском и перевод, ответа и оценок нет', async () => {
    stubApi({ queue: () => jsonResponse(200, queue(2, card(7))) })
    renderPage()
    expect(await findSentence('I like to _____ every morning.')).toBeInTheDocument()
    expect(screen.getByText('Я люблю бегать по утрам.')).toBeInTheDocument()
    expect(screen.getByText('Осталось: 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /показать ответ/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Забыл/ })).not.toBeInTheDocument()
    expect(screen.queryByText(/run every/)).not.toBeInTheDocument() // слово не выдано до ответа
  })

  it('оборот: слово подсвечено, атрибуция Tatoeba, четыре оценки с интервалами', async () => {
    stubApi({ queue: () => jsonResponse(200, queue(1, card(7))) })
    renderPage()
    await userEvent.setup().click(await screen.findByRole('button', { name: /показать ответ/i }))
    const mark = await screen.findByText('run')
    expect(mark.tagName).toBe('MARK')
    expect(screen.getByRole('link', { name: '#11' })).toHaveAttribute('href', 'https://tatoeba.org/en/sentences/show/11')
    expect(screen.getByRole('link', { name: '#22' })).toHaveAttribute('href', 'https://tatoeba.org/en/sentences/show/22')
    expect(screen.getByText(/CK/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Забыл.*1 мин/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Трудно.*6 мин/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Хорошо.*10 мин/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Легко.*4 дн/ })).toBeInTheDocument()
  })

  it.each([[' '], ['{Enter}']])('ответ открывается с клавиатуры (%j)', async (key) => {
    stubApi({ queue: () => jsonResponse(200, queue(1, card(7))) })
    renderPage()
    await findSentence('I like to _____ every morning.')
    await userEvent.setup().keyboard(key)
    expect(await screen.findByText('run')).toHaveProperty('tagName', 'MARK')
  })

  it('клавиши 1–4 до ответа ничего не отправляют, после ответа оценивают', async () => {
    const { reviews } = stubApi({ queue: () => jsonResponse(200, queue(1, card(7))), review: () => jsonResponse(200, queue(0)) })
    const user = userEvent.setup()
    renderPage()
    await findSentence('I like to _____ every morning.')
    await user.keyboard('3')
    expect(reviews).toHaveLength(0)
    await user.keyboard(' ')
    await screen.findByText('run')
    await user.keyboard('3')
    await waitFor(() => expect(reviews).toHaveLength(1))
    expect(reviews[0]).toMatchObject({ cardId: 7, rating: 3 })
    expect(typeof reviews[0].key).toBe('string')
    expect(reviews[0].key.length).toBeGreaterThan(0)
  })

  it('после оценки показывает следующую карточку закрытой и обновляет счётчик; ключ у неё новый', async () => {
    const { reviews } = stubApi({
      queue: () => jsonResponse(200, queue(2, card(7))),
      review: () => jsonResponse(200, queue(1, card(8, 'go', SENTENCE2))),
    })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: /показать ответ/i }))
    await user.click(await screen.findByRole('button', { name: /^Хорошо/ }))
    expect(await findSentence('We _____ to school together.')).toBeInTheDocument()
    expect(screen.getByText('Осталось: 1')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Хорошо/ })).not.toBeInTheDocument() // новая карточка снова закрыта

    await user.keyboard(' ')
    await user.keyboard('4')
    await waitFor(() => expect(reviews).toHaveLength(2))
    expect(reviews[1]).toMatchObject({ cardId: 8, rating: 4 })
    expect(reviews[1].key).not.toBe(reviews[0].key)
  })

  it('когда оценена последняя карточка, показывает «На сегодня всё»', async () => {
    stubApi({ queue: () => jsonResponse(200, queue(1, card(7))), review: () => jsonResponse(200, queue(0)) })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: /показать ответ/i }))
    await user.click(await screen.findByRole('button', { name: /^Легко/ }))
    expect(await screen.findByText('На сегодня всё')).toBeInTheDocument()
  })

  it('двойное нажатие (клавиша и клик) отправляет одну оценку', async () => {
    const { reviews } = stubApi({
      queue: () => jsonResponse(200, queue(1, card(7))),
      review: () => new Promise<Response>((resolve) => setTimeout(() => resolve(jsonResponse(200, queue(0))), 50)),
    })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: /показать ответ/i }))
    const good = await screen.findByRole('button', { name: /^Хорошо/ })
    await user.keyboard('33')
    await user.dblClick(good).catch(() => {}) // кнопка может исчезнуть после ответа
    await waitFor(() => expect(screen.getByText('На сегодня всё')).toBeInTheDocument())
    expect(reviews).toHaveLength(1)
  })

  it('ошибка оценки: сообщение, карточка остаётся, повтор уходит с тем же ключом', async () => {
    let calls = 0
    const { reviews } = stubApi({
      queue: () => jsonResponse(200, queue(1, card(7))),
      review: () => (++calls === 1 ? errorResponse(500, 'internal', 'Внутренняя ошибка') : jsonResponse(200, queue(0))),
    })
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: /показать ответ/i }))
    await user.click(await screen.findByRole('button', { name: /^Хорошо/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('button', { name: /^Хорошо/ })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: /^Хорошо/ }))
    expect(await screen.findByText('На сегодня всё')).toBeInTheDocument()
    expect(reviews).toHaveLength(2)
    expect(reviews[1].key).toBe(reviews[0].key)
  })

  it('ошибка загрузки очереди: сообщение и «Повторить»', async () => {
    let ok = false
    stubApi({ queue: () => (ok ? jsonResponse(200, queue(1, card(7))) : errorResponse(500, 'internal', 'Внутренняя ошибка')) })
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    ok = true
    await userEvent.setup().click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await findSentence('I like to _____ every morning.')).toBeInTheDocument()
  })

  it('карточка без предложения: слово, часть речи и уровень, оценка сразу доступна', async () => {
    const { reviews } = stubApi({ queue: () => jsonResponse(200, queue(1, card(7, 'run', null))), review: () => jsonResponse(200, queue(0)) })
    renderPage()
    expect(await screen.findByText('run')).toBeInTheDocument()
    expect(screen.getByText(/предложений для этого слова не нашлось/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /показать ответ/i })).not.toBeInTheDocument()
    await userEvent.setup().click(screen.getByRole('button', { name: /^Хорошо/ }))
    await waitFor(() => expect(reviews).toHaveLength(1))
  })
})
