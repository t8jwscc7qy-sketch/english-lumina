import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
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
    expect(screen.getByRole('link', { name: /Добавить слова|Слова/ })).toBeInTheDocument()
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

  it('смена фокуса окна не меняет упражнение и не шлёт повторный GET', async () => {
    const { fn } = stubApi({ next: () => jsonResponse(200, ex(1)) })
    renderPage()
    await findPrompt('She is _____ engineer.')
    fireEvent.focus(window)
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new Event('online'))
    await new Promise((r) => setTimeout(r, 50))
    expect(fn.mock.calls.filter(([u]) => String(u).endsWith('/exercises/next'))).toHaveLength(1)
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

  it.each([
    [409, 'already_answered'],
    [404, 'not_found'],
  ])('ответ %i %s: сообщение и кнопка «Дальше»', async (status, code) => {
    stubApi({ next: () => jsonResponse(200, ex(1)), answer: () => errorResponse(status, code, 'Не вышло') })
    const user = userEvent.setup()
    renderPage()
    await findPrompt('She is _____ engineer.')
    await user.keyboard('2')
    expect(await screen.findByRole('alert')).toHaveTextContent('Не вышло')
    expect(screen.getByRole('button', { name: 'Дальше' })).toBeInTheDocument()
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
    await screen.findByText('Неверно').catch(() => screen.findByText('Верно'))
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
})
