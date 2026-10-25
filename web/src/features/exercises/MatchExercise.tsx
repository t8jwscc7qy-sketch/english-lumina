import { useEffect, useRef, useState } from 'react'
import { Check, X } from 'lucide-react'
import type { AnswerResult } from '@/api/client'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import {
  emptyState,
  isComplete,
  optionLabel,
  pairColor,
  selectOption,
  selectWord,
  toGiven,
  undoLast,
  type MatchState,
} from './match'

type Props = {
  words: string[]
  options: string[]
  /** Ответ нельзя менять: идёт запрос, получен результат или сбой оставил ответ на повтор. */
  frozen: boolean
  pending: boolean
  result?: AnswerResult
  onSubmit: (given: string) => void
  /** Вернуть фокус на карточку: тогда Enter и цифры работают после клика мышью. */
  restoreFocus: () => void
}

/** Цвета пар: рамка плитки и значок с номером пары. Цвет не единственный признак: номер и подписи в именах плиток. */
const PAIR_STYLE = [
  { frame: '[--frame:var(--sky)]', badge: 'bg-sky text-ink' },
  { frame: '[--frame:var(--grass)]', badge: 'bg-grass text-ink' },
  { frame: '[--frame:var(--sun)]', badge: 'bg-sun text-ink' },
  { frame: '[--frame:var(--coral-soft)]', badge: 'bg-coral-soft text-ink' },
  { frame: '[--frame:var(--bridge)]', badge: 'bg-[var(--bridge)] text-white dark:text-ink' },
] as const

function isOtherControl(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement) || t === document.body) return false
  return t.closest('a[href], button, summary, [role="button"], [role="link"], [role="menuitem"], [role="tab"]') !== null
}

function isTypingTarget(t: EventTarget | null): boolean {
  return t instanceof HTMLElement && (t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))
}

