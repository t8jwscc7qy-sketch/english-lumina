import type { CSSProperties } from 'react'
import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { cn } from '@/lib/utils'
import { foxArt, type FoxMood } from './foxSprite'

export type { FoxMood }

const ART = {
  neutral: foxArt('neutral'),
  happy: foxArt('happy'),
  thinking: foxArt('thinking'),
  sleeping: foxArt('sleeping'),
} satisfies Record<FoxMood, ReturnType<typeof foxArt>>

interface FoxProps {
  /** Размер одной клетки в пикселях экрана (целое): лисёнок 28×25 клеток. */
  scale?: number
  mood?: FoxMood
  /** Покачивание на одну клетку двумя кадрами. */
  bob?: boolean
  className?: string
  /** Если задан, картинку читают скринридеры; иначе она декоративная. */
  title?: string
}

/** Маскот «Вёрст»: пиксельный лисёнок-путник с голубым шарфом. Четыре настроения, цвета из токенов `--fox*`. */
export function Fox({ scale = 6, mood = 'neutral', bob = true, className, title }: FoxProps) {
  return (
    <span
      data-mood={mood}
      className={cn('inline-block shrink-0', bob && 'fox-bob', className)}
      style={{ '--bob': `${scale}px` } as CSSProperties}
    >
      <PixelArtSvg art={ART[mood]} scale={scale} title={title} className="block" />
    </span>
  )
}
