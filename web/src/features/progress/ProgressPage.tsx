import { useEffect, useState } from 'react'
import { Link, Navigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { XpBar } from '@/components/pixel/XpBar'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { useTopics } from '@/features/exercises/exercises'
import { reliabilityLabel, topicName } from '@/features/exercises/topics'
import { useSettings } from '@/features/settings/settings'
import { formatDayRu } from '@/features/today/progress'
import { describeDuration, neededHours } from '@/lib/forecast'
import { forecastFromPace, type PaceForecast } from '@/lib/progressForecast'
import { HoursChart } from './HoursChart'
import { formatFinishDate, formatHoursMinutes, formatRange, formatShortDay, useProgressOverview, type ProgressOverview } from './progress'

/** Меньше стольких ответов по теме — оценка ещё грубая. */
const FEW_ANSWERS = 5
const WEAK_COUNT = 5
const PACE_DAYS = 14

const WEEKS = { one: 'неделю', few: 'недели', many: 'недель' }
function weeksWord(n: number): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return WEEKS.one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return WEEKS.few
  return WEEKS.many
}

export function ProgressPage() {
  const overview = useProgressOverview()
  const settings = useSettings()
  const topics = useTopics()
  const qc = useQueryClient()
  const [today] = useState(() => new Date())

  useEffect(() => {
    document.title = 'Прогресс — Verst'
  }, [])

  const unauthorized = overview.error instanceof ApiError && overview.error.code === 'unauthorized'
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])
  if (unauthorized) return <Navigate to="/login" replace />

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 p-6 xl:p-8">
      <header>
        <h1 className="text-title">Прогресс</h1>
      </header>

      {(overview.isPending || !settings.data) && !overview.isError && <p role="status">Считаем прогресс…</p>}

      {overview.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {overview.error.message}
          </p>
          <Button onClick={() => overview.refetch()}>Повторить</Button>
        </div>
      )}

      {overview.data && settings.data && (
        <Content
          o={overview.data}
          current={settings.data.currentLevel}
          target={settings.data.targetLevel}
          today={today}
        />
      )}

      <TopicMap topics={topics.data?.topics} />
    </div>
  )
}

