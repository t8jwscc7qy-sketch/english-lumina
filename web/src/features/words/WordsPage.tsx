import { Check, ChevronDown, Plus } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { components } from '@/api/schema.gen'
import { cn } from '@/lib/utils'
import { useAddCard } from '@/features/cards/cards'
import { useDebounced } from '@/lib/useDebounced'
import { ExamplesPanel } from './ExamplesPanel'
import { useWordSearch } from './words'

type Word = components['schemas']['Word']

const SEARCH_DELAY_MS = 250
const MAX_RESULTS = 50 // столько отдаёт сервер

const SOURCE_NAMES: Record<string, string> = { cefrj: 'CEFR-J', octanove: 'Octanove' }
const sourceName = (name: string) => SOURCE_NAMES[name] ?? name

/** Кнопка «В карточки»: слово без примеров сервер отклоняет (409), текст ответа показываем как есть. */
function AddToCards({ word }: { word: Word }) {
  const add = useAddCard()
  const busy = useRef(false)

  if (word.inCards || add.isSuccess) {
    return (
      <Button variant="ghost" size="sm" disabled className="-ml-3">
        <Check className="size-4" aria-hidden />В карточках
      </Button>
    )
  }
  return (
    <div className="flex flex-col items-start gap-1">
      <Button
        variant="secondary"
        size="sm"
        disabled={add.isPending}
        onClick={() => {
          if (busy.current) return // двойной клик: пока идёт запрос, второго нет
          busy.current = true
          add.mutate({ lemma: word.lemma, pos: word.pos }, { onSettled: () => (busy.current = false) })
        }}
      >
        {add.isPending ? (
          'Добавляем…'
        ) : (
          <>
            <Plus className="size-4" aria-hidden />В карточки
          </>
        )}
      </Button>
      {add.isError && (
        <p role="alert" className="text-sm text-coral-text">
          {add.error.message}
        </p>
      )}
    </div>
  )
}

function WordRow({ word }: { word: Word }) {
  const [open, setOpen] = useState(false)
  const panelId = useId()
  const levels = word.levels.map((l) => `${sourceName(l.source)} ${l.level}`).join(' · ')
  const notes = word.levels.flatMap((l) => l.notes).join('; ')
  return (
    <li className="flex flex-col gap-1 p-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="text-lg font-semibold break-words">{word.lemma}</span>
        <span className="text-sm text-muted-foreground">{word.pos}</span>
        <span
          aria-label={`Уровень ${word.level}`}
          className="rounded-md bg-muted px-2 py-0.5 text-sm font-semibold tabular-nums"
        >
          {word.level}
        </span>
      </div>
      <p className="text-sm text-muted-foreground">{levels}</p>
      {notes && <p className="text-sm break-words">{notes}</p>}
      <div className="flex flex-col items-start gap-3">
        <div className="flex flex-wrap items-start gap-x-2 gap-y-1">
          <AddToCards word={word} />
        <Button
          variant="ghost"
          size="sm"
          aria-expanded={open}
          aria-controls={open ? panelId : undefined}
          onClick={() => setOpen((v) => !v)}
        >
          Примеры
          <ChevronDown className={cn('size-4 transition-transform', open && 'rotate-180')} aria-hidden />
        </Button>
        </div>
        {open && (
          <div id={panelId} className="w-full">
            <ExamplesPanel lemma={word.lemma} pos={word.pos} />
          </div>
        )}
      </div>
    </li>
  )
}

export function WordsPage() {
  const [input, setInput] = useState('')
  const term = useDebounced(input.trim(), SEARCH_DELAY_MS)
  const search = useWordSearch(term)
  const typing = input.trim() !== ''

  useEffect(() => {
    document.title = 'Слова — Verst'
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Слова</h1>
        <p className="text-muted-foreground">
          Уровень слова по спискам CEFR и примеры предложений из Tatoeba.
        </p>
      </header>

      <div className="flex flex-col gap-2">
        <Label htmlFor="word-search">Слово</Label>
        <Input
          id="word-search"
          type="search"
          autoFocus
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="например, run"
        />
      </div>

      {!typing && <p className="text-muted-foreground">Введи начало слова, например run: покажу его уровень.</p>}

      {typing && search.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {search.error.message}
          </p>
          <Button onClick={() => search.refetch()}>Повторить</Button>
        </div>
      )}

      {typing && !search.isError && !search.data && <p role="status">Ищем…</p>}

      {typing && !search.isError && search.data && (
        <>
          {search.data.words.length === 0 ? (
            <p className="text-muted-foreground">
              Ничего не найдено по «{term || input.trim()}». Ищу по началу слова, а не по середине.
            </p>
          ) : (
            <>
              <p role="status" className="text-sm text-muted-foreground tabular-nums">
                Найдено: {search.data.words.length}
                {search.data.words.length >= MAX_RESULTS && ` (показаны первые ${MAX_RESULTS}, уточни запрос)`}
              </p>
              <ul
                aria-busy={search.isPlaceholderData}
                className="surface divide-y divide-border data-[busy=true]:opacity-60"
                data-busy={search.isPlaceholderData}
              >
                {search.data.words.map((w) => (
                  <WordRow key={`${w.lemma}|${w.pos}`} word={w} />
                ))}
              </ul>
            </>
          )}
        </>
      )}
    </div>
  )
}
