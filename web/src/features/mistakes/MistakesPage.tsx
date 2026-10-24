import { useEffect } from 'react'
import { Navigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { topicName } from '@/features/exercises/topics'
import { formatDue, useMistakes, type Mistake } from './mistakes'

const numberFormat = new Intl.NumberFormat('ru-RU')

function Count({ label, value }: { label: string; value: number }) {
  return (
    <div className="surface flex flex-col gap-1 p-4">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="text-2xl font-semibold tabular-nums">{numberFormat.format(value)}</dd>
    </div>
  )
}

/** Ошибки по темам, темы идут в порядке самой срочной ошибки (сервер уже отсортировал по сроку). */
function groupByTopic(list: Mistake[]): [string, Mistake[]][] {
  const map = new Map<string, Mistake[]>()
  for (const m of list) map.set(m.topic, [...(map.get(m.topic) ?? []), m])
  return [...map]
}

export function MistakesPage() {
  const mistakes = useMistakes()
  const qc = useQueryClient()
  // «Сейчас» — момент получения списка: так отрисовка не зависит от часов и срок не прыгает при перерисовке.
  const now = new Date(mistakes.dataUpdatedAt)

  useEffect(() => {
    document.title = 'Ошибки — Verst'
  }, [])

  const unauthorized = mistakes.error instanceof ApiError && mistakes.error.code === 'unauthorized'
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])
  if (unauthorized) return <Navigate to="/login" replace />

  const data = mistakes.data

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header>
        <h1 className="text-title">Ошибки</h1>
      </header>

      {mistakes.isPending && <p role="status">Загружаем ошибки…</p>}

      {mistakes.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {mistakes.error.message}
          </p>
          <Button onClick={() => mistakes.refetch()}>Повторить</Button>
        </div>
      )}

      {data && (
        <>
          <dl className="grid gap-3 sm:grid-cols-3">
            <Count label="Открыто" value={data.counts.open} />
            <Count label="Ждут повтора сейчас" value={data.counts.dueNow} />
            <Count label="Закрыто за неделю" value={data.counts.closedThisWeek} />
          </dl>

          {data.open.length === 0 ? (
            <p className="surface p-6 text-muted-foreground">
              Пока ошибок нет. Ошибки из упражнений появятся здесь и вернутся через день
            </p>
          ) : (
            groupByTopic(data.open).map(([topic, items]) => (
              <section key={topic} className="surface flex flex-col gap-2 p-6">
                <h2 className="text-sm font-semibold text-muted-foreground">{topicName(topic)}</h2>
                <ul className="flex flex-col divide-y divide-border">
                  {items.map((m) => (
                    <li key={m.id} className="flex flex-col gap-1 py-3 first:pt-1 last:pb-0">
                      <p lang={m.kind === 'order' ? 'ru' : 'en'} className="break-words text-lg">
                        {m.prompt}
                      </p>
                      <p className="break-words text-sm text-muted-foreground">
                        {'Твой ответ: '}
                        <span lang="en" className="text-foreground">{m.given}</span>
                      </p>
                      <p className="break-words text-sm text-muted-foreground">
                        {'Верно: '}
                        <span lang="en" className="font-semibold text-foreground">{m.answer}</span>
                      </p>
                      <p className="text-sm text-muted-foreground">
                        <span>{formatDue(m.nextDue, now)}</span>
                        {' · '}
                        <span className="tabular-nums">Серия {m.streak} из 2</span>
                      </p>
                    </li>
                  ))}
                </ul>
              </section>
            ))
          )}
        </>
      )}
    </div>
  )
}