/** «Сопоставление»: слева английские слова, справа переводы вперемешку; пары собираются нажатиями или клавишами. */
export function MatchExercise({ words, options, frozen, pending, result, onSubmit, restoreFocus }: Props) {
  const [state, setState] = useState<MatchState>(() => emptyState(words.length))
  const [input, setInput] = useState<'pointer' | 'keyboard'>('pointer')
  const complete = isComplete(state)
  const given = toGiven(words, options, state)

  const live = useRef({ state, complete, given, frozen, pending, onSubmit, n: words.length, m: options.length })
  useEffect(() => {
    live.current = { state, complete, given, frozen, pending, onSubmit, n: words.length, m: options.length }
  })

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const c = live.current
      if (e.repeat || e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target) || c.frozen) return
      const num = Number(e.key)
      const letter = e.key.length === 1 ? 'abcdefghij'.indexOf(e.key.toLowerCase()) : -1
      if (Number.isInteger(num) && num >= 1 && num <= c.n) {
        e.preventDefault()
        setInput('keyboard')
        setState((s) => selectWord(s, num - 1))
      } else if (letter >= 0 && letter < c.m) {
        e.preventDefault()
        setInput('keyboard')
        setState((s) => selectOption(s, letter))
      } else if (e.key === 'Backspace') {
        e.preventDefault()
        setInput('keyboard')
        setState(undoLast)
      } else if (e.key === 'Enter' && c.complete && !c.pending && !isOtherControl(e.target)) {
        e.preventDefault()
        setInput('keyboard')
        c.onSubmit(c.given)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const wordOf = (o: number) => state.pairs.indexOf(o)
  const verdict = (w: number) => result?.pairs?.find((p) => p.word === words[w])
  const rightCount = result?.pairs?.filter((p) => p.correct).length ?? 0

  function tileTone(w: number | null): string {
    if (w === null || w < 0) return '[--frame:var(--input)]'
    const v = verdict(w)
    if (v) return v.correct ? '[--frame:var(--grass-text)]' : '[--frame:var(--coral)]'
    return PAIR_STYLE[pairColor(w)].frame
  }

  function Mark({ w }: { w: number }) {
    const v = verdict(w)
    if (v) {
      return v.correct ? (
        <Check aria-hidden className="size-5 shrink-0 text-grass-text" strokeWidth={2} />
      ) : (
        <X aria-hidden className="size-5 shrink-0 text-coral-text" strokeWidth={2} />
      )
    }
    return (
      <span aria-hidden className={cn('grid size-6 shrink-0 place-items-center text-xs font-bold tabular-nums', PAIR_STYLE[pairColor(w)].badge)}>
        {w + 1}
      </span>
    )
  }

  const tile =
    'pixel-frame pixel-press flex min-h-12 w-full items-center gap-3 bg-card px-3 py-2 text-left font-semibold transition-colors hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring disabled:cursor-default'

  return (
    <div data-input={input} onPointerDown={() => setInput('pointer')} className="flex flex-col gap-5">
      <div className="grid grid-cols-2 gap-x-4 gap-y-3 sm:gap-x-8">
        <ol aria-label="Слова" className="flex flex-col gap-3">
          {words.map((word, i) => {
            const paired = state.pairs[i]
            const v = verdict(i)
            const name =
              `${i + 1}. ${word}` +
              (paired !== null ? `, соединено с «${options[paired]}»` : '') +
              (v ? (v.correct ? ', верно' : ', неверно') : '')
            return (
              <li key={word}>
                <button
                  type="button"
                  disabled={frozen}
                  aria-label={name}
                  aria-pressed={state.selected === i}
                  aria-keyshortcuts={String(i + 1)}
                  onClick={() => {
                    setState((s) => selectWord(s, i))
                    restoreFocus()
                  }}
                  className={cn(tile, state.selected === i ? '[--frame:var(--sun)] -translate-y-0.5 bg-muted' : tileTone(paired !== null ? i : null))}
                >
                  <kbd aria-hidden className="grid size-6 shrink-0 place-items-center border border-border font-sans text-xs">{i + 1}</kbd>
                  <span lang="en" className="min-w-0 flex-1 break-words">{word}</span>
                  {paired !== null && <Mark w={i} />}
                </button>
              </li>
            )
          })}
        </ol>
        <ul aria-label="Переводы" className="flex flex-col gap-3">
          {options.map((option, j) => {
            const w = wordOf(j)
            const v = w >= 0 ? verdict(w) : undefined
            const name =
              `${optionLabel(j)}. ${option}` +
              (w >= 0 ? `, соединено со словом «${words[w]}»` : '') +
              (v ? (v.correct ? ', верно' : ', неверно') : '')
            return (
              <li key={option}>
                <button
                  type="button"
                  disabled={frozen}
                  aria-label={name}
                  aria-keyshortcuts={optionLabel(j)}
                  onClick={() => {
                    setState((s) => selectOption(s, j))
                    restoreFocus()
                  }}
                  className={cn(tile, tileTone(w >= 0 ? w : null))}
                >
                  <kbd aria-hidden className="grid size-6 shrink-0 place-items-center border border-border font-sans text-xs uppercase">{optionLabel(j)}</kbd>
                  <span lang="ru" className="min-w-0 flex-1 break-words">{option}</span>
                  {w >= 0 && <Mark w={w} />}
                </button>
              </li>
            )
          })}
        </ul>
      </div>

      {!result && (
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" disabled={!complete || frozen || pending} onClick={() => onSubmit(given)}>
            {pending ? 'Проверяем…' : 'Проверить'}
          </Button>
          <p className="text-sm text-muted-foreground">
            {complete ? 'Все пары собраны.' : `Соединено: ${state.pairs.filter((p) => p !== null).length} из ${words.length}.`}
          </p>
        </div>
      )}

      {result?.pairs && (
        <div className="flex flex-col gap-3">
          <p className="font-semibold tabular-nums">{`Верно ${rightCount} из ${result.pairs.length}`}</p>
          <ul aria-label="Разбор пар" className="flex flex-col gap-1.5 text-sm">
            {result.pairs.map((p) => (
              <li key={p.word} className={cn('flex flex-wrap items-baseline gap-x-2', !p.correct && 'text-coral-text')}>
                <span lang="en" className="font-semibold">{p.word}</span>
                <span aria-hidden>→</span>
                <span lang="ru">{p.given}</span>
                <span className="font-semibold">{p.correct ? '— верно' : '— неверно'}</span>
                {!p.correct && (
                  <span>
                    {'верный перевод: '}
                    <span lang="ru" className="font-semibold">{p.answer}</span>
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

