import { Mascot } from '@/components/brand/Wordmark'
import { sortNew, type Milestone } from './milestones'

/**
 * «Вспышка успеха» в итоге дня: веха, достигнутая сегодня. Один раз при появлении, до 600 мс, ease-out
 * (`motion-safe:`: при «уменьшить движение» показывается сразу, без движения). Звука и уведомлений нет.
 */
export function MilestoneFlash({ milestones, ids }: { milestones: Milestone[]; ids: string[] }) {
  const fresh = sortNew(milestones, ids)
  if (fresh.length === 0) return null
  return (
    <section
      role="status"
      aria-live="polite"
      className="surface flex items-center gap-4 border-sun p-6 motion-safe:animate-[flash-in_600ms_cubic-bezier(0.23,1,0.32,1)_backwards]"
    >
      <Mascot scale={2} className="shrink-0" />
      <div className="flex min-w-0 flex-col gap-1">
        <p className="text-sm font-semibold text-muted-foreground">Новая веха</p>
        <ul className="flex flex-col gap-0.5">
          {fresh.map((m, i) => (
            <li
              key={m.id}
              {...(i === 0 ? { 'data-primary': 'true' } : {})}
              className={i === 0 ? 'text-title break-words' : 'font-semibold break-words'}
            >
              {m.title}
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
