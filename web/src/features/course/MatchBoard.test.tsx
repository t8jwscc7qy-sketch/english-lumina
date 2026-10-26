import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { MatchBoard } from './MatchBoard'

describe('MatchBoard', () => {
  it('одинаковый перевод у двух слов не вешает задание: пары собираются по номерам', async () => {
    const onDone = vi.fn()
    const user = userEvent.setup()
    render(
      <MatchBoard
        drill={{ kind: 'match', old: false, pairs: [{ en: 'fine', ru: 'хорошо' }, { en: 'well', ru: 'хорошо' }, { en: 'hello', ru: 'привет' }] }}
        onDone={onDone}
      />,
    )
    for (const en of ['fine', 'well']) {
      await user.click(screen.getByRole('button', { name: en }))
      await user.click(screen.getAllByRole('button', { name: 'хорошо' }).find((b) => !(b as HTMLButtonElement).disabled)!)
    }
    await user.click(screen.getByRole('button', { name: 'hello' }))
    await user.click(screen.getByRole('button', { name: 'привет' }))
    expect(onDone).toHaveBeenCalledWith(0)
  })

  it('неверная пара считается ошибкой и отмечается у перевода', async () => {
    const onDone = vi.fn()
    const user = userEvent.setup()
    render(<MatchBoard drill={{ kind: 'match', old: false, pairs: [{ en: 'how', ru: 'как' }, { en: 'hello', ru: 'привет' }, { en: 'I', ru: 'я' }] }} onDone={onDone} />)
    await user.click(screen.getByRole('button', { name: 'how' }))
    await user.click(screen.getByRole('button', { name: 'привет' }))
    expect(screen.getByRole('status')).toHaveTextContent('Неверная пара')
    for (const [en, ru] of [['how', 'как'], ['hello', 'привет'], ['I', 'я']]) {
      await user.click(screen.getByRole('button', { name: en }))
      await user.click(screen.getByRole('button', { name: ru }))
    }
    expect(onDone).toHaveBeenCalledWith(1)
  })
})
