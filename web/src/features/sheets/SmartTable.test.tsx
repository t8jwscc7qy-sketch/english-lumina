import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { SHEET } from './fixtures'
import { SmartTable } from './SmartTable'

const table = SHEET.blocks[1]

const bodyCells = () =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((r) => within(r).getAllByRole('cell')[0].textContent)

describe('SmartTable', () => {
  it('статус строки значком и словом', () => {
    render(<SmartTable block={table} />)
    expect(screen.getByText('трудно')).toBeInTheDocument()
    expect(screen.getByText('не встречал')).toBeInTheDocument()
    expect(screen.getByText('изучено')).toBeInTheDocument()
  })

  it('фильтры «Мои слабые» и «Ещё не учил»', async () => {
    const user = userEvent.setup()
    render(<SmartTable block={table} />)
    await user.click(screen.getByRole('button', { name: 'Мои слабые (1)' }))
    expect(bodyCells()).toEqual(['go'])
    await user.click(screen.getByRole('button', { name: 'Ещё не учил' }))
    expect(bodyCells()).toEqual(['see'])
    await user.click(screen.getByRole('button', { name: 'Все' }))
    expect(bodyCells()).toEqual(['go', 'see', 'take'])
  })

  it('сортировка по столбцу меняет порядок и направление', async () => {
    const user = userEvent.setup()
    render(<SmartTable block={table} />)
    await user.click(screen.getByRole('button', { name: /Сортировать по столбцу «Past»/ }))
    expect(bodyCells()).toEqual(['see', 'take', 'go'])
    expect(screen.getByRole('columnheader', { name: /Past/ })).toHaveAttribute('aria-sort', 'ascending')
    await user.click(screen.getByRole('button', { name: /Сортировать по столбцу «Past»/ }))
    expect(bodyCells()).toEqual(['go', 'take', 'see'])
  })

  it('поиск появляется у длинных таблиц и фильтрует строки', async () => {
    const user = userEvent.setup()
    const rows = Array.from({ length: 9 }, (_, i) => ({ key: `w${i}`, cells: [`word${i}`, `x${i}`], status: 'unseen' as const }))
    render(<SmartTable block={{ ...table, rows }} />)
    await user.type(screen.getByRole('searchbox', { name: 'Поиск по таблице' }), 'word7')
    expect(bodyCells()).toEqual(['word7'])
    await user.clear(screen.getByRole('searchbox', { name: 'Поиск по таблице' }))
    await user.type(screen.getByRole('searchbox', { name: 'Поиск по таблице' }), 'нет такого')
    expect(screen.getByText(/Ничего не нашлось/)).toBeInTheDocument()
  })

  it('обычная таблица без привязки: без статуса и фильтров', () => {
    render(<SmartTable block={{ type: 'table', columns: [{ title: 'A' }, { title: 'B' }], rows: [{ cells: ['**x**', 'y'] }] }} />)
    expect(screen.queryByRole('button', { name: /Мои слабые/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('columnheader', { name: 'Статус' })).not.toBeInTheDocument()
    expect(screen.getByText('x').tagName).toBe('STRONG')
  })

  it('пустые «Мои ошибки» — пояснение, а не пустая таблица', () => {
    render(<SmartTable block={{ type: 'table', bind: 'mistake', columns: [{ title: 'Задание' }, { title: 'Ответ' }], rows: [] }} />)
    expect(screen.getByText(/Открытых ошибок нет/)).toBeInTheDocument()
  })

  it('тема: слабая помечена, рейтинг показан', () => {
    render(
      <SmartTable
        block={{ type: 'table', bind: 'topic', columns: [{ title: 'Тема' }, { title: 'Что' }], rows: [{ key: 'grammar:articles', cells: ['Артикли', 'a/the'], status: 'weak', rating: 1200.4 }] }}
      />,
    )
    expect(screen.getByText('слабая')).toBeInTheDocument()
    expect(screen.getByText('1200')).toBeInTheDocument()
  })
})
