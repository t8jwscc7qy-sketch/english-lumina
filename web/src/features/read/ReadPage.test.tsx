import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { STATUS } from '@/features/learn/fixtures'
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

describe('ReadPage: конструкции', () => {
  it('«Встретилось»: кнопки конструкций, пояснение с примером из текста и ссылкой на шпаргалку', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    expect(await screen.findByText('Встретилось:')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: /Конструкция/ })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Past Simple' }))
    const panel = screen.getByRole('region', { name: 'Конструкция: Past Simple' })
    expect(panel).toHaveTextContent('Законченное действие в прошлом.')
    expect(panel).toHaveTextContent('I like my dog.') // предложение с номером 1 из этого текста
    expect(panel).toHaveTextContent('She visited her aunt.')
    expect(screen.getByRole('link', { name: 'Шпаргалка по теме' })).toHaveAttribute('href', '/sheets/irregular-verbs')
    await user.click(screen.getByRole('button', { name: 'Модальные глаголы' }))
    expect(screen.getByRole('region', { name: 'Конструкция: Модальные глаголы' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Шпаргалка по теме' })).not.toBeInTheDocument()
    await user.keyboard('{Escape}')
  })

  it('нет конструкций — блока «Встретилось» нет', async () => {
    stub(() => jsonResponse(200, { ...TEXT, constructions: [] }))
    render()
    await screen.findByText(/Понимание этого текста/)
    expect(screen.queryByText('Встретилось:')).not.toBeInTheDocument()
  })

  it('«Закрыть» убирает пояснение', async () => {
    stub(() => jsonResponse(200, TEXT))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('button', { name: 'Модальные глаголы' }))
    await user.click(within(screen.getByRole('region', { name: 'Конструкция: Модальные глаголы' })).getByRole('button', { name: 'Закрыть' }))
    expect(screen.queryByRole('region', { name: /Конструкция/ })).not.toBeInTheDocument()
  })
})

describe('ReadPage: холодный старт', () => {
  it('нет текста: сколько слов в учёте и сколько осталось до первого текста', async () => {
    stub(
      () => errorResponse(404, 'no_text', 'Пока не из чего собрать текст.'),
      (url) => (url.endsWith('/learn/status') ? jsonResponse(200, { ...STATUS, trackedWords: 6, textMinWords: 20 }) : null),
    )
    render()
    expect(await screen.findByText('В учёте слов: 6 из 20. Осталось 14: первый блок «Знакомства» даёт до 7 слов.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Знакомство' })).toHaveAttribute('href', '/learn')
  })

  it('порог набран, а текста всё равно нет: подсказки про остаток нет', async () => {
    stub(
      () => errorResponse(404, 'no_text', 'Пока не из чего собрать текст.'),
      (url) => (url.endsWith('/learn/status') ? jsonResponse(200, { ...STATUS, trackedWords: 25, textMinWords: 20 }) : null),
    )
    render()
    await screen.findByRole('heading', { name: 'Пока не из чего собрать текст' })
    expect(screen.queryByText(/Осталось/)).not.toBeInTheDocument()
  })
})

describe('ReadPage: авторские тексты', () => {
  it('у авторского текста нет атрибуции Tatoeba, в источниках сказано, что текст авторский', async () => {
    const authored = {
      ...TEXT,
      sentences: TEXT.sentences.map((s) => ({ en: s.en, ru: s.ru, tokens: s.tokens })),
    }
    stub(() => jsonResponse(200, authored))
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByText('Источники предложений'))
    expect(screen.getByText(/Авторский текст проекта Verst/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '#101' })).not.toBeInTheDocument()
    // слова по-прежнему нажимаются
    await user.click((await screen.findAllByRole('button', { name: 'dog' }))[0])
    expect(screen.getByRole('region', { name: 'Слово dog' })).toBeInTheDocument()
  })
})

describe('ReadPage: живая озвучка', () => {
  it('у предложения с записью — «▶ Запись», без — «▶ Голос»; в источниках автор и лицензия записи', async () => {
    class FakeUtt {
      text: string
      constructor(text: string) {
        this.text = text
      }
    }
    vi.stubGlobal('SpeechSynthesisUtterance', FakeUtt)
    vi.stubGlobal('speechSynthesis', { cancel: () => {}, speak: () => {} })
    const withAudio = {
      ...TEXT,
      sentences: [
        { ...TEXT.sentences[0], audio: { url: 'https://tatoeba.org/audio/download/55', author: 'alice', license: 'CC BY 4.0', attributionUrl: 'https://example.org/alice' } },
        TEXT.sentences[1],
      ],
    }
    stub(() => jsonResponse(200, withAudio))
    const user = userEvent.setup()
    render()
    expect(await screen.findByRole('button', { name: 'Прослушать предложение 1' })).toHaveTextContent('▶ Запись')
    expect(screen.getByRole('button', { name: 'Прослушать предложение 2' })).toHaveTextContent('▶ Голос')
    await user.click(screen.getByText('Источники предложений'))
    expect(screen.getByText(/озвучка:/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'alice' })).toHaveAttribute('href', 'https://example.org/alice')
    expect(screen.getByText(/CC BY 4\.0/)).toBeInTheDocument()
  })

  it('нет ни синтеза, ни записи — кнопок звука на предложениях нет', async () => {
    stub(() => jsonResponse(200, TEXT))
    render()
    await screen.findByText(/Понимание этого текста/)
    expect(screen.queryByRole('button', { name: /Прослушать предложение/ })).not.toBeInTheDocument()
  })
})
