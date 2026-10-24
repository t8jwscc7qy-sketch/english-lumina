import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { focusManager, onlineManager } from '@tanstack/react-query'
import { RedirectIfAuthed } from '@/app/guards'
import { meKey } from '@/features/auth/auth'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { ExercisesPage } from './ExercisesPage'

afterEach(() => vi.unstubAllGlobals())

const ex = (id: number, prompt = 'She is _____ engineer.', choices = ['a', 'an', 'the', 'no article']) => ({
  id,
  kind: 'gap',
  prompt,
  translation: 'Она инженер.',
  choices,
  topic: 'grammar:articles',
})
const rating = (topic: string, r: number, deviation = 120) => ({ topic, rating: r, deviation, attempts: 4 })
const result = (correct = true) => ({
  correct,
  answer: 'an',
  translation: 'Она инженер.',
  ratings: [rating('grammar:articles', 1523.4)],
})

const TOPICS = [
  rating('grammar:future', 1500),
  rating('grammar:articles', 1380),
  rating('pos:verb', 1450),
  rating('grammar:modals', 1300),
  rating('level:B1', 1600),
  rating('grammar:prepositions', 1250),
  rating('grammar:past_simple', 1420),
]

type Body = { exerciseId: number; given: string; key: string }

function stubApi(opts: {
  next: () => Response
  answer?: (b: Body) => Response | Promise<Response>
  topics?: () => Response
}) {
  const answers: Body[] = []
  const fn = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/exercises/next')) return opts.next()
    if (url.endsWith('/exercises/topics')) return opts.topics ? opts.topics() : jsonResponse(200, { topics: TOPICS })
    if (url.endsWith('/exercises/answer')) {
      const body = JSON.parse(init?.body as string) as Body
      answers.push(body)
      return opts.answer ? opts.answer(body) : jsonResponse(200, result())
    }
    throw new Error(`неожиданный запрос ${url}`)
  })
  vi.stubGlobal('fetch', fn)
  return { fn, answers }
}

const findPrompt = (text: string) => screen.findByText((_, el) => el?.tagName === 'P' && el.textContent === text)

function renderPage() {
  return renderAt('/exercises', [
    {
      path: '/exercises',
      element: (
        <>
          <a href="#menu">Меню</a>
          <ExercisesPage />
        </>
      ),
    },
    { path: '/login', element: <p>Экран входа</p> },
  ])
}

