import { CircleAlert, Layers, Pencil } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { XpBar } from '@/components/pixel/XpBar'
import { Fox, type FoxMood } from '@/components/brand/Fox'
import { Button } from '@/components/ui/button'
import { useMe } from '@/features/auth/auth'
import { useCardQueue } from '@/features/cards/cards'
import { useSettings } from '@/features/settings/settings'
import { describeDuration, forecastForward } from '@/lib/forecast'
import { DEMO_TODAY } from './demo'
import { formatDayRu, formatMinutes, sessionButtonLabel, streakWord, useProgressToday } from './progress'
import { StageMap } from './StageMap'

/** Карточка задачи дня: ссылка на свой раздел. Целиком кликабельна, при наведении «приподнимается». */
function TaskCard({
  to,
  title,
  value,
  hint,
  icon,
  tone = 'default',
}: {
  to: string
  title: string
  value: number | string
  hint: string
  icon: ReactNode
  tone?: 'default' | 'error'
}) {
  return (
    <Link
      to={to}
      className="block rounded-xl transition-[translate,box-shadow] duration-150 hover:-translate-y-0.5 hover:[--shadow-card:var(--shadow-card-hover)] active:translate-y-0"
    >
      {/* div, а не article: у ссылки с article внутри пустое доступное имя (контент article в него не попадает) */}
      <div className="surface flex h-full flex-col gap-2 p-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
          <span className={tone === 'error' ? 'text-coral-text' : undefined}>{icon}</span>
          <h3 className="font-sans text-sm font-semibold">{title}</h3>
        </div>
        <p className="text-display tabular-nums">{value}</p>
        <p className="text-sm text-muted-foreground">{hint}</p>
      </div>
    </Link>
  )
}

/** Настроение лисёнка и его реплика по состоянию дня. Ничего не выдумывает: только факты из прогресса и часа суток. */
function foxState(
  p: ReturnType<typeof useProgressToday>['data'],
  now: Date,
  minimum: number,
): { mood: FoxMood; line: string | null } {
  if (!p) return { mood: 'neutral', line: null }
  if (p.doneToday) return { mood: 'happy', line: 'Минимум на сегодня выполнен. Дальше по желанию.' }
  if (now.getHours() < 5) return { mood: 'sleeping', line: 'Уже поздно. Можно чуть позаниматься или лечь спать.' }
  if (p.minutes === 0 && p.streakDays === 0) {
    return { mood: 'thinking', line: `Давай начнём с малого: хватит и ${minimum} минут.` }
  }
  const left = Math.max(0, minimum - p.minutes)
  return { mood: 'neutral', line: `До минимума на сегодня ещё ${formatMinutes(left)}\u00a0мин.` }
}

