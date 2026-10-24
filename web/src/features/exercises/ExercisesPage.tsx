import { useEffect } from 'react'
import { Link, Navigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { ExerciseCard } from './ExerciseCard'
import { useNextExercise, useTopics } from './exercises'
import { WeakTopics } from './WeakTopics'

export function ExercisesPage() {
  const next = useNextExercise()
  const topics = useTopics()
  const qc = useQueryClient()

  useEffect(() => {
    document.title = 'Упражнения — Verst'
  }, [])

  const unauthorized = next.error instanceof ApiError && next.error.code === 'unauthorized'
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])
  if (unauthorized) return <Navigate to="/login" replace />

  const noExercise = next.error instanceof ApiError && next.error.code === 'no_exercise'

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header>
        <h1 className="text-title">Упражнения</h1>
      </header>

      {next.isPending && <p role="status">Подбираем упражнение…</p>}

      {next.isError && noExercise && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <h2 className="text-title">Подходящих упражнений пока нет</h2>
          <p className="text-muted-foreground">Попробуй позже или поменяй уровень в настройках.</p>
          <Button asChild variant="secondary">
            <Link to="/settings">Настройки</Link>
          </Button>
        </section>
      )}

      {next.isError && !noExercise && (
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
