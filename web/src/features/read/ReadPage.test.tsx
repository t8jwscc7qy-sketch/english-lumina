import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { TEXT } from './fixtures'
import { ReadPage } from './ReadPage'

type Call = { url: string; method: string; body: unknown }

function stub(next: () => Response, extra: (url: string) => Response | null = () => null) {
  const calls: Call[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined })
      const e = extra(url)
      if (e) return e
      if (url.includes('/texts/next')) return next()
      if (url.endsWith('/texts/rate')) return jsonResponse(200, { goal: 0.83, changed: true })
      if (url.endsWith('/texts/know-word')) return jsonResponse(200, { marked: true })
      if (url.endsWith('/cards')) return jsonResponse(201, { cardId: 1, created: true })
      return jsonResponse(200, {})
    }),
  )
  return calls
}

const render = (path = '/read') => renderAt(path, [{ path: '/read', element: <ReadPage /> }])

beforeEach(() => {
  // По умолчанию синтеза речи нет: чтение работает без звука.
  vi.stubGlobal('speechSynthesis', undefined)
})
afterEach(() => vi.unstubAllGlobals())

describe('ReadPage', () => {
  it('показывает понимание, новые слова и текст с кнопками-словами', async () => {
    const calls = stub(() => jsonResponse(200, TEXT))
    render()
    expect(await screen.findByText('Понимание этого текста: 86% (цель 85%)')).toBeInTheDocument()
    expect(screen.getByText('Новые слова: dog, run')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'dog' })).toHaveLength(2)
    expect(screen.getByText(/не рассказ/)).toBeInTheDocument()
    expect(calls[0].url).toContain('mode=read')
  })

  it('нажатие на слово открывает подсказку; «Добавить в карточки» и «Знаю» шлют запросы', async () => {
    const calls = stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await user.click((await screen.findAllByRole('button', { name: 'dog' }))[0])
    const panel = screen.getByRole('region', { name: 'Слово dog' })
    expect(panel).toHaveTextContent('собака')
    expect(panel).toHaveTextContent('/dɔɡ/')
    await user.click(screen.getByRole('button', { name: 'Добавить в карточки' }))
    expect(await screen.findByRole('button', { name: 'Добавлено в карточки' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Знаю' }))
    expect(await screen.findByRole('button', { name: 'Отмечено как известное' })).toBeDisabled()
    expect(calls.find((c) => c.url.endsWith('/cards'))?.body).toEqual({ lemma: 'dog', pos: 'noun' })
    expect(calls.find((c) => c.url.endsWith('/texts/know-word'))?.body).toEqual({ lexemeId: 11 })
  })

  it('слово без подсказки: честное сообщение, без кнопок', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: 'fast' }))
    expect(screen.getByText(/Подсказки для этой формы пока нет/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Знаю' })).not.toBeInTheDocument()
  })

  it('слово с апострофом нажимается (can\'t)', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: "can't" }))
    expect(screen.getByRole('region', { name: "Слово can't" })).toBeInTheDocument()
  })

  it('перевод предложения по кнопке и «Все переводы»', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await screen.findAllByRole('button', { name: 'Перевод' })
    expect(screen.queryByText('Я люблю свою собаку.')).not.toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Перевод' })[1])
    expect(screen.getByText('Я люблю свою собаку.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Все переводы' }))
    expect(screen.getByText('Собака не может быстро бегать.')).toBeInTheDocument()
  })

  it('«Как понял»: одна оценка с textId и режимом, затем «Следующий текст»', async () => {
    let n = 0
    const calls = stub(() => jsonResponse(200, { ...TEXT, id: ++n === 1 ? 7 : 8 }))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: 'Почти всё' }))
    expect(await screen.findByText(/Цель понимания теперь 83%/)).toBeInTheDocument()
    expect(calls.filter((c) => c.url.endsWith('/texts/rate'))).toHaveLength(1)
    expect(calls.find((c) => c.url.endsWith('/texts/rate'))?.body).toEqual({ textId: 7, understood: 'easy', mode: 'read' })
    await user.click(screen.getByRole('button', { name: 'Следующий текст' }))
    await waitFor(() => expect(calls.filter((c) => c.url.includes('/texts/next'))).toHaveLength(2))
    expect(await screen.findByRole('button', { name: 'Почти всё' })).toBeInTheDocument()
  })

  it('no_text — совет начать со знакомства, не ошибка', async () => {
    stub(() => errorResponse(404, 'no_text', 'Пока не из чего собрать текст.'))
    render()
    expect(await screen.findByRole('heading', { name: 'Пока не из чего собрать текст' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Знакомство' })).toHaveAttribute('href', '/learn')
  })

  it('другая ошибка — сообщение и «Повторить»', async () => {
    stub(() => errorResponse(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    render()
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
  })

  it('режим «слушать»: текст скрыт до «Показать текст», запрос с mode=listen', async () => {
    const calls = stub(() => jsonResponse(200, { ...TEXT, mode: 'listen' }))
    const user = userEvent.setup()
    render('/read?mode=listen')
    expect(await screen.findByRole('button', { name: 'Показать текст' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'dog' })).not.toBeInTheDocument()
    expect(calls[0].url).toContain('mode=listen')
    await user.click(screen.getByRole('button', { name: 'Показать текст' }))
    expect(screen.getAllByRole('button', { name: 'dog' }).length).toBeGreaterThan(0)
  })

  it('нет синтеза речи — кнопок звука нет, чтение работает', async () => {
    stub(() => jsonResponse(200, TEXT))
    render()
    await screen.findByText(/Понимание этого текста/)
    expect(screen.queryByRole('button', { name: 'Прослушать' })).not.toBeInTheDocument()
  })

  it('есть синтез речи — «Прослушать» читает предложения подряд с выбранной скоростью', async () => {
    const spoken: { text: string; rate: number }[] = []
    class Utt {
      text: string
      lang = ''
      rate = 1
      onstart: (() => void) | null = null
      onend: (() => void) | null = null
      onerror: (() => void) | null = null
      onboundary: ((e: { charIndex: number }) => void) | null = null
      constructor(t: string) {
        this.text = t
      }
    }
    vi.stubGlobal('SpeechSynthesisUtterance', Utt)
    vi.stubGlobal('speechSynthesis', {
      cancel: () => {},
      speak: (u: Utt) => {
        spoken.push({ text: u.text, rate: u.rate })
        queueMicrotask(() => {
          u.onstart?.()
          u.onend?.()
        })
      },
    })
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await user.selectOptions(await screen.findByLabelText('Скорость'), '0.7')
    await user.click(screen.getByRole('button', { name: 'Прослушать' }))
    await waitFor(() => expect(spoken).toHaveLength(2))
    expect(spoken[0]).toEqual({ text: "The dog can't run fast.", rate: 0.7 })
    expect(spoken[1].text).toBe('I like my dog.')
  })

  it('тексты из данных выводятся только текстом', async () => {
    const evil = { ...TEXT, sentences: [{ ...TEXT.sentences[0], en: '<img src=x onerror=alert(1)> dog here now', tokens: ['dog'] }, TEXT.sentences[1]] }
    stub(() => jsonResponse(200, evil))
    const { container } = render()
    await screen.findByText(/Понимание этого текста/)
    expect(container.querySelector('img')).toBeNull()
  })
})

describe('ReadPage: исправления после ревью', () => {
  it('источники: оба автора и оба номера', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByText('Источники предложений'))
    expect(screen.getByRole('link', { name: '#101' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '#201' })).toBeInTheDocument()
    expect(screen.getByText(/ivan, Tatoeba/)).toBeInTheDocument()
  })

  it('«Другой текст» просит новый (new=1) и показывает его', async () => {
    let n = 0
    const calls = stub(() => jsonResponse(200, { ...TEXT, id: ++n === 1 ? 7 : 8 }))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: 'Другой текст' }))
    await waitFor(() => expect(calls.some((c) => c.url.includes('new=1'))).toBe(true))
    expect(await screen.findByRole('button', { name: 'Почти всё' })).toBeInTheDocument()
  })

  it('панель слова забирает фокус, Esc закрывает и возвращает фокус на слово', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    const word = (await screen.findAllByRole('button', { name: 'dog' }))[0]
    await user.click(word)
    const panel = screen.getByRole('region', { name: 'Слово dog' })
    expect(panel).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('region', { name: 'Слово dog' })).not.toBeInTheDocument()
    await waitFor(() => expect(document.activeElement?.getAttribute('data-word')).toBe('dog'))
  })
})
