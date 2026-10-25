import { XpBar } from '@/components/pixel/XpBar'
import { formatDayRu } from '@/features/today/progress'
import { nextPerKind, type Milestone } from './milestones'

const fmt = (n: number) => n.toLocaleString('ru-RU', { maximumFractionDigits: 1 })

/** Блок «Вехи» на странице «Прогресс»: достигнутые с датой (новые сверху) и ближайшие с полосой. Без сигнальных цветов. */
export function MilestonesBlock({ milestones }: { milestones: Milestone[] }) {
  const reached = milestones
    .filter((m) => m.reachedOn)
    .sort((a, b) => (b.reachedOn ?? '').localeCompare(a.reachedOn ?? ''))
  const next = nextPerKind(milestones)
  return (
    <section aria-labelledby="milestones-title" className="surface flex flex-col gap-4 p-6">
      <h2 id="milestones-title" className="font-sans text-sm font-semibold text-muted-foreground">
        Вехи
      </h2>
      {reached.length > 0 ? (
        <ul aria-label="Достигнутые вехи" className="flex flex-col gap-1">
          {reached.map((m) => (
            <li key={m.id} className="font-semibold">
              {`${m.title} · ${formatDayRu(m.reachedOn ?? '')}`}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-muted-foreground">Первая веха ждёт тебя после занятий.</p>
      )}
      {next.length > 0 && (
        <ul aria-label="Ближайшие вехи" className="flex flex-col gap-3">
          {next.map((m) => (
            <li key={m.id} className="flex flex-col gap-1.5">
              {m.kind === 'first' ? (
                <p className="text-sm text-muted-foreground">{`${m.title}: ещё впереди`}</p>
              ) : (
                <>
                  <p className="text-sm tabular-nums">{`${m.title}: ${fmt(m.progress.value)} из ${fmt(m.progress.target)}`}</p>
                  <XpBar
                    value={m.progress.value}
                    total={m.progress.target}
                    label={m.title}
                    segments={10}
                  />
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
