import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { AddToCards } from './AddToCards'

afterEach(() => vi.unstubAllGlobals())

function renderButton(inCards: boolean) {
  return renderAt('/', [{ path: '/', element: <AddToCards word={{ lemma: 'go', pos: 'verb', inCards }} /> }])
}
const posts = (fn: ReturnType<typeof vi.fn>) => fn.mock.calls.filter((c) => c[0] === '/api/v1/cards' && c[1]?.method === 'POST')

describe('AddToCards', () => {
  it('клик отправляет слово один раз и меняет подпись на «В карточках»', async () => {
    const fn = vi.fn(async () => jsonResponse(201, { cardId: 1, created: true }))
    vi.stubGlobal('fetch', fn)
    const user = userEvent.setup()
    renderButton(false)
    const button = screen.getByRole('button', { name: 'Добавить в карточки' })
    await user.dblClick(button)
    const done = await screen.findByRole('button', { name: 'В карточках' })
    expect(done).toBeDisabled()
    expect(posts(fn)).toHaveLength(1)
    expect(JSON.parse(posts(fn)[0][1].body as string)).toEqual({ lemma: 'go', pos: 'verb' })
  })

  it('слово уже в карточках: только тихое «В карточках», запросов нет', () => {
    const fn = vi.fn()
    vi.stubGlobal('fetch', fn)
    renderButton(true)
    expect(screen.getByRole('button', { name: 'В карточках' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Добавить в карточки' })).not.toBeInTheDocument()
    expect(fn).not.toHaveBeenCalled()
  })

  it('409 no_examples: сообщение под кнопкой, кнопка остаётся доступной', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errorResponse(409, 'no_examples', 'Для этого слова нет подходящих примеров')))
    renderButton(false)
    await userEvent.setup().click(screen.getByRole('button', { name: 'Добавить в карточки' }))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('нет подходящих примеров')
    expect(alert).toHaveClass('text-coral-text')
    expect(screen.getByRole('button', { name: 'Добавить в карточки' })).toBeEnabled()
  })

  it('нажатие мышью не уводит фокус с карточки', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(201, { cardId: 1, created: true })))
    renderButton(false)
    const button = screen.getByRole('button', { name: 'Добавить в карточки' })
    const prevented = !button.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }))
    expect(prevented).toBe(true)
  })
})
