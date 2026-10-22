import { motion } from 'motion/react'
import { steps } from 'motion'
import { Mascot } from '@/components/brand/Wordmark'
import { Dither } from '@/components/pixel/Dither'
import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { LEVELS, type Forecast, type Level } from './levels'
import { flagArt } from './sprites'
import { cn } from '@/lib/utils'

interface StageMapProps {
  current: Level
  target: Level
  forecast: Forecast
}

type Stage = 'done' | 'current' | 'bridge' | 'future'

function stageOf(index: number, current: number, target: number): Stage {
  if (index < current) return 'done'
  if (index === current) return 'current'
  if (index <= target) return 'bridge'
  return 'future'
}

/**
 * Карта этапов: уровни A1…C1 как платформы. Пройденные зелёные, «ты здесь» голубая с маскотом,
 * путь до цели — точечный растр (честная неопределённость), остальное серое.
 */
export function StageMap({ current, target, forecast }: StageMapProps) {
  const cur = LEVELS.indexOf(current)
  const tgt = LEVELS.indexOf(target)

  return (
    <div className="flex flex-col gap-4">
      <ol aria-label="Путь по уровням" className="grid grid-cols-5">
        {LEVELS.map((level, i) => {
          const stage = stageOf(i, cur, tgt)
          const isTarget = i === tgt
          const status =
            stage === 'done' ? 'пройден' : stage === 'current' ? 'ты здесь' : isTarget ? 'цель' : ''
          return (
            <li
              key={level}
              aria-current={stage === 'current' ? 'step' : undefined}
              aria-label={status ? `${level}, ${status}` : level}
              className="flex min-w-0 flex-col items-center"
            >
              <div className="flex h-24 items-end justify-center">
                {stage === 'current' ? (
                  <motion.div
                    animate={{ y: [0, -4, 0] }}
                    transition={{ duration: 1.2, repeat: Infinity, ease: steps(2) }}
                  >
                    <Mascot scale={4} />
                  </motion.div>
                ) : (
                  <PixelArtSvg
                    art={flagArt(isTarget ? 'target' : stage === 'done' ? 'done' : 'future')}
                    scale={4}
                  />
                )}
              </div>

              {/* Платформа. Склеены встык: путь выглядит непрерывной дорогой. */}
              <div
                className={cn(
                  'relative h-6 w-full border-y-4 border-line',
                  // Торцы дороги: тонкие вертикальные заглушки на краях первой и последней платформы.
                  i === 0 &&
                    'before:absolute before:-top-1 before:-bottom-1 before:left-0 before:w-1 before:bg-line',
                  i === LEVELS.length - 1 &&
                    'after:absolute after:-top-1 after:-bottom-1 after:right-0 after:w-1 after:bg-line',
                  stage === 'done' && 'bg-grass',
                  stage === 'current' && 'bg-sky',
                  (stage === 'bridge' || stage === 'future') && 'bg-muted',
                )}
              >
                {stage === 'bridge' && (
                  <Dither level={2} color="var(--grass)" className="absolute inset-0 block" />
                )}
              </div>

              <p
                aria-hidden
                className={cn(
                  'mt-3 font-pixel text-base',
                  stage === 'future' && !isTarget && 'text-muted-foreground',
                )}
              >
                {level}
              </p>
              <p
                aria-hidden
                className={cn(
                  'h-5 text-sm max-md:hidden',
                  stage === 'current' ? 'font-semibold' : 'text-muted-foreground',
                )}
              >
                {stage === 'current'
                  ? 'Ты здесь'
                  : isTarget
                    ? 'Цель'
                    : stage === 'done'
                      ? 'Пройден'
                      : ''}
              </p>
            </li>
          )
        })}
      </ol>

      {forecast === 'collecting' && cur < tgt && (
        <p className="flex items-center gap-3 text-sm">
          <span className="relative block h-4 w-12 shrink-0 border-4 border-line bg-muted">
            <Dither level={2} color="var(--grass)" className="absolute inset-0 block" />
          </span>
          <span>
            <strong className="font-semibold">Собираю данные.</strong> Дату цели покажу после 1–2
            недель занятий: сначала нужно увидеть, как быстро ты растёшь.
          </span>
        </p>
      )}
    </div>
  )
}
