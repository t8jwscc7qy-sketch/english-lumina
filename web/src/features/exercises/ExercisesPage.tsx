import { FoxNote } from '@/components/brand/FoxNote'
import { useEffect } from 'react'
import { Link, Navigate, useSearchParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { ExerciseCard } from './ExerciseCard'
import { useNextExercise, useTopics } from './exercises'
import { WeakTopics } from './WeakTopics'

export function ExercisesPage() {
  const camp = useSearchParams()[0].get('mode') === 'camp'
  const next = useNextExercise(camp ? 'camp' : undefined)
  const topics = useTopics()
  const qc = useQueryClient()

  useEffect(() => {
    document.title = camp ? 'Упражнения Привала — Verst' : 'Упражнения — Verst'
  }, [camp])

  const unauthorized = next.error instanceof ApiError && next.error.code === 'unauthorized'
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])
  if (unauthorized) return <Navigate to="/login" replace />

  const noExercise = next.error instanceof ApiError && next.error.code === 'no_exercise'
  const campClosed = next.error instanceof ApiError && next.error.code === 'camp_closed'

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header>
        <h1 className="text-title">{camp ? 'Упражнения Привала' : 'Упражнения'}</h1>
        {camp && <p className="text-muted-foreground">Только предложения из слов, которые ты уже изучил.</p>}
      </header>

      {next.isPending && <p role="status">Подбираем упражнение…</p>}

      {next.isError && campClosed && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <FoxNote mood="thinking">
            <h2 className="text-title">Привал пока закрыт</h2>
            <p className="text-muted-foreground">{next.error.message}</p>
          </FoxNote>
          <Button asChild variant="secondary">
            <Link to="/camp">К Привалу</Link>
          </Button>
        </section>
      )}

      {next.isError && noExercise && camp && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <FoxNote mood="thinking">
            <h2 className="text-title">Подходящих упражнений пока нет</h2>
            <p className="text-muted-foreground">Добавь слов: чем больше изучено, тем больше предложений подойдёт.</p>
          </FoxNote>
          <div className="flex flex-wrap gap-3">
            <Button onClick={() => next.refetch()}>Попробовать ещё</Button>
            <Button asChild variant="secondary">
              <Link to="/learn">Знакомство</Link>
            </Button>
          </div>
        </section>
      )}

      {next.isError && noExercise && !camp && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <FoxNote mood="thinking">
            <h2 className="text-title">Подходящих упражнений пока нет</h2>
            <p className="text-muted-foreground">Попробуй позже или поменяй уровень в настройках.</p>
          </FoxNote>
          <Button asChild variant="secondary">
            <Link to="/settings">Настройки</Link>
          </Button>
        </section>
      )}

      {next.isError && !noExercise && !campClosed && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {next.error.message}
          </p>
          <Button onClick={() => next.refetch()}>Повторить</Button>
        </div>
      )}

      {!next.isError && next.data && (
        <ExerciseCard key={`${next.data.id}-${next.dataUpdatedAt}`} exercise={next.data} onNext={() => void next.refetch()} loadingNext={next.isFetching} />
      )}

      {topics.data && <WeakTopics topics={topics.data.topics} />}
    </div>
  )
}
