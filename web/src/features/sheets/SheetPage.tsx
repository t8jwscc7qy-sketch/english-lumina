import { useEffect } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError, type SheetBlock } from '@/api/client'
import { FoxNote } from '@/components/brand/FoxNote'
import { Button } from '@/components/ui/button'
import { RichText } from './richText'
import { SmartTable } from './SmartTable'
import { useReviewSheet, useSetFavorite, useSheet } from './sheets'

function Block({ block }: { block: SheetBlock }) {
  switch (block.type) {
    case 'text':
      return <p className="max-w-prose">{block.text}</p>
    case 'tip':
      return (
        <p className="surface max-w-prose px-4 py-3 font-semibold">
          <span className="text-muted-foreground">Запомни: </span>
          <RichText text={block.text ?? ''} />
        </p>
      )
    case 'table':
      return <SmartTable block={block} />
    case 'mistakes':
      return (
        <section aria-label="Частые ошибки" className="flex flex-col gap-3">
          <h2 className="text-title">Частые ошибки</h2>
          <ul className="flex flex-col gap-3">
            {(block.pairs ?? []).map((p, i) => (
              <li key={i} className="surface flex flex-col gap-1 p-4">
                <p lang="en">
                  <span className="text-coral-text">✗ </span>
                  <s>{p.wrong}</s>
                </p>
                <p lang="en" className="font-semibold">
                  <span aria-hidden="true">✓ </span>
                  {p.right}
                </p>
                <p className="text-sm text-muted-foreground">{p.note}</p>
              </li>
            ))}
          </ul>
        </section>
      )
    case 'examples':
      return (
        <section aria-label="Примеры" className="flex flex-col gap-3">
          <h2 className="text-title">Примеры</h2>
          <ul className="flex flex-col gap-3">
            {(block.examples ?? []).map((e, i) => (
              <li key={i} className="flex flex-col gap-0.5 border-l-2 border-input pl-4">
                <p lang="en">
                  <RichText text={e.en} />
                </p>
                <p className="text-muted-foreground">{e.ru}</p>
              </li>
            ))}
          </ul>
        </section>
      )
    default:
      return null
  }
}

export function SheetPage() {
  const id = useParams().id ?? ''
  const sheet = useSheet(id)
  const favorite = useSetFavorite()
  const review = useReviewSheet()
  const s = sheet.data

  useEffect(() => {
    document.title = s ? `${s.title} — Verst` : 'Шпаргалка — Verst'
  }, [s])

  const notFound = sheet.error instanceof ApiError && sheet.error.code === 'not_found'

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 p-6 xl:p-8">
      <p className="no-print">
        <Link to="/sheets" className="text-sm underline underline-offset-4">
          ← Все шпаргалки
        </Link>
      </p>

      {sheet.isPending && <p role="status">Открываем шпаргалку…</p>}
      {notFound && (
        <section className="surface p-6">
          <FoxNote mood="thinking">
            <h1 className="text-title">Такой шпаргалки нет</h1>
            <p className="text-muted-foreground">Выбери другую из списка.</p>
          </FoxNote>
        </section>
      )}
      {sheet.isError && !notFound && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">{sheet.error.message}</p>
          <Button onClick={() => sheet.refetch()}>Повторить</Button>
        </div>
      )}

      {s && (
        <>
          <header className="flex flex-col gap-3">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <h1 className="text-title break-words">{s.title}</h1>
              <div className="no-print flex flex-wrap gap-2">
                <Button
                  variant="secondary"
                  aria-pressed={s.favorite}
                  disabled={favorite.isPending}
                  onClick={() => favorite.mutate({ id: s.id, favorite: !s.favorite })}
                >
                  {s.favorite ? '★ Закреплено' : '☆ Закрепить'}
                </Button>
                <Button variant="secondary" onClick={() => window.print()}>
                  Печать
                </Button>
              </div>
            </div>
            <p className="text-muted-foreground">{s.summary}</p>
            {s.status === 'draft' ? (
              <div role="note" className="flex flex-wrap items-center gap-3 border border-dashed border-input bg-card px-3 py-2 text-sm">
                <span className="font-semibold">Черновик: проверь, прежде чем заучивать.</span>
                <Button
                  size="sm"
                  variant="secondary"
                  className="no-print"
                  disabled={review.isPending}
                  onClick={() => review.mutate(s.id)}
                >
                  Я проверил
                </Button>
              </div>
            ) : (
              <p role="note" className="w-fit border border-input bg-card px-3 py-1 text-sm font-semibold">
                ✓ Проверено
              </p>
            )}
            {s.sources.length > 0 && <p className="text-xs text-muted-foreground">{`Источники: ${s.sources.join('; ')}`}</p>}
            {(favorite.isError || review.isError) && (
              <p role="alert" className="text-sm text-coral-text">
                {(favorite.error ?? review.error)?.message}
              </p>
            )}
          </header>

          {s.blocks.map((b, i) => (
            <Block key={i} block={b} />
          ))}
        </>
      )}
    </div>
  )
}
