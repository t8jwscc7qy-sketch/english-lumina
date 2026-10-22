import { useMemo } from 'react'
import { artToRects, type PixelArt } from '@/lib/pixel'

interface PixelArtSvgProps {
  art: PixelArt
  /** Размер одной клетки в пикселях экрана. Целое число, чтобы края оставались чёткими. */
  scale?: number
  /** Название для вспомогательных технологий; без него картинка считается декоративной. */
  title?: string
  className?: string
}

export function PixelArtSvg({ art, scale = 4, title, className }: PixelArtSvgProps) {
  const rects = useMemo(() => artToRects(art), [art])
  const cols = Math.max(0, ...art.rows.map((r) => r.length))
  const rows = art.rows.length
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox={`0 0 ${cols} ${rows}`}
      width={cols * scale}
      height={rows * scale}
      shapeRendering="crispEdges"
      className={className}
      {...(title ? { role: 'img', 'aria-label': title } : { 'aria-hidden': true })}
    >
      {rects.map((r, i) => (
        <rect key={i} x={r.x} y={r.y} width={r.w} height={1} fill={r.fill} />
      ))}
    </svg>
  )
}
