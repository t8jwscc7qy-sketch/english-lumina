import type { CSSProperties } from 'react'
import { ditherCells, type DitherLevel } from '@/lib/pixel'

const TILE = 8 // плитка 8×8 из четырёх клеток 4×4

function maskFor(level: DitherLevel): string {
  const rects = ditherCells(level)
    .map(([x, y]) => `<rect x='${x * 4}' y='${y * 4}' width='4' height='4'/>`)
    .join('')
  const svg = `<svg xmlns='http://www.w3.org/2000/svg' width='${TILE}' height='${TILE}' shape-rendering='crispEdges'>${rects}</svg>`
  return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`
}

interface DitherProps {
  /** Ступень 0…4: плотность точечного растра. 0 — пусто, 4 — сплошная заливка. */
  level: DitherLevel
  /** CSS-цвет растра, по умолчанию текущий цвет текста. */
  color?: string
  className?: string
}

/** Точечный растр (дизер) вместо плоской заливки: так в Verst рисуются неопределённость и сила темы. */
export function Dither({ level, color = 'currentColor', className }: DitherProps) {
  const mask = maskFor(level)
  const style: CSSProperties = {
    backgroundColor: color,
    maskImage: mask,
    WebkitMaskImage: mask,
    maskSize: `${TILE}px ${TILE}px`,
    WebkitMaskSize: `${TILE}px ${TILE}px`,
    imageRendering: 'pixelated',
  }
  return <span aria-hidden className={className} style={style} data-dither-level={level} />
}
