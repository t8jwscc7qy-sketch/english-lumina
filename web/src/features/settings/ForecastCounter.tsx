import { describeDuration, forecastBackward, forecastForward, parseDay } from '@/lib/forecast'
import type { Level } from '@/features/today/levels'

const monthFormat = new Intl.DateTimeFormat('ru-RU', { month: 'long' })
const dayFormat = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

/** «апрель 2027»: месяц в именительном падеже, без «г.». */
function monthYear(d: Date): string {
  return `${monthFormat.format(d)} ${d.getFullYear()}`
}

/**
 * Живой счётчик срока. Показывает вилку, а не точную дату: оценка идёт по часам занятий, а не по твоему темпу.
 * `minutesPerDay` = null, когда минуты введены неверно.
 */
export function ForecastCounter({
  current,
  target,
  minutesPerDay,
  targetDate,
  now,
}: {
  current: Level | null
  target: Level | null
  minutesPerDay: number | null
  targetDate: string
  now: Date
}) {
  return (
    <aside aria-label="Прогноз срока" className="surface flex flex-col gap-3 p-5">
      <h2 className="text-sm font-semibold text-muted-foreground">Сколько это займёт</h2>
      <div role="status" aria-live="polite" className="flex flex-col gap-3">
        <Body {...{ current, target, minutesPerDay, targetDate, now }} />
      </div>
      <p className="text-sm text-muted-foreground">
        Оценка по часам занятий Cambridge English для учёбы без преподавателя. Когда появятся
        упражнения, пересчитаю по твоему настоящему темпу.
      </p>
    </aside>
  )
}

function Body({
  current,
  target,
  minutesPerDay,
  targetDate,
  now,
}: {
  current: Level | null
  target: Level | null
  minutesPerDay: number | null
  targetDate: string
  now: Date
}) {
  if (!current || !target) return <p>Выбери уровень — покажу, сколько это займёт.</p>
  if (minutesPerDay === null) return <p>Укажи от 5 до 240 минут в день — и я посчитаю срок.</p>

  const forward = forecastForward(current, target, minutesPerDay, now)
  if (forward.kind === 'reached') return <p>Цель совпадает с уровнем — выбери цель повыше.</p>

  const { earliest, latest } = forward.finishBy
  const from = monthYear(earliest)
  const to = monthYear(latest)
  const goal = targetDate ? parseDay(targetDate) : null
  const backward = goal ? forecastBackward(current, target, goal, now) : null

  return (
    <>
      <p className="text-title">{describeDuration(forward.days)}</p>
      <p>
        до {target} при {minutesPerDay}&nbsp;мин в день
      </p>
      <p className="text-sm text-muted-foreground">
        Финиш примерно {from === to ? from : `${from} — ${to}`}.
      </p>
      {goal && backward?.kind === 'past' && (
        <p className="text-sm font-semibold text-coral-text">Эта дата уже прошла.</p>
      )}
      {goal && backward?.kind === 'needed' && (
        <p className={backward.unrealistic ? 'text-sm font-semibold text-coral-text' : 'text-sm'}>
          {backward.unrealistic
            ? `К ${dayFormat.format(goal)} понадобится больше 4 часов в день — вряд ли получится. Сдвинь дату.`
            : `Чтобы успеть к ${dayFormat.format(goal)}, нужно ${Math.ceil(backward.minutesPerDay.min)}–${Math.ceil(backward.minutesPerDay.max)} мин в день.`}
        </p>
      )}
    </>
  )
}
