import { useMemo } from 'react'
import { foxArt, type FoxMood } from '@/components/brand/foxSprite'
import { artToRects, type PixelArt } from '@/lib/pixel'
import { buildScene, FIRE_AT, FIRE_FRAMES, FIRE_PALETTE, FOX_AT, phaseOfHour, SCENE_H, SCENE_W } from './scene'

function Sprite({ art, x, y, className }: { art: PixelArt; x: number; y: number; className?: string }) {
  const rects = useMemo(() => artToRects(art), [art])
  return (
    <g transform={`translate(${x} ${y})`}>
      <g className={className}>
        {rects.map((r, i) => (
          <rect key={i} x={r.x} y={r.y} width={r.w} height={1} style={{ fill: r.fill }} />
        ))}
      </g>
    </g>
  )
}

/**
 * Лагерь у дороги: пиксельное небо по времени суток, холмы, сосны, дорога, лисёнок у костра.
 * Весь рисунок в одной сетке клеток, поэтому лисёнок и сцена одного «разрешения». Декоративный.
 */
export function CampScene({ hour, mood, className }: { hour: number; mood: FoxMood; className?: string }) {
  const phase = phaseOfHour(hour)
  const rects = useMemo(() => buildScene(phase), [phase])
  const fox = useMemo(() => foxArt(mood), [mood])
  const fireA = useMemo(() => ({ palette: FIRE_PALETTE, rows: FIRE_FRAMES[0] }), [])
  const fireB = useMemo(() => ({ palette: FIRE_PALETTE, rows: FIRE_FRAMES[1] }), [])
  return (
    <svg
      viewBox={`0 0 ${SCENE_W} ${SCENE_H}`}
      preserveAspectRatio="xMidYMax slice"
      shapeRendering="crispEdges"
      aria-hidden
      data-phase={phase}
      data-mood={mood}
      className={className}
    >
      {rects.map((r, i) => (
        <rect key={i} x={r.x} y={r.y} width={r.w} height={r.h} style={{ fill: r.fill }} opacity={r.opacity} />
      ))}
      <Sprite art={fireA} x={FIRE_AT.x} y={FIRE_AT.y} className="fire-a" />
      <Sprite art={fireB} x={FIRE_AT.x} y={FIRE_AT.y} className="fire-b" />
      <Sprite art={fox} x={FOX_AT.x} y={FOX_AT.y} className="scene-bob" />
    </svg>
  )
}
