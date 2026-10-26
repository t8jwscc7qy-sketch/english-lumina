import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import type { PlacementPage as Page, PlacementResult } from '@/api/client'
import { FoxNote } from '@/components/brand/FoxNote'
import { Button } from '@/components/ui/button'
import { newKey, nextSkip, usePlacementPage, useSubmitPlacement } from './learn'

const OUTCOME: Record<PlacementResult['next'], string> = {
  more: 'Много знакомых слов. Можно пройти ещё страницу.',
  learn: 'Часть слов новая. Самое время познакомиться с ними.',
  stop: 'Большинство слов тебе пока незнакомо: тест не нужен, переходи к знакомству.',
}

function Words({ skip, page, onDone }: { skip: number; page: Page; onDone: (r: PlacementResult, known: number) => void }) {
  const submit = useSubmitPlacement()
  const [key] = useState(() => newKey())
  const [known, setKnown] = useState<Set<number>>(new Set())

  function toggle(id: number) {
    setKnown((s) => {
      const next = new Set(s)
      if (!next.delete(id)) next.add(id)
      return next
    })
  }

  function send() {
    if (submit.isPending) return
    const shown = page.words.map((w) => w.lexemeId)
    const ids = shown.filter((id) => known.has(id))
    submit.mutate({ known: ids, shown, key }, { onSuccess: (r) => onDone(r, ids.length) })
  }

  return (
    <>
      <p className="text-sm text-muted-foreground">
        {`Отметь слова, которые точно знаешь (Пробел). Остальные не отмечай. Показано ${page.words.length}, пропущено ${skip}.`}
      </p>
      <ul className="surface grid divide-y divide-input p-0 md:grid-cols-2 md:divide-y-0">
        {page.words.map((w) => (
          <li key={w.lexemeId} className="border-input md:border-b">
            <label className="flex cursor-pointer items-center gap-3 px-4 py-3 has-[:focus-visible]:outline has-[:focus-visible]:outline-2">
              <input type="checkbox" checked={known.has(w.lexemeId)} onChange={() => toggle(w.lexemeId)} className="size-5 accent-primary" />
              <span lang="en" className="font-semibold break-words">{w.lemma}</span>
              <span className="text-sm text-muted-foreground">{w.pos}</span>
              <span className="sr-only">знаю</span>
            </label>
          </li>
        ))}
      </ul>
      <div className="flex flex-wrap items-center gap-3">
        <Button size="lg" disabled={submit.isPending} onClick={send}>
          {submit.isPending ? 'Сохраняем…' : `Готово (знаю: ${known.size})`}
        </Button>
      </div>
      {submit.isError && (
        <p role="alert" className="rounded-lg border border-coral bg-card p-3 text-sm font-semibold text-coral-text">
          {submit.error.message}
        </p>
      )}
    </>
  )
}

export function PlacementPage() {
  const [skip, setSkip] = useState(0)
  const [result, setResult] = useState<{ r: PlacementResult; known: number; shown: number } | null>(null)
  const page = usePlacementPage(skip)
  const resultRef = useRef<HTMLElement>(null)

  // Список с кнопкой заменяется итогом: фокус переходит на итог, а не теряется.
  useEffect(() => {
    if (result) resultRef.current?.focus()
  }, [result])

  useEffect(() => {
    document.title = 'Быстрый тест — Verst'
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Быстрый тест</h1>
        <p className="text-muted-foreground">Слова, которые ты уже знаешь, не придётся учить заново.</p>
      </header>

      {page.isPending && <p role="status">Загружаем слова…</p>}
      {page.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">{page.error.message}</p>
          <Button onClick={() => page.refetch()}>Повторить</Button>
        </div>
      )}

      {page.data && page.data.words.length === 0 && !result && (
        <section className="surface p-6">
          <FoxNote mood="happy">
            <h2 className="text-title">Слов для теста больше нет</h2>
          </FoxNote>
        </section>
      )}

      {result && (
        <section ref={resultRef} tabIndex={-1} aria-live="polite" className="surface flex flex-col items-start gap-3 p-6 outline-none">
          <FoxNote mood={result.r.next === 'stop' ? 'thinking' : 'happy'}>
            <h2 className="text-title">{`Отмечено слов: ${result.r.marked}`}</h2>
            <p className="text-muted-foreground">{OUTCOME[result.r.next]}</p>
          </FoxNote>
          <div className="flex flex-wrap gap-3">
            {result.r.next === 'more' && page.data?.hasMore && (
              <Button
                onClick={() => {
                  setSkip(nextSkip(skip, result.shown, result.known))
                  setResult(null)
                }}
              >
                Ещё страница
              </Button>
            )}
            <Button asChild variant={result.r.next === 'more' ? 'secondary' : 'default'}>
              <Link to="/learn">К знакомству</Link>
            </Button>
          </div>
        </section>
      )}

      {page.data && page.data.words.length > 0 && !result && (
        <Words
          key={skip}
          skip={skip}
          page={page.data}
          onDone={(r, known) => setResult({ r, known, shown: page.data.words.length })}
        />
      )}
    </div>
  )
}
