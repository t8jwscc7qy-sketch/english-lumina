import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StageMap } from './StageMap'

describe('StageMap', () => {
  it('показывает пять уровней по порядку', () => {
    render(<StageMap current="B1" target="B2" forecast="collecting" />)
    const items = within(screen.getByRole('list', { name: 'Путь по уровням' })).getAllByRole(
      'listitem',
    )
    expect(items.map((li) => li.textContent?.match(/A1|A2|B1|B2|C1/)?.[0])).toEqual([
      'A1',
      'A2',
      'B1',
      'B2',
      'C1',
    ])
  })

  it('отмечает текущий уровень как «ты здесь» и ставит aria-current', () => {
    render(<StageMap current="B1" target="B2" forecast="collecting" />)
    const current = screen.getByRole('listitem', { current: 'step' })
    expect(current).toHaveTextContent('B1')
    expect(current).toHaveTextContent('Ты здесь')
  })

  it('отмечает пройденные уровни и цель для вспомогательных технологий', () => {
    render(<StageMap current="B1" target="B2" forecast="collecting" />)
    expect(screen.getByRole('listitem', { name: /A1.*пройден/i })).toBeInTheDocument()
    expect(screen.getByRole('listitem', { name: /A2.*пройден/i })).toBeInTheDocument()
    expect(screen.getByRole('listitem', { name: /B2.*цель/i })).toBeInTheDocument()
  })

  it('пока прогноз собирается, не называет дату', () => {
    render(<StageMap current="B1" target="B2" forecast="collecting" />)
    expect(screen.getByText(/собираю данные/i)).toBeInTheDocument()
    expect(screen.queryByText(/\d{4}/)).not.toBeInTheDocument()
  })

  it('на первом уровне нет пройденных', () => {
    render(<StageMap current="A1" target="B2" forecast="collecting" />)
    expect(screen.queryByRole('listitem', { name: /пройден/i })).not.toBeInTheDocument()
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent('A1')
  })

  it('когда цель достигнута, моста до цели нет', () => {
    render(<StageMap current="B2" target="B2" forecast="collecting" />)
    expect(screen.queryByText(/собираю данные/i)).not.toBeInTheDocument()
  })
})
