import { useEffect, useId } from 'react'
import { Button } from '@/components/ui/button'
import type { components } from '@/api/schema.gen'
import { useSources } from './sources'

type Source = components['schemas']['Source']

const dateFormat = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
const numberFormat = new Intl.NumberFormat('ru-RU')

/** Ссылкой делаем только https-адрес: в базе может лежать что угодно, а javascript: в href опасен. */
function hostOf(url: string): string | null {
  try {
    const u = new URL(url)
    return u.protocol === 'https:' ? u.hostname : null
  } catch {
    return null
  }
}

function SourceCard({ source }: { source: Source }) {
  const titleId = useId()
  const host = hostOf(source.url)
  return (
    <article aria-labelledby={titleId} className="surface flex flex-col gap-4 p-6">
      <h2 id={titleId} className="flex flex-wrap items-baseline gap-x-3 text-title break-words">
        {source.name}
        <span className="text-sm font-semibold text-muted-foreground">версия {source.version}</span>
      </h2>
      <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-[8rem_minmax(0,1fr)]">
        <dt className="font-semibold text-muted-foreground">Лицензия</dt>
        <dd className="break-words">{source.license}</dd>
        <dt className="font-semibold text-muted-foreground">Автор данных</dt>
        <dd className="break-words">{source.attribution}</dd>
        <dt className="font-semibold text-muted-foreground">Файл</dt>
        <dd className="break-all">
          {host ? (
            <a href={source.url} target="_blank" rel="noreferrer noopener" className="underline underline-offset-4 hover:no-underline">
              {host}
            </a>
          ) : (
            source.url
          )}
        </dd>
        <dt className="font-semibold text-muted-foreground">Строк</dt>
        <dd className="tabular-nums">{numberFormat.format(source.rowCount)}</dd>
        <dt className="font-semibold text-muted-foreground">Загружено</dt>
        <dd>{dateFormat.format(new Date(source.importedAt))}</dd>
      </dl>
    </article>
  )
}

export function SourcesPage() {
  const sources = useSources()

  useEffect(() => {
    document.title = 'Источники и лицензии — Verst'
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Источники и лицензии</h1>
        <p className="text-muted-foreground">
          Откуда взяты слова и примеры. Лицензии требуют указывать, кому принадлежат данные.
        </p>
      </header>

      {sources.isPending && <p role="status">Загружаем источники…</p>}

      {sources.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {sources.error.message}
          </p>
          <Button onClick={() => sources.refetch()}>Повторить</Button>
        </div>
      )}

      {sources.data && sources.data.sources.length === 0 && (
        <p className="surface p-6 text-muted-foreground">
          Данные ещё не загружены. Загрузи их командой{' '}
          <code className="rounded-md bg-muted px-1.5 py-0.5 text-sm text-foreground">go run ./cmd/import cefrj</code>.
        </p>
      )}

      {sources.data && sources.data.sources.length > 0 && (
        <div className="flex flex-col gap-4">
          {sources.data.sources.map((s) => (
            <SourceCard key={`${s.name}@${s.version}`} source={s} />
          ))}
        </div>
      )}
    </div>
  )
}
