import { useMemo } from 'react'
import { foxArt, type FoxMood } from '@/components/brand/foxSprite'
import { artToRects, type PixelArt } from '@/lib/pixel'
import { buildScene, FIRE_BASE_Y, FIRE_CX, fireFrames, fireLevel, FOX_AT, logCount, phaseOfHour, SCENE_H, SCENE_W } from './scene'

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
export function CampScene({
  hour,
  mood,
  streakDays = 0,
  doneToday = false,
  milestonesReached = 0,
  hasNextMilestone = false,
  className,
}: {
  hour: number
  mood: FoxMood
  /** Серия: ступень костра и число поленьев. */
  streakDays?: number
  /** Минимум дня выполнен: огонь на ступень выше. */
  doneToday?: boolean
  /** Достигнуто вех: столбики на дороге. */
  milestonesReached?: number
  hasNextMilestone?: boolean
  className?: string
}) {
  const phase = phaseOfHour(hour)
  const level = fireLevel(streakDays, doneToday)
  const logs = logCount(streakDays)
  const rects = useMemo(
    () => buildScene(phase, { fireLevel: level, logs, posts: milestonesReached, nextPost: hasNextMilestone }),
    [phase, level, logs, milestonesReached, hasNextMilestone],
  )
  const fox = useMemo(() => foxArt(mood), [mood])
  const [fireA, fireB] = useMemo(() => fireFrames(level), [level])
  const fireX = FIRE_CX - Math.floor(fireA.rows[0].length / 2)
  const fireY = FIRE_BASE_Y - fireA.rows.length
  return (
    <svg
      viewBox={`0 0 ${SCENE_W} ${SCENE_H}`}
      preserveAspectRatio="xMidYMax slice"
      shapeRendering="crispEdges"
      aria-hidden
      data-phase={phase}
      data-mood={mood}
      data-fire={level}
      className={className}
    >
      {rects.map((r, i) => (
        <rect key={i} x={r.x} y={r.y} width={r.w} height={r.h} style={{ fill: r.fill }} opacity={r.opacity} />
      ))}
      <Sprite art={fireA} x={fireX} y={fireY} className="fire-a" />
      <Sprite art={fireB} x={fireX} y={fireY} className="fire-b" />
      <Sprite art={fox} x={FOX_AT.x} y={FOX_AT.y} className="scene-bob" />
    </svg>
  )
}
