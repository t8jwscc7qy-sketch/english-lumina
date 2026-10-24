import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, Navigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { XpBar } from '@/components/pixel/XpBar'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { useCardQueue } from '@/features/cards/cards'
import { ReviewCard } from '@/features/cards/ReviewCard'
import { ExerciseCard } from '@/features/exercises/ExerciseCard'
import { useNextExercise } from '@/features/exercises/exercises'
import { formatDayRu, formatMinutes, progressKey, streakWord, useProgressToday, type ProgressToday } from '@/features/today/progress'

/** Не больше стольких карточек за сессию: очередь в сотню карточек не должна съедать весь день. */
export const MAX_CARDS_PER_SESSION = 10

type Phase = 'run' | 'minimum' | 'summary'

function plural(n: number, one: string, few: string, many: string): string {
  const mod100 = n % 100
  if (mod100 >= 11 && mod100 <= 14) return many
  const mod10 = n % 10
  if (mod10 === 1) return one
  if (mod10 >= 2 && mod10 <= 4) return few
  return many
}

const isUnauthorized = (e: unknown) => e instanceof ApiError && e.code === 'unauthorized'

export function SessionPage() {
  const qc = useQueryClient()
  const queue = useCardQueue()
  const progress = useProgressToday()
  const [phase, setPhase] = useState<Phase>('run')
  const [cardsDone, setCardsDone] = useState(0)
  const [extra, setExtra] = useState(false) // «Ещё немного»: экран минимума больше не показываем
  // Шаг карточек завершается, когда показано первое упражнение, и больше не возвращается сам: карточка, снова ставшая
  // готовой во время упражнений (короткий шаг обучения, «Добавить в карточки»), не должна прятать результат ответа.
  const [cardsOver, setCardsOver] = useState(false)
  const [entryChecked, setEntryChecked] = useState(false)
  const headingRef = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    document.title = 'Сессия — Verst'
  }, [])

  useEffect(() => {
    if (phase !== 'run') headingRef.current?.focus() // на экранах итога и минимума фокус идёт на заголовок
  }, [phase])

  // Минимум уже выполнен при входе («Позаниматься ещё»): экран «Минимум выполнен» не нужен, он только что был.
  // Корректировка состояния при рендере (однократно, по первым данным), а не эффект: так нет лишней перерисовки.
  if (!entryChecked && progress.data) {
    setEntryChecked(true)
    if (progress.data.doneToday) setExtra(true)
  }

  const unauthorized = isUnauthorized(queue.error) || isUnauthorized(progress.error)
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])

  if (!cardsOver && phase === 'run' && queue.data && (!queue.data.card || cardsDone >= MAX_CARDS_PER_SESSION)) setCardsOver(true)

  const refreshProgress = useCallback(() => void qc.invalidateQueries({ queryKey: progressKey }), [qc])
  const onRated = useCallback(() => {
    setCardsDone((n) => n + 1)
    refreshProgress()
  }, [refreshProgress])
  const resume = useCallback(() => {
    setExtra(true)
    setPhase('run')
  }, [])

  if (unauthorized) return <Navigate to="/login" replace />

  const card = queue.data?.card
  const dueCount = queue.data?.dueCount ?? 0
  const cardsStep = phase === 'run' && !cardsOver && !!card && cardsDone < MAX_CARDS_PER_SESSION
  const exercisesStep = phase === 'run' && !!queue.data && !cardsStep
  const cardsTotal = cardsDone + Math.min(dueCount, MAX_CARDS_PER_SESSION - cardsDone)
  const p = progress.data
  const minutes = p?.minutes ?? 0
  const minimum = p?.minimumDay ?? 10

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-title">Сессия</h1>
          {phase === 'run' && (
            <Button variant="secondary" size="sm" onClick={() => setPhase('summary')}>
              Закончить
            </Button>
          )}
        </div>
        <div className="flex flex-col gap-2">
          <XpBar value={minutes} total={minimum} label="Минимальный день" segments={10} />
          <p className="text-sm tabular-nums text-muted-foreground">{`${formatMinutes(minutes)} из ${minimum} мин`}</p>
        </div>
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
      </header>

      {queue.isPending && <p role="status">Загружаем сессию…</p>}

      {queue.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {queue.error.message}
          </p>
          <Button onClick={() => queue.refetch()}>Повторить</Button>
        </div>
      )}

      {cardsStep && card && (
        <>
          <p aria-live="polite" className="text-sm font-semibold tabular-nums">{`Карточка ${cardsDone + 1} из ${cardsTotal}`}</p>
          <ReviewCard key={card.cardId} card={card} onRated={onRated} />
        </>
      )}

      {exercisesStep && (
        <ExercisesStep
          extra={extra}
          refetchProgress={() => progress.refetch()}
          onAnswered={refreshProgress}
          onMinimum={() => setPhase('minimum')}
          onFinish={() => setPhase('summary')}
        />
      )}

      {phase === 'minimum' && (
        <section className="surface flex flex-col items-start gap-4 p-6">
          <h2 ref={headingRef} tabIndex={-1} className="text-title outline-none">
            Минимум на сегодня выполнен
          </h2>
          <p className="text-muted-foreground">Можно закончить или позаниматься ещё: серия уже засчитана.</p>
          <div className="flex flex-wrap gap-3">
            <Button size="lg" onClick={() => setPhase('summary')}>
              Закончить
            </Button>
            <Button size="lg" variant="secondary" onClick={resume}>
              Ещё немного
            </Button>
          </div>
        </section>
      )}

      {phase === 'summary' && <Summary progress={p} headingRef={headingRef} onResume={resume} />}
    </div>
  )
}

