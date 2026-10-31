import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { BLOCK, REASON } from './fixtures'
import { LearnPage } from './LearnPage'

afterEach(() => vi.unstubAllGlobals())

function stub(block: () => Response, introduce?: () => Response) {
  const calls: { introduce: unknown[] } = { introduce: [] }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/learn/introduce')) {
        calls.introduce.push(JSON.parse(String(init?.body)))
        return introduce ? introduce() : jsonResponse(200, { introduced: 2, cardIds: [1, 2] })
      }
      if (url.endsWith('/learn/block')) return block()
      return jsonResponse(200, {})
    }),
  )
  return calls
}

const render = () =>
  renderAt('/learn', [
    { path: '/learn', element: <LearnPage /> },
    { path: '/cards', element: <p>страница карточек</p> },
  ])

describe('LearnPage', () => {
  it('показывает слово: перевод, IPA, примеры с выделением и атрибуцией', async () => {
    stub(() => jsonResponse(200, BLOCK))
    const { container } = render()
    expect(await screen.findByRole('heading', { name: 'apple' })).toBeInTheDocument()
    expect(screen.getByText('Слово 1 из 2')).toBeInTheDocument()
    expect(screen.getByText('яблоко')).toHaveAttribute('lang', 'ru')
    expect(container.querySelector('[lang="en-fonipa"]')).toHaveTextContent('/ˈæpəl/')
    expect(container.querySelector('mark')).toHaveTextContent('apple')
    expect(screen.getByText('Я ем яблоко каждый день.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '#11' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Прослушать слово apple' })).toBeInTheDocument()
  })

  it('«Дальше», «Назад» и стрелки двигают шаги', async () => {
    stub(() => jsonResponse(200, BLOCK))
    const user = userEvent.setup()
    render()
    await screen.findByRole('heading', { name: 'apple' })
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(screen.getByRole('heading', { name: 'run' })).toBeInTheDocument()
    expect(screen.getByText('бежать, управлять')).toBeInTheDocument()
    await user.keyboard('{ArrowLeft}')
    expect(screen.getByRole('heading', { name: 'apple' })).toBeInTheDocument()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('heading', { name: 'run' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Начать повторять' })).toBeInTheDocument()
  })

  it('после последнего слова вводит все слова с ключом и уходит на карточки', async () => {
    const calls = stub(() => jsonResponse(200, BLOCK))
    const user = userEvent.setup()
    render()
    await screen.findByRole('heading', { name: 'apple' })
    await user.keyboard('{ArrowRight}')
    await user.click(screen.getByRole('button', { name: 'Начать повторять' }))
    expect(await screen.findByText('страница карточек')).toBeInTheDocument()
    expect(calls.introduce).toHaveLength(1)
    expect(calls.introduce[0]).toMatchObject({ lexemeIds: [1, 2] })
    expect((calls.introduce[0] as { key: string }).key).toBeTruthy()
  })

  it('ошибка ввода остаётся на странице, повторное нажатие шлёт тот же ключ', async () => {
    let n = 0
    const calls = stub(
      () => jsonResponse(200, { ...BLOCK, words: [BLOCK.words[0]] }),
      () => (n++ === 0 ? errorResponse(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз') : jsonResponse(200, { introduced: 1, cardIds: [1] })),
    )
    const user = userEvent.setup()
    render()
    await screen.findByRole('heading', { name: 'apple' })
    await user.click(screen.getByRole('button', { name: 'Начать повторять' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    await user.click(screen.getByRole('button', { name: 'Начать повторять' }))
    expect(await screen.findByText('страница карточек')).toBeInTheDocument()
    expect((calls.introduce[0] as { key: string }).key).toBe((calls.introduce[1] as { key: string }).key)
  })

  it('404 no_words — спокойное сообщение, не ошибка', async () => {
    stub(() => errorResponse(404, 'no_words', 'Слов для знакомства пока нет'))
    render()
    expect(await screen.findByRole('heading', { name: 'Слов для знакомства пока нет' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('лимит исчерпан — «На сегодня всё» с причиной', async () => {
    stub(() => jsonResponse(200, { words: [], limit: 5, remaining: 0, reason: { code: 'done', text: 'На сегодня всё' } }))
    render()
    expect(await screen.findByRole('heading', { name: 'На сегодня всё' })).toBeInTheDocument()
  })

  it('ошибка загрузки показывает сообщение и кнопку «Повторить»', async () => {
    stub(() => errorResponse(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    render()
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
  })

  it('тексты из базы выводятся только текстом', async () => {
    const evil = { ...BLOCK.words[0], lemma: '<img src=x onerror=alert(1)>', translations: ['<b>жирный</b>'] }
    stub(() => jsonResponse(200, { ...BLOCK, words: [evil], reason: REASON }))
    const { container } = render()
    await waitFor(() => expect(container.querySelector('h2')).toBeInTheDocument())
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
  })
})

describe('LearnPage: фразы со звуком', () => {
  it('у фразы с записью «▶ Запись», без записи при отсутствии синтеза кнопки нет', async () => {
    const withAudio = {
      ...BLOCK,
      words: [
        { ...BLOCK.words[0], examples: [{ ...BLOCK.words[0].examples[0], en: { ...BLOCK.words[0].examples[0].en, audio: { url: 'https://tatoeba.org/audio/download/9', author: 'alice', license: 'CC BY 4.0' } } }] },
        BLOCK.words[1],
      ],
    }
    stub(() => jsonResponse(200, withAudio))
    const user = userEvent.setup()
    render()
    await screen.findByRole('heading', { name: 'apple' })
    expect(screen.getByRole('button', { name: /Прослушать фразу: I eat an apple/ })).toHaveTextContent('▶ Запись')
    await user.click(screen.getByRole('button', { name: 'Дальше' }))
    expect(screen.queryByRole('button', { name: /Прослушать фразу/ })).not.toBeInTheDocument()
  })
})
