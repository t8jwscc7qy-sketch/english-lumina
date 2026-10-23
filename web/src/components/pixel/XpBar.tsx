import { cn } from '@/lib/utils'
import { filledSegments } from '@/lib/pixel'

interface XpBarProps {
  value: number
  total: number
  /** Подпись для вспомогательных технологий (её же можно показать рядом). */
  label: string
  segments?: number
  className?: string
}

/** Полоска опыта из тонких скруглённых сегментов. Жёлтый — опыт и достижение. */
export function XpBar({ value, total, label, segments = 10, className }: XpBarProps) {
  const filled = filledSegments(value, total, segments)
  const now = total > 0 ? Math.min(total, Math.max(0, value)) : 0
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={total}
      aria-valuenow={now}
      className={cn('flex h-3 gap-0.5 overflow-hidden rounded-full bg-muted p-0.5', className)}
    >
      {Array.from({ length: segments }, (_, i) => (
        <span
          key={i}
          data-filled={i < filled}
          className={cn('h-full flex-1 rounded-full', i < filled ? 'bg-sun' : 'bg-background/70')}
        />
      ))}
    </div>
  )
}
