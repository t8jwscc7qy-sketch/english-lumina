import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { nextSkip } from './learn'
import { PlacementPage } from './PlacementPage'

afterEach(() => vi.unstubAllGlobals())

const WORDS = [
  { lexemeId: 1, lemma: 'apple', pos: 'noun' },
  { lexemeId: 2, lemma: 'run', pos: 'verb' },
  { lexemeId: 3, lemma: 'big', pos: 'adjective' },
]

function stub(next: 'more' | 'learn' | 'stop', opts: { hasMore?: boolean; submitFails?: boolean } = {}) {
  const calls = { submit: [] as { known: number[]; shown: number[]; key: string }[], gets: [] as string[] }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init?: RequestInit) => {
      if (url.includes('/learn/placement') && init?.method === 'POST') {
        calls.submit.push(JSON.parse(String(init.body)))
        return opts.submitFails ? errorResponse(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз') : jsonResponse(200, { marked: calls.submit.at(-1)!.known.length, next })
      }
      if (url.includes('/learn/placement')) {
        calls.gets.push(url)
        return jsonResponse(200, { words: calls.gets.length > 1 ? [] : WORDS, skip: 0, hasMore: opts.hasMore ?? false })
      }
      return jsonResponse(200, {})
    }),
  )
  return calls
}

const render = () =>
  renderAt('/learn/placement', [
    { path: '/learn/placement', element: <PlacementPage /> },
    { path: '/learn', element: <p>знакомство</p> },
  ])

describe('PlacementPage', () => {
  it('по умолчанию ничего не отмечено; «знаю» меняется Пробелом; отправляет known и shown', async () => {
    const calls = stub('learn')
    const user = userEvent.setup()
    render()
    const apple = await screen.findByRole('checkbox', { name: /apple/ })
    expect(apple).not.toBeChecked()
    apple.focus()
    await user.keyboard(' ')
    expect(apple).toBeChecked()
    await user.click(screen.getByRole('button', { name: /Готово/ }))
    expect(await screen.findByRole('heading', { name: 'Отмечено слов: 1' })).toBeInTheDocument()
    expect(calls.submit[0]).toMatchObject({ known: [1], shown: [1, 2, 3] })
    expect(calls.submit[0].key).toBeTruthy()
    expect(screen.getByRole('link', { name: 'К знакомству' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Ещё страница' })).not.toBeInTheDocument()
  })

  it('next = more и есть ещё слова: «Ещё страница» просит следующую с пропуском «не знаю»', async () => {
    const calls = stub('more', { hasMore: true })
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('checkbox', { name: /apple/ }))
    await user.click(screen.getByRole('button', { name: /Готово/ }))
    await user.click(await screen.findByRole('button', { name: 'Ещё страница' }))
    await screen.findByRole('heading', { name: 'Слов для теста больше нет' })
    expect(calls.gets.at(-1)).toContain('skip=2')
  })

  it('next = stop — «тест не нужен»', async () => {
    stub('stop')
    const user = userEvent.setup()
    render()
    await screen.findByRole('checkbox', { name: /apple/ })
    await user.click(screen.getByRole('button', { name: /Готово/ }))
    expect(await screen.findByText(/тест не нужен/)).toBeInTheDocument()
  })

  it('ошибка отправки остаётся на странице, отметки сохраняются', async () => {
    stub('learn', { submitFails: true })
    const user = userEvent.setup()
    render()
    await user.click(await screen.findByRole('checkbox', { name: /apple/ }))
    await user.click(screen.getByRole('button', { name: /Готово/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Внутренняя ошибка')
    expect(screen.getByRole('checkbox', { name: /apple/ })).toBeChecked()
  })
})

describe('nextSkip', () => {
  it('«знаю» из списка пропадают, «не знаю» остаются', () => {
    expect(nextSkip(0, 50, 45)).toBe(5)
    expect(nextSkip(5, 50, 0)).toBe(55)
  })
})