function ExercisesStep({
  extra,
  refetchProgress,
  onAnswered,
  onMinimum,
  onFinish,
}: {
  extra: boolean
  refetchProgress: () => Promise<{ data?: ProgressToday }>
  onAnswered: () => void
  onMinimum: () => void
  onFinish: () => void
}) {
  const next = useNextExercise()
  const qc = useQueryClient()
  const [advancing, setAdvancing] = useState(false)

  const unauthorized = isUnauthorized(next.error)
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])
  if (unauthorized) return <Navigate to="/login" replace />

  const noExercise = next.error instanceof ApiError && next.error.code === 'no_exercise'

  // После «Дальше»: если минимум уже выполнен и пользователь не просил «Ещё немного», показываем экран минимума.
  async function advance() {
    setAdvancing(true)
    try {
      const r = await refetchProgress()
      if (!extra && r.data?.doneToday) {
        onMinimum()
        return
      }
      await next.refetch()
    } finally {
      setAdvancing(false)
    }
  }

  if (next.isPending) return <p role="status">Подбираем упражнение…</p>

  if (next.isError && noExercise) {
    return (
      <section className="surface flex flex-col items-start gap-3 p-6">
        <h2 className="text-title">Подходящих упражнений пока нет</h2>
        <p className="text-muted-foreground">Попробуй позже или поменяй уровень в настройках.</p>
        <div className="flex flex-wrap gap-3">
          <Button asChild>
            <Link to="/">На главную</Link>
          </Button>
          <Button variant="secondary" onClick={onFinish}>
            Закончить
          </Button>
        </div>
      </section>
    )
  }

  if (next.isError) {
    return (
      <div className="surface flex flex-col items-start gap-4 p-6">
        <p role="alert" className="text-muted-foreground">
          {next.error.message}
        </p>
        <Button onClick={() => next.refetch()}>Повторить</Button>
      </div>
    )
  }

  return (
    <ExerciseCard
      key={`${next.data.id}-${next.dataUpdatedAt}`}
      exercise={next.data}
      onNext={() => void advance()}
      loadingNext={advancing || next.isFetching}
      onAnswered={onAnswered}
    />
  )
}

function Summary({
  progress,
  headingRef,
  onResume,
}: {
  progress: ProgressToday | undefined
  headingRef: React.RefObject<HTMLHeadingElement | null>
  onResume: () => void
}) {
  const t = progress?.today
  return (
    <section aria-labelledby="summary-title" className="surface flex flex-col gap-4 p-6">
      <h2 id="summary-title" ref={headingRef} tabIndex={-1} className="text-title outline-none">
        Итог дня
      </h2>
      {progress ? (
        <ul className="flex flex-col gap-2">
          <li>{`Минут сегодня: ${formatMinutes(progress.minutes)} из ${progress.minimumDay}`}</li>
          {t && t.answers > 0 && (
            <li>{`${t.answers} ${plural(t.answers, 'ответ', 'ответа', 'ответов')}, ${t.correct} ${plural(t.correct, 'верный', 'верных', 'верных')}`}</li>
          )}
          {t && t.cardsReviewed > 0 && <li>{`Карточек повторено: ${t.cardsReviewed}`}</li>}
          {t && (t.mistakesOpened > 0 || t.mistakesClosed > 0) && (
            <li>{`Ошибки: открыто ${t.mistakesOpened}, закрыто ${t.mistakesClosed}`}</li>
          )}
          <li>{`Серия: ${progress.streakDays} ${streakWord(progress.streakDays)}`}</li>
          {progress.freezeUsedOn && <li className="text-sm text-muted-foreground">{`Заморозка спасла ${formatDayRu(progress.freezeUsedOn)}`}</li>}
        </ul>
      ) : (
        <p className="text-muted-foreground">Прогресс за сегодня не загрузился, но все ответы сохранены.</p>
      )}
      <div className="flex flex-wrap gap-3">
        <Button asChild size="lg">
          <Link to="/">На главную</Link>
        </Button>
        <Button size="lg" variant="secondary" onClick={onResume}>
          Ещё немного
        </Button>
      </div>
    </section>
  )
}
