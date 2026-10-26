import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { type SheetSummary } from '@/api/client'
import { Button } from '@/components/ui/button'
import { filterSheets, pinned, useSheets } from './sheets'

function SheetCard({ s }: { s: SheetSummary }) {
  return (
    <li>
      <Link to={`/sheets/${s.id}`} className="block transition-[translate] duration-150 hover:-translate-y-0.5">
        <div className="surface flex h-full flex-col gap-2 p-4">
          <div className="flex items-start justify-between gap-2">
            <h3 className="font-sans font-semibold break-words">{s.title}</h3>
            <span className="text-xs text-muted-foreground">{s.level}</span>
          </div>
          <p className="text-sm text-muted-foreground">{s.summary}</p>
          <p className="mt-auto flex flex-wrap gap-x-3 text-xs text-muted-foreground">
            {s.favorite && <span>★ закреплено</span>}
            <span>{s.status === 'reviewed' ? '✓ проверено' : 'черновик'}</span>
            {s.weakCount > 0 && <span className="font-semibold text-coral-text">{`слабых: ${s.weakCount}`}</span>}
          </p>
        </div>
      </Link>
    </li>
  )
}

export function SheetsPage() {
  const sheets = useSheets()
  const [query, setQuery] = useState('')
  const [tag, setTag] = useState<string | null>(null)

  useEffect(() => {
    document.title = 'Шпаргалки — Verst'
  }, [])

  const all = sheets.data
  const tags = useMemo(() => [...new Set((all ?? []).flatMap((s) => s.tags))].sort(), [all])
  const found = useMemo(() => filterSheets(all ?? [], query, tag), [all, query, tag])
  const favorites = useMemo(() => pinned(found), [found])
  const rest = found.filter((s) => !s.favorite)

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Шпаргалки</h1>
        <p className="text-muted-foreground">Короткие таблицы и правила, которые открываются за пять секунд.</p>
      </header>

      {sheets.isPending && <p role="status">Загружаем шпаргалки…</p>}
      {sheets.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">{sheets.error.message}</p>
          <Button onClick={() => sheets.refetch()}>Повторить</Button>
        </div>
      )}

      {all && (
        <>
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-2">
              <span className="sr-only">Поиск шпаргалки</span>
              <input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Найти шпаргалку"
                className="h-10 w-64 border border-input bg-card px-3"
              />
            </label>
            <div role="group" aria-label="Теги" className="flex flex-wrap gap-2">
              {tags.map((t) => (
                <button
                  key={t}
                  type="button"
                  aria-pressed={tag === t}
                  onClick={() => setTag(tag === t ? null : t)}
                  className={`pixel-frame min-h-10 px-3 text-sm font-semibold ${tag === t ? 'bg-primary text-primary-foreground' : 'bg-card'}`}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>

          {found.length === 0 && (
            <p role="status" className="text-muted-foreground">
              Ничего не нашлось: поменяй поиск или тег.
            </p>
          )}

          {favorites.length > 0 && (
            <section aria-labelledby="fav-title" className="flex flex-col gap-3">
              <h2 id="fav-title" className="text-title">Закреплённое</h2>
              <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {favorites.map((s) => (
                  <SheetCard key={s.id} s={s} />
                ))}
              </ul>
            </section>
          )}

          {rest.length > 0 && (
            <section aria-labelledby="all-title" className="flex flex-col gap-3">
              <h2 id="all-title" className="text-title">{favorites.length > 0 ? 'Остальные' : 'Все шпаргалки'}</h2>
              <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {rest.map((s) => (
                  <SheetCard key={s.id} s={s} />
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  )
}