function Content({ o, current, target, today }: { o: ProgressOverview; current: Parameters<typeof forecastFromPace>[0]; target: Parameters<typeof forecastFromPace>[1]; today: Date }) {
  const forecast = forecastFromPace(current, target, o.studiedMinutes, o.paceMinutesPerDay, o.paceReady, o.daysSinceStart, today)
  const needed = neededHours(current, target)
  const studiedHours = o.studiedMinutes / 60
  const hasData = o.daysSinceStart > 0

  return (
    <>
      <section aria-labelledby="forecast-title" className="surface flex flex-col gap-3 p-6">
        <h2 id="forecast-title" className="font-sans text-sm font-semibold text-muted-foreground">
          Прогноз
        </h2>
        <ForecastBody forecast={forecast} target={target} today={today} />
        <p className="text-sm text-muted-foreground">
          Считаются только минуты в сервисе; уровень «сейчас» выбран тобой, поэтому часы в сервисе вычитаются из нужных. Часы Cambridge рассчитаны на занятия с преподавателем, поправка 1,5–2 на самостоятельную учёбу — допущение.
        </p>
      </section>

      <section aria-labelledby="hours-title" className="surface flex flex-col gap-4 p-6">
        <h2 id="hours-title" className="font-sans text-sm font-semibold text-muted-foreground">
          Часы занятий
        </h2>
        {hasData ? (
          <HoursChart points={o.daily} neededMin={needed?.min ?? 0} neededMax={needed?.max ?? 0} />
        ) : (
          <p className="text-muted-foreground">Начни сессию, и здесь появится линия</p>
        )}
        {needed && (
          <div className="flex flex-col gap-2">
            <XpBar value={studiedHours} total={needed.max} label="Набрано часов" segments={20} />
            <p className="text-sm tabular-nums">{`Набрано ${formatHoursMinutes(o.studiedMinutes)} из ${formatRange(needed.min, needed.max)}`}</p>
          </div>
        )}
        {hasData && (
          <details className="text-sm">
            <summary className="cursor-pointer font-semibold">Данные по дням</summary>
            <table className="mt-2 w-full tabular-nums">
              <thead>
                <tr className="text-left text-muted-foreground">
                  <th scope="col" className="py-1 pr-4 font-semibold">День</th>
                  <th scope="col" className="py-1 pr-4 font-semibold">Минут за день</th>
                  <th scope="col" className="py-1 font-semibold">Часов всего</th>
                </tr>
              </thead>
              <tbody>
                {o.daily.map((p) => (
                  <tr key={p.day} className="border-t border-border">
                    <td className="py-1 pr-4">{formatShortDay(p.day)}</td>
                    <td className="py-1 pr-4">{p.minutes}</td>
                    <td className="py-1">{p.cumulativeHours}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </details>
        )}
      </section>

      <section aria-labelledby="pace-title" className="surface flex flex-col gap-2 p-6">
        <h2 id="pace-title" className="font-sans text-sm font-semibold text-muted-foreground">
          Темп
        </h2>
        <p className="tabular-nums">
          {o.paceReady ? `Средний темп за ${PACE_DAYS} дней: ${formatHoursMinutes(o.paceMinutesPerDay)} в день` : `Темп покажу после ${PACE_DAYS} суток занятий`}
        </p>
        <p>{o.startedOn ? `Занимаешься с ${formatDayRu(o.startedOn)}` : 'Занятий пока не было'}</p>
        <p className="tabular-nums">{`Набрано: ${formatHoursMinutes(o.studiedMinutes)}`}</p>
      </section>
    </>
  )
}

function ForecastBody({ forecast, target, today }: { forecast: PaceForecast; target: string; today: Date }) {
  switch (forecast.kind) {
    case 'reached':
      return (
        <p>
          Цель достигнута. <Link to="/settings" className="underline underline-offset-4">Выбери цель повыше</Link>
        </p>
      )
    case 'enough':
      return (
        <p>
          Набрано больше нужных часов: оцени уровень и подними цель.{' '}
          <Link to="/settings" className="underline underline-offset-4">Настройки</Link>
        </p>
      )
    case 'collecting':
      return (
        <div className="flex flex-col gap-2">
          <p className="text-title">Собираю данные</p>
          <p>Дату покажу после {PACE_DAYS} суток занятий: сначала нужно увидеть, как быстро ты растёшь.</p>
          <XpBar value={Math.min(forecast.daysSinceStart, PACE_DAYS)} total={PACE_DAYS} label="Сутки занятий" segments={PACE_DAYS} />
          <p className="text-sm tabular-nums">{`${forecast.daysSinceStart} из ${PACE_DAYS} суток`}</p>
        </div>
      )
    case 'idle':
      return (
        <div className="flex flex-col items-start gap-3">
          <p className="text-title">Сейчас занятий нет</p>
          <p>Назвать дату нельзя: за последние {PACE_DAYS} суток минут почти не набралось. Начни сессию, и прогноз появится.</p>
          <Button asChild>
            <Link to="/session">Начать сессию</Link>
          </Button>
        </div>
      )
    case 'estimate': {
      const when = `${formatFinishDate(forecast.finishBy.earliest, today)} – ${formatFinishDate(forecast.finishBy.latest, today)}`
      return (
        <div className="flex flex-col gap-2">
          <p className="text-title">
            До {target} примерно <span className="font-semibold">{describeDuration(forecast.days)}</span>
          </p>
          <p>{`к ${when}`}</p>
          {forecast.unrealistic && <p className="text-sm font-semibold text-coral-text">Такой темп вряд ли удержать: больше 4 часов в день.</p>}
          {forecast.saveWithTenMore && (
            <p>{`Что изменить: +10 минут в день сократят срок примерно на ${forecast.saveWithTenMore.weeks} ${weeksWord(forecast.saveWithTenMore.weeks)}`}</p>
          )}
        </div>
      )
    }
  }
}

function TopicMap({ topics }: { topics: { topic: string; rating: number; deviation: number; attempts: number }[] | undefined }) {
  if (!topics) return null
  const sorted = [...topics].sort((a, b) => a.rating - b.rating)
  const weak = new Set(sorted.filter((t) => t.attempts >= FEW_ANSWERS).slice(0, WEAK_COUNT).map((t) => t.topic))
  return (
    <section aria-labelledby="topics-title" className="surface flex flex-col gap-3 p-6">
      <h2 id="topics-title" className="font-sans text-sm font-semibold text-muted-foreground">
        Карта тем
      </h2>
      {sorted.length === 0 ? (
        <p className="text-muted-foreground">Пока нет данных — ответь на несколько упражнений.</p>
      ) : (
        <ul aria-label="Карта тем" className="flex flex-col divide-y divide-border">
          {sorted.map((t) => (
            <li key={t.topic} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5 py-2 first:pt-0 last:pb-0">
              <span className="font-semibold">{topicName(t.topic)}</span>
              <span className="text-sm tabular-nums text-muted-foreground">
                <span className="font-semibold text-foreground">{Math.round(t.rating)}</span>
                {t.attempts < FEW_ANSWERS ? ' · мало ответов' : ` · надёжность: ${reliabilityLabel(t.deviation)}`}
                {weak.has(t.topic) && ' · '}
                {weak.has(t.topic) && <span className="font-semibold text-foreground">слабая тема</span>}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
