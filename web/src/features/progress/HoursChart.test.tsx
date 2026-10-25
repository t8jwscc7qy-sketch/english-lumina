import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { HoursChart } from './HoursChart'

function points(n: number, perDay: number, start = new Date(2026, 6, 28)) {
  return Array.from({ length: n }, (_, i) => {
    const d = new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)
    const day = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    return { day, minutes: perDay, cumulativeHours: ((i + 1) * perDay) / 60 }
  })
}

describe('HoursChart', () => {
  it('скрыт от читалок: таблица по дням живёт отдельно', () => {
    const { container } = render(<HoursChart points={points(90, 20)} neededMin={262} neededMax={350} />)
    expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('линия из ровно стольких точек, сколько суток', () => {
    render(<HoursChart points={points(90, 20)} neededMin={262} neededMax={350} />)
    const line = screen.getByTestId('hours-line')
    expect(line.getAttribute('points')!.trim().split(/\s+/)).toHaveLength(90)
  })

  it('подписи оси времени раз в 2 недели и подпись последнего значения', () => {
    render(<HoursChart points={points(90, 60)} neededMin={262} neededMax={350} />)
    expect(screen.getByText('28 июл')).toBeInTheDocument() // первые сутки
    expect(screen.getByText('11 авг')).toBeInTheDocument() // +14
    expect(screen.getByText('90 ч')).toBeInTheDocument() // итог: 90 суток по часу
  })

  it('полоса «нужно» рисуется, только когда укладывается в шкалу данных', () => {
    const { rerender } = render(<HoursChart points={points(90, 60)} neededMin={80} neededMax={95} />)
    expect(screen.getByTestId('needed-band')).toBeInTheDocument()
    rerender(<HoursChart points={points(90, 1)} neededMin={262} neededMax={350} />)
    expect(screen.queryByTestId('needed-band')).not.toBeInTheDocument()
  })

  it('пустой список и нулевые часы не роняют график', () => {
    const { container, rerender } = render(<HoursChart points={[]} neededMin={262} neededMax={350} />)
    expect(container).toBeEmptyDOMElement()
    rerender(<HoursChart points={points(90, 0)} neededMin={262} neededMax={350} />)
    expect(screen.getByTestId('hours-line')).toBeInTheDocument()
  })

  it('наведение показывает подсказку с датой и часами', () => {
    render(<HoursChart points={points(90, 60)} neededMin={262} neededMax={350} />)
    const surface = screen.getByTestId('hours-surface')
    surface.getBoundingClientRect = () => ({ left: 0, top: 0, width: 900, height: 300, right: 900, bottom: 300, x: 0, y: 0, toJSON: () => ({}) })
    fireEvent.pointerMove(surface, { clientX: 0, clientY: 100 })
    expect(screen.getByRole('tooltip', { hidden: true })).toHaveTextContent('28 июл')
    expect(screen.getByRole('tooltip', { hidden: true })).toHaveTextContent('1 ч')
    fireEvent.pointerLeave(surface)
    expect(screen.queryByRole('tooltip', { hidden: true })).not.toBeInTheDocument()
  })
})
