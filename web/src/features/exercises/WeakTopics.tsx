import type { Rating } from './exercises'
import { reliabilityLabel, topicName, weakestTopics } from './topics'

/** Меньше стольких ответов по теме — оценка ещё грубая. */
const FEW_ANSWERS = 5

/** Пять тем с наименьшим рейтингом. Надёжность оценки — простыми словами, без терминов рейтинговой системы. */
export function WeakTopics({ topics }: { topics: Rating[] }) {
  const weak = weakestTopics(topics)
  return (
    <section aria-labelledby="weak-topics" className="surface flex flex-col gap-3 p-4">
      <h2 id="weak-topics" className="text-sm font-semibold">
        Слабые темы
      </h2>
      {weak.length === 0 ? (
        <p className="text-sm text-muted-foreground">Пока нет данных — ответь на несколько упражнений.</p>
      ) : (
        <ul className="flex flex-col divide-y divide-border">
          {weak.map((t) => (
            <li key={t.topic} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5 py-2 first:pt-0 last:pb-0">
              <span className="font-semibold">{topicName(t.topic)}</span>
              <span className="text-sm text-muted-foreground tabular-nums">
                <span className="font-semibold text-foreground">{Math.round(t.rating)}</span>
                {t.attempts < FEW_ANSWERS ? ' · мало ответов' : ` · надёжность: ${reliabilityLabel(t.deviation)}`}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
