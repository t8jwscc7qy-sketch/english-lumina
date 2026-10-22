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

/** Полоска опыта из квадратных сегментов (4px контур). Жёлтый — опыт и достижение. */
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
      className={cn('flex h-6 gap-1 border-4 border-line bg-card p-1', className)}
    >
      {Array.from({ length: segments }, (_, i) => (
        <span
          key={i}
          data-filled={i < filled}
          className={cn('h-full flex-1', i < filled ? 'bg-sun' : 'bg-muted')}
        />
      ))}
    </div>
  )
}