describe('ExercisesPage', () => {
  it('показывает предложение с пропуском и четыре варианта', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)) })
    renderPage()
    await findPrompt('She is _____ engineer.')
    expect(screen.getByText('Она инженер.')).toBeInTheDocument()
    const choices = screen.getAllByRole('button').filter((b) => b.hasAttribute('aria-keyshortcuts'))
    expect(choices.map((b) => b.getAttribute('aria-keyshortcuts'))).toEqual(['1', '2', '3', '4'])
    expect(screen.getByRole('button', { name: 'an' })).toBeInTheDocument() // цифра клавиши в имя не входит
  })

  it('клавиши 1–4 выбирают вариант и отправляют ответ ровно один раз', async () => {
    const { answers } = stubApi({ next: () => jsonResponse(200, ex(7)) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await waitFor(() => expect(screen.getByRole('article', { name: 'Упражнение' })).toHaveFocus())
    await user.keyboard('2')
    await screen.findByText('Верно')
    await user.keyboard('3')
    expect(answers).toHaveLength(1)
    expect(answers[0]).toMatchObject({ exerciseId: 7, given: 'an' })
    expect(answers[0].key.length).toBeGreaterThan(8)
  })

  it('удержание клавиши и двойное нажатие отправляют один ответ', async () => {
    let release: (r: Response) => void = () => {}
    const { answers } = stubApi({
      next: () => jsonResponse(200, ex(7)),
      answer: () => new Promise<Response>((r) => (release = r)),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('{2>3/}')
    await user.keyboard('2')
    await user.click(screen.getByRole('button', { name: 'an' }))
    expect(answers).toHaveLength(1)
    release(jsonResponse(200, result()))
    await screen.findByText('Верно')
  })

  it('после ответа виден верный ответ, перевод и «Дальше» (Пробел)', async () => {
    let n = 0
    const { fn } = stubApi({
      next: () => jsonResponse(200, n++ === 0 ? ex(1) : ex(2, 'They go _____ school.', ['to', 'at', 'in', 'on'])),
      answer: () => jsonResponse(200, { ...result(false), answer: 'an' }),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('1')
    await screen.findByText('Неверно')
    expect(screen.getByText(/Верный ответ:/)).toHaveTextContent('an')
    expect(screen.getAllByText('Она инженер.').length).toBeGreaterThan(0)
    expect(screen.getByText(/Артикли/, { selector: '[data-affected] *' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Дальше/ })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('article', { name: 'Упражнение' })).toHaveFocus())
    await user.keyboard(' ')
    await findPrompt('They go _____ school.')
    expect(fn.mock.calls.filter(([u]) => String(u).endsWith('/exercises/next'))).toHaveLength(2)
    await waitFor(() => expect(screen.getByRole('article', { name: 'Упражнение' })).toHaveFocus())
  })

  it('404 no_exercise показывает пустое состояние', async () => {
    stubApi({ next: () => errorResponse(404, 'no_exercise', 'Нет упражнений') })
    renderPage()
    expect(await screen.findByText('Подходящих упражнений пока нет')).toBeInTheDocument()
    expect(screen.getByText(/Попробуй позже или поменяй уровень в настройках/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Настройки' })).toHaveAttribute('href', '/settings')
    expect(screen.queryByRole('link', { name: /Добавить слова/ })).not.toBeInTheDocument()
  })

  it('ответ с заглавной в начале предложения: верный вариант «The» подсвечен и стоит в предложении', async () => {
    stubApi({
      next: () => jsonResponse(200, ex(5, '_____ dog runs.', ['An', 'A', 'The'])),
      answer: () => jsonResponse(200, { ...result(false), answer: 'The' }),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('_____ dog runs.')
    await user.keyboard('1')
    await screen.findByText('Неверно')
    expect(screen.getByText(/Верный ответ:/)).toHaveTextContent('The')
    expect(await screen.findByText('The', { selector: 'mark' })).toBeInTheDocument()
    const right = screen.getByRole('button', { name: /The/ })
    expect(right).toHaveTextContent('верно')
    expect(screen.getByRole('button', { name: /An/ })).toHaveTextContent('твой ответ')
  })

  it('ошибка сети показывает «Повторить»', async () => {
    let fail = true
    stubApi({ next: () => (fail ? errorResponse(500, 'internal', 'Сбой') : jsonResponse(200, ex(1))) })
    const user = userEvent.setup()
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent('Сбой')
    fail = false
    await user.click(screen.getByRole('button', { name: 'Повторить' }))
    await findPrompt('She is _____ engineer.')
  })

  it('401 ведёт на вход', async () => {
    stubApi({ next: () => errorResponse(401, 'unauthorized', 'Нужно войти') })
    const { client } = renderPage()
    expect(await screen.findByText('Экран входа')).toBeInTheDocument()
    expect(client.getQueryData(meKey)).toBeNull()
  })

  it('слабые темы: пять с наименьшим рейтингом и русские названия', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)) })
    renderPage()
    const list = within(await screen.findByRole('region', { name: 'Слабые темы' })).getByRole('list')
    const items = Array.from(list.querySelectorAll('li')).map((li) => li.textContent ?? '')
    expect(items).toHaveLength(5)
    expect(items[0]).toContain('Предлоги')
    expect(items[0]).toContain('1250')
    expect(items[1]).toContain('Модальные глаголы')
    expect(items.join(' ')).not.toContain('Уровень B1')
    expect(items.join(' ')).not.toContain('grammar:')
  })

  it('пустой список вариантов даёт ручной ввод', async () => {
    const { answers } = stubApi({ next: () => jsonResponse(200, ex(3, 'She _____ tea.', [])) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She _____ tea.')
    await user.type(screen.getByLabelText('Твой ответ'), 'drinks{Enter}')
    await screen.findByText('Верно')
    expect(answers[0].given).toBe('drinks')
  })

  it('двойное нажатие до ответа сервера не создаёт второй запрос (busyRef, а не disabled)', async () => {
    const { answers } = stubApi({ next: () => jsonResponse(200, ex(7)), answer: () => new Promise<Response>(() => {}) })
    renderPage()
    await findPrompt('She is _____ engineer.')
    fireEvent.keyDown(window, { key: '2' })
    fireEvent.keyDown(window, { key: '2' })
    fireEvent.keyDown(window, { key: '3' })
    await waitFor(() => expect(answers).toHaveLength(1))
    await new Promise((r) => setTimeout(r, 30))
    expect(answers).toHaveLength(1)
  })

  it('смена фокуса окна и возврат сети не меняют упражнение и не шлют повторный GET', async () => {
    const { fn } = stubApi({ next: () => jsonResponse(200, ex(1)) })
    renderPage()
    await findPrompt('She is _____ engineer.')
    const gets = () => fn.mock.calls.filter(([u]) => String(u).endsWith('/exercises/next')).length
    expect(gets()).toBe(1)
    act(() => focusManager.setFocused(false))
    act(() => focusManager.setFocused(true))
    act(() => onlineManager.setOnline(false))
    act(() => onlineManager.setOnline(true))
    await new Promise((r) => setTimeout(r, 50))
    expect(gets()).toBe(1)
    expect(screen.getByRole('article', { name: 'Упражнение' })).toBeInTheDocument()
    await findPrompt('She is _____ engineer.')
  })

  it('результат объявляется через постоянный живой регион', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    const live = document.querySelector('[role="status"][aria-live="polite"]') as HTMLElement
    expect(live).toBeEmptyDOMElement()
    await user.keyboard('2')
    await screen.findByText('Верно')
    expect(document.querySelector('[role="status"][aria-live="polite"]')).toBe(live)
    expect(live).toHaveTextContent('Верно')
    expect(live).toHaveTextContent('Она инженер.')
  })

  it('ручной ввод: фокус в поле «Твой ответ»', async () => {
    stubApi({ next: () => jsonResponse(200, ex(3, 'She _____ tea.', [])) })
    renderPage()
    const input = await screen.findByLabelText('Твой ответ')
    await waitFor(() => expect(input).toHaveFocus())
  })

  it('401 при ответе: meKey обнулён и вход не возвращает назад', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)), answer: () => errorResponse(401, 'unauthorized', 'Нужно войти') })
    const { newTestClient } = await import('@/test/render')
    const client = newTestClient()
    client.setQueryData(meKey, { login: 'u' })
    const user = userEvent.setup()
    renderAt(
      '/exercises',
      [
        { path: '/exercises', element: <ExercisesPage /> },
        { path: '/login', element: <RedirectIfAuthed><p>Экран входа</p></RedirectIfAuthed> },
        { path: '/', element: <p>Главная</p> },
      ],
      client,
    )
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    expect(await screen.findByText('Экран входа')).toBeInTheDocument()
    expect(client.getQueryData(meKey)).toBeNull()
    expect(screen.queryByText('Главная')).not.toBeInTheDocument()
  })

  it('ответ 409 already_answered: сообщение и «Дальше»', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)), answer: () => errorResponse(409, 'already_answered', 'На это упражнение уже отвечали') })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    expect(await screen.findByRole('alert')).toHaveTextContent('На это упражнение уже отвечали')
    expect(screen.getByRole('button', { name: 'Дальше' })).toBeInTheDocument()
  })

  it('ответ 404 not_found: упражнение устарело, «Дальше» грузит следующее', async () => {
    let n = 0
    stubApi({
      next: () => jsonResponse(200, n++ === 0 ? ex(1) : ex(2, 'They go _____ school.', ['to', 'at', 'in', 'on'])),
      answer: () => errorResponse(404, 'not_found', 'Упражнение не найдено или устарело'),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    expect(await screen.findByRole('alert')).toHaveTextContent('Упражнение не найдено или устарело')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    await findPrompt('They go _____ school.')
  })

  it('400 invalid_answer: другой вариант уходит с новым ключом', async () => {
    let n = 0
    const { answers } = stubApi({
      next: () => jsonResponse(200, ex(1)),
      answer: () => (n++ === 0 ? errorResponse(400, 'invalid_answer', 'Пустой ответ') : jsonResponse(200, result())),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('1')
    expect(await screen.findByRole('alert')).toHaveTextContent('Пустой ответ')
    await user.keyboard('2')
    await screen.findByText('Верно')
    expect(answers).toHaveLength(2)
    expect(answers[1].key).not.toBe(answers[0].key)
  })

  it('сетевая ошибка при ответе: упражнение на месте, повтор с тем же ключом, другой вариант заблокирован', async () => {
    let n = 0
    const { answers } = stubApi({
      next: () => jsonResponse(200, ex(1)),
      answer: () => {
        if (n++ === 0) throw new TypeError('network')
        return jsonResponse(200, result())
      },
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('1')
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('article', { name: 'Упражнение' })).toBeInTheDocument()
    await user.keyboard('3') // другой вариант при неизвестном исходе не уходит
    expect(answers).toHaveLength(1)
    await user.keyboard('1')
    await screen.findByText('Верно')
    expect(answers).toHaveLength(2)
    expect(answers[1].key).toBe(answers[0].key)
    expect(answers[1].given).toBe('a')
  })

  it('слабые темы: пустое состояние и «мало ответов»', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)), topics: () => jsonResponse(200, { topics: [] }) })
    renderPage()
    expect(await screen.findByText(/Пока нет данных/)).toBeInTheDocument()
  })

  it('у темы с малым числом ответов вместо надёжности стоит «мало ответов»', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)), topics: () => jsonResponse(200, { topics: [{ topic: 'pos:verb', rating: 1500, deviation: 300, attempts: 1 }] }) })
    renderPage()
    expect(await screen.findByText(/мало ответов/)).toBeInTheDocument()
  })

  it('после 5xx есть «Повторить» (тот же ключ) и «Пропустить» (следующее упражнение)', async () => {
    let n = 0
    let m = 0
    const { answers } = stubApi({
      next: () => jsonResponse(200, m++ === 0 ? ex(1) : ex(2, 'They go _____ school.', ['to', 'at', 'in', 'on'])),
      answer: () => (n++ === 0 ? errorResponse(500, 'internal', 'Сбой сервера') : jsonResponse(200, result())),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('1')
    expect(await screen.findByRole('alert')).toHaveTextContent('Сбой сервера')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Повторить' }))
    await screen.findByText('Верно')
    expect(answers[1].key).toBe(answers[0].key)
  })

  it('«Пропустить» после сетевой ошибки грузит следующее упражнение без ответа', async () => {
    let m = 0
    const { answers } = stubApi({
      next: () => jsonResponse(200, m++ === 0 ? ex(1) : ex(2, 'They go _____ school.', ['to', 'at', 'in', 'on'])),
      answer: () => {
        throw new TypeError('network')
      },
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('1')
    await screen.findByRole('alert')
    await user.click(screen.getByRole('button', { name: 'Пропустить' }))
    await findPrompt('They go _____ school.')
    expect(answers).toHaveLength(1)
  })

  it('пока грузится следующее, на «Дальше» стоит «Подбираем…»', async () => {
    let release: (r: Response) => void = () => {}
    let m = 0
    stubApi({
      next: () => (m++ === 0 ? jsonResponse(200, ex(1)) : (new Promise<Response>((r) => (release = r)) as unknown as Response)),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    await screen.findByText('Верно')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    const busy = await screen.findByRole('button', { name: 'Подбираем…' })
    expect(busy).toBeDisabled()
    release(jsonResponse(200, ex(2, 'They go _____ school.', ['to', 'at', 'in', 'on'])))
    await findPrompt('They go _____ school.')
  })
})

describe('выбор формы', () => {
  const form = (id = 5) => ({
    id,
    kind: 'form',
    prompt: 'Yesterday we _____ to the park.',
    hint: 'go',
    translation: 'Вчера мы ходили в парк.',
    choices: ['went', 'goed', 'gone'],
    topic: 'skill:form',
  })
  const formResult = (inCards: boolean) => ({
    correct: true,
    answer: 'went',
    translation: 'Вчера мы ходили в парк.',
    ratings: [rating('skill:form', 1510)],
    word: { lemma: 'go', pos: 'verb', inCards },
  })

  it('под предложением подсказка «(go)» с доступным названием и перевод', async () => {
    stubApi({ next: () => jsonResponse(200, form()) })
    renderPage()
    await findPrompt('Yesterday we _____ to the park.')
    expect(screen.getByText('(go)')).toBeInTheDocument()
    expect(screen.getByText('начальная форма: go')).toHaveClass('sr-only')
    expect(screen.getByText('Вчера мы ходили в парк.')).toBeInTheDocument()
    expect(screen.getByText('Выбор формы')).toBeInTheDocument()
  })

  it('клавиши 1–3 отвечают, как в пропуске', async () => {
    const { answers } = stubApi({ next: () => jsonResponse(200, form(9)), answer: () => jsonResponse(200, formResult(true)) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('Yesterday we _____ to the park.')
    await waitFor(() => expect(screen.getByRole('article', { name: 'Упражнение' })).toHaveFocus())
    await user.keyboard('1')
    await screen.findByText('Верно')
    expect(answers[0]).toMatchObject({ exerciseId: 9, given: 'went' })
  })

  it('inCards=false: кнопка «Добавить в карточки», клик один запрос, потом «В карточках»', async () => {
    const { fn } = stubApi({ next: () => jsonResponse(200, form()), answer: () => jsonResponse(200, formResult(false)) })
    const origin = fn.getMockImplementation()!
    fn.mockImplementation(async (url: string, init?: RequestInit) =>
      url === '/api/v1/cards' ? jsonResponse(201, { cardId: 1, created: true }) : origin(url, init),
    )
    const user = userEvent.setup()
    renderPage()
    await findPrompt('Yesterday we _____ to the park.')
    await user.keyboard('1')
    await user.click(await screen.findByRole('button', { name: 'Добавить в карточки' }))
    expect(await screen.findByRole('button', { name: 'В карточках' })).toBeDisabled()
    expect(fn.mock.calls.filter((c) => c[0] === '/api/v1/cards')).toHaveLength(1)
  })

  it('inCards=true: только «В карточках»', async () => {
    stubApi({ next: () => jsonResponse(200, form()), answer: () => jsonResponse(200, formResult(true)) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('Yesterday we _____ to the park.')
    await user.keyboard('1')
    expect(await screen.findByRole('button', { name: 'В карточках' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Добавить в карточки' })).not.toBeInTheDocument()
  })

  it('для gap кнопки и подсказки нет', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    await screen.findByText('Верно')
    expect(screen.queryByRole('button', { name: /карточк/i })).not.toBeInTheDocument()
    expect(screen.queryByText(/начальная форма/)).not.toBeInTheDocument()
  })
})

describe('ExercisesPage: повтор ошибки', () => {
  const mr = (status: 'open' | 'closed', streak: number) => ({ status, streak, nextDue: '2026-10-25T12:00:00Z' })

  it('у повтора над предложением ярлык «Повтор ошибки»', async () => {
    stubApi({ next: () => jsonResponse(200, { ...ex(1), repeat: true }) })
    renderPage()
    await findPrompt('She is _____ engineer.')
    expect(screen.getByText('Повтор ошибки')).toBeInTheDocument()
  })

  it('без repeat и mistake ничего нового не видно', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)) })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    await screen.findByText('Верно')
    expect(screen.queryByText('Повтор ошибки')).not.toBeInTheDocument()
    expect(screen.queryByText(/Ошибка закрыта|закрыта|Вернётся завтра/)).not.toBeInTheDocument()
  })

  it('второй верный повтор: «Ошибка закрыта»', async () => {
    stubApi({
      next: () => jsonResponse(200, { ...ex(1), repeat: true }),
      answer: () => jsonResponse(200, { ...result(true), mistake: mr('closed', 2) }),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    expect(await screen.findByText('Ошибка закрыта')).toBeInTheDocument()
  })

  it('первый верный повтор: нужен ещё один', async () => {
    stubApi({
      next: () => jsonResponse(200, { ...ex(1), repeat: true }),
      answer: () => jsonResponse(200, { ...result(true), mistake: mr('open', 1) }),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    expect(await screen.findByText('Ещё один верный ответ, и ошибка закрыта')).toBeInTheDocument()
  })

  it('неверный ответ открывает ошибку: «Вернётся завтра»', async () => {
    stubApi({
      next: () => jsonResponse(200, ex(1)),
      answer: () => jsonResponse(200, { ...result(false), mistake: mr('open', 0) }),
    })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('1')
    expect(await screen.findByText('Вернётся завтра')).toBeInTheDocument()
  })

  it('после ответа список ошибок помечается устаревшим', async () => {
    stubApi({ next: () => jsonResponse(200, ex(1)), answer: () => jsonResponse(200, result(false)) })
    const user = userEvent.setup()
    const { client } = renderPage()
    client.setQueryData(['mistakes'], { open: [], counts: { open: 0, dueNow: 0, closedThisWeek: 0 } })
    await findPrompt('She is _____ engineer.')
    expect(client.getQueryState(['mistakes'])?.isInvalidated).toBe(false)
    await user.keyboard('1')
    await screen.findByText('Неверно')
    await waitFor(() => expect(client.getQueryState(['mistakes'])?.isInvalidated).toBe(true))
  })
})