export function TodayPage() {
  const me = useMe()
  const settings = useSettings()
  const cardQueue = useCardQueue()
  const progress = useProgressToday()
  const [now] = useState(() => new Date())
  const d = DEMO_TODAY

  useEffect(() => {
    document.title = 'Сегодня — Verst'
  }, [])

  // Под защитником маршрутов настройки уже в кэше; это окно только в отдельных тестах и при сбое.
  if (!settings.data) return <p role="status" className="p-6">Загружаем…</p>
  const { currentLevel: current, targetLevel: target, dailyMinutes } = settings.data
  const forecast = forecastForward(current, target, dailyMinutes, now)

  const p = progress.data
  const streakDays = p?.streakDays ?? 0
  const minutes = p?.minutes ?? 0
  const minimum = p?.minimumDay ?? 10
  const { mood, line } = foxState(p, now, minimum)

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6 p-6 xl:p-8">
      <section
        aria-label="Главное на сегодня"
        className="surface relative grid grid-cols-[minmax(0,1fr)] items-center gap-6 overflow-hidden p-6 md:grid-cols-[auto_minmax(0,1fr)] xl:grid-cols-[auto_minmax(0,1fr)_19rem] xl:gap-8 xl:p-8"
      >
        {/* Мягкое сияние за лисёнком: цвета токенов, без декоративных фигур. */}
        <div
          aria-hidden
          className="pointer-events-none absolute -left-10 -top-16 size-72 rounded-full opacity-70 blur-2xl"
          style={{
            background:
              'radial-gradient(closest-side, color-mix(in srgb, var(--sun) 38%, transparent), color-mix(in srgb, var(--sky) 16%, transparent) 60%, transparent)',
          }}
        />
        <div className="relative mx-auto md:mx-0">
          <Fox mood={mood} size={168} className="max-md:size-32" />
        </div>

        <header className="relative flex min-w-0 flex-col gap-3">
          <h1 className="text-title break-words">Привет, {me.data?.login}</h1>
          {line && (
            <p className="relative w-fit max-w-prose rounded-2xl rounded-tl-sm bg-secondary px-4 py-2 font-semibold">
              {line}
            </p>
          )}
          <p className="text-muted-foreground">
            Цель: {current} → {target} · {dailyMinutes}&nbsp;мин в день
          </p>
          <p>
            {forecast.kind === 'reached' ? (
              <>
                Цель достигнута. <Link to="/settings" className="underline underline-offset-4">Выбери цель повыше</Link>
              </>
            ) : (
              <>
                До {target} примерно{' '}
                <span className="font-semibold">{describeDuration(forecast.days)}</span>
              </>
            )}
          </p>
          {/* Честная пометка. Жёлтый в этом мире значит «опыт и главное действие», поэтому рамка пунктирная нейтральная. */}
          <p
            role="note"
            className="w-fit rounded-lg border border-dashed border-input bg-card px-3 py-1 text-sm font-semibold"
          >
            Демо-данные: путь, опыт уровня и числа в задачах на упражнения и ошибки пока демонстрационные.
          </p>
        </header>

        <aside
          aria-label="Серия и сессия"
          className="relative flex flex-col gap-4 md:col-span-2 xl:col-span-1"
        >
          <section className="flex flex-col gap-1">
            <h2 className="font-sans text-sm font-semibold text-muted-foreground">Серия</h2>
            <p className="text-display">
              {streakDays} {streakWord(streakDays)}
            </p>
            {p?.freezeUsedOn && (
              <p className="text-sm text-muted-foreground">{`Заморозка спасла ${formatDayRu(p.freezeUsedOn)}`}</p>
            )}
          </section>

          <section className="flex flex-col gap-2">
            <h2 className="font-sans text-sm font-semibold text-muted-foreground">
              Минимальный день
            </h2>
            <XpBar value={minutes} total={minimum} label="Минимальный день" segments={10} />
            <p className="text-sm tabular-nums">
              {`${formatMinutes(minutes)}\u00a0из\u00a0${minimum}\u00a0мин`}
            </p>
          </section>

          <div className="flex flex-col gap-2">
            <Button asChild size="lg">
              <Link to="/session">{sessionButtonLabel(p)}</Link>
            </Button>
            {progress.isError && (
              <div className="flex flex-wrap items-center gap-2">
                <p role="alert" className="text-sm text-coral-text">
                  {progress.error.message}
                </p>
                <Button size="sm" variant="secondary" onClick={() => progress.refetch()}>
                  Повторить
                </Button>
              </div>
            )}
          </div>
        </aside>
      </section>

      <section
        aria-labelledby="path-title"
        className="surface flex flex-col gap-6 p-6 xl:p-8"
      >
        <h2 id="path-title" className="text-title">
          Твой путь
        </h2>
        <StageMap current={current} target={target} forecast="collecting" />
        <div className="flex flex-col gap-2">
          <p className="flex justify-between text-sm tabular-nums">
            <span className="font-semibold">Опыт уровня {current}</span>
            <span className="text-muted-foreground">
              {d.levelXp.value}&nbsp;из&nbsp;{d.levelXp.total}
            </span>
          </p>
          <XpBar
            value={d.levelXp.value}
            total={d.levelXp.total}
            label={`Опыт уровня ${current}`}
            segments={20}
          />
        </div>
      </section>

      <section aria-labelledby="tasks-title" className="grid gap-4 md:grid-cols-3">
        <h2 id="tasks-title" className="sr-only">
          Задачи на сегодня
        </h2>
        <TaskCard
          to="/cards"
          title="Карточки к повторению"
          value={cardQueue.data ? cardQueue.data.dueCount : '—'}
          hint="Слова, которые пора вспомнить"
          icon={<Layers className="size-5" strokeWidth={1.75} aria-hidden />}
        />
        <TaskCard
          to="/exercises"
          title="Упражнения на слабые темы"
          value={d.exercises.count}
          hint={`Тема: ${d.exercises.topic}`}
          icon={<Pencil className="size-5" strokeWidth={1.75} aria-hidden />}
        />
        <TaskCard
          to="/mistakes"
          title="Работа над ошибками"
          value={d.mistakes}
          hint="Вернутся, пока не исчезнут"
          tone="error"
          icon={<CircleAlert className="size-5" strokeWidth={1.75} aria-hidden />}
        />
      </section>
    </div>
  )
}
