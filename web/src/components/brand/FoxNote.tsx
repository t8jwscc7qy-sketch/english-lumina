import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { Fox, type FoxMood } from './Fox'

/** Лисёнок рядом с короткой фразой: пустые состояния и пояснения. Лисёнок декоративный, смысл несёт текст. */
export function FoxNote({
  mood = 'neutral',
  children,
  className,
}: {
  mood?: FoxMood
  children: ReactNode
  className?: string
}) {
  return (
    <div className={cn('flex items-center gap-4', className)}>
      <Fox scale={3} mood={mood} />
      <div className="flex min-w-0 flex-col gap-1">{children}</div>
    </div>
  )
}
