/** Пиксель-арт: цвета по символам и строки сетки. «.» — пустая клетка. */
export interface PixelArt {
  palette: Record<string, string>
  rows: string[]
}

export interface Rect {
  x: number
  y: number
  w: number
  fill: string
}

/** Превращает картинку в прямоугольники высотой в одну клетку, склеивая соседей одного цвета. */
export function artToRects(art: PixelArt): Rect[] {
  const rects: Rect[] = []
  art.rows.forEach((row, y) => {
    let x = 0
    while (x < row.length) {
      const ch = row[x]
      if (ch === '.') {
        x++
        continue
      }
      const fill = art.palette[ch]
      if (fill === undefined) throw new Error(`Символ «${ch}» не описан в палитре`)
      let end = x + 1
      while (end < row.length && row[end] === ch) end++
      rects.push({ x, y, w: end - x, fill })
      x = end
    }
  })
  return rects
}

export type DitherLevel = 0 | 1 | 2 | 3 | 4

/** Закрашенные клетки плитки 2×2 для ступени шкалы. Ступень 2 — шахматка. */
export function ditherCells(level: DitherLevel): [number, number][] {
  const order: [number, number][] = [
    [0, 0],
    [1, 1],
    [1, 0],
    [0, 1],
  ]
  return order.slice(0, level)
}

/** Доля освоения 0…1 → ступень шкалы 0…4. Значения вне диапазона обрезаются, NaN считается нулём. */
export function masteryLevel(score: number): DitherLevel {
  if (!Number.isFinite(score)) return 0
  const clamped = Math.min(1, Math.max(0, score))
  return Math.min(4, Math.floor(clamped * 4 + 1e-9)) as DitherLevel
}

/** Сколько сегментов полоски опыта закрашено: value из total при count сегментах. */
export function filledSegments(value: number, total: number, count: number): number {
  if (!(total > 0) || !Number.isFinite(value)) return 0
  const share = Math.min(1, Math.max(0, value / total))
  return Math.min(count, Math.floor(share * count + 1e-9))
}
