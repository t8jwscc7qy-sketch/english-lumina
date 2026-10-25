import { motion } from 'motion/react'
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
              <div className="flex h-28 items-end justify-center">
                {stage === 'current' ? (
                  <motion.div
                    animate={{ y: [0, -4, 0] }}
                    transition={{ duration: 2.4, repeat: Infinity, ease: 'easeInOut' }}
                  >
                    <Mascot scale={3} className="max-md:[&_svg]:h-auto max-md:[&_svg]:w-14" />
                  </motion.div>
                ) : (
                  <PixelArtSvg
                    art={flagArt(isTarget ? 'target' : stage === 'done' ? 'done' : 'future')}
                    scale={4}
                    className="max-md:h-auto max-md:w-[27px]"
                  />
                )}
              </div>

              {/* Платформа. Склеены встык: путь выглядит непрерывной дорогой. */}
              <div
                className={cn(
                  'relative h-3 w-full overflow-hidden',
                  
                  
                  stage === 'done' && 'bg-grass',
                  stage === 'current' && 'bg-sky',
                  (stage === 'bridge' || stage === 'future') && 'bg-muted',
                )}
              >
                {stage === 'bridge' && (
                  <Dither level={2} color="var(--bridge)" className="absolute inset-0 block" />
                )}
              </div>

              <p
                aria-hidden
                className={cn(
                  'mt-3 text-base font-semibold',
                  stage === 'future' && !isTarget && 'text-muted-foreground',
                )}
              >
                {level}
              </p>
              <p
                aria-hidden
                className={cn(
                  'h-5 text-sm',
                  stage === 'current'
                    ? 'font-semibold whitespace-nowrap max-md:text-xs'
                    : 'text-muted-foreground max-md:hidden',
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
          <span className="relative block h-3 w-12 shrink-0 overflow-hidden bg-muted">
            <Dither level={2} color="var(--bridge)" className="absolute inset-0 block" />
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
