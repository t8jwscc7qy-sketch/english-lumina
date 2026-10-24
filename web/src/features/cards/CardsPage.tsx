import { useEffect } from 'react'
import { Link } from 'react-router'
import { Button } from '@/components/ui/button'
import { useCardQueue } from './cards'
import { ReviewCard } from './ReviewCard'

export function CardsPage() {
  const queue = useCardQueue()
  const card = queue.data?.card

  useEffect(() => {
    document.title = 'Карточки — Verst'
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h1 className="text-title">Карточки</h1>
        {queue.data && queue.data.dueCount > 0 && (
          <p aria-live="polite" className="text-sm text-muted-foreground tabular-nums">
            {`Осталось: ${queue.data.dueCount}`}
          </p>
        )}
      </header>

      {queue.isPending && <p role="status">Загружаем карточки…</p>}

      {queue.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {queue.error.message}
          </p>
          <Button onClick={() => queue.refetch()}>Повторить</Button>
        </div>
      )}

      {queue.data && !card && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <h2 className="text-title">На сегодня всё</h2>
          <p className="text-muted-foreground">{`Всего карточек: ${queue.data.totalCount}. Новые слова добавляются со страницы «Слова».`}</p>
          <Button asChild variant="secondary">
            <Link to="/words">Добавить слова</Link>
          </Button>
        </section>
      )}

      {card && <ReviewCard key={card.cardId} card={card} />}
    </div>
  )
}
