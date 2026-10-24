import { useEffect, useRef, useState } from 'react'
import { Check, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type Props = {
  tiles: string[]
  /** Ответ нельзя менять: идёт запрос, получен результат или сбой оставил ответ на повтор. */
  frozen: boolean
  pending: boolean
  /** Результат ответа; пока его нет, строка ответа не окрашена. */
  correct?: boolean
  onSubmit: (given: string) => void
  /** Вернуть фокус на карточку: тогда Enter и цифры работают после клика мышью. */
  restoreFocus: () => void
}

/** Номер клавиши плитки: 1–9, десятая — 0. */
const keyOf = (slot: number) => (slot === 9 ? '0' : String(slot + 1))

function isOtherControl(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement) || t === document.body) return false
  return (
    t.closest(
      'a[href], button, summary, [role="button"], [role="link"], [role="menuitem"], [role="tab"]',
    ) !== null
  )
}

function isTypingTarget(t: EventTarget | null): boolean {
  return (
    t instanceof HTMLElement &&
    (t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))
  )
}

/** «Порядок слов»: плитки внизу, строка ответа сверху. Состояние — номера плиток в порядке выбора. */
export function OrderExercise({ tiles, frozen, pending, correct, onSubmit, restoreFocus }: Props) {
  const [placed, setPlaced] = useState<number[]>([])
  const full = placed.length === tiles.length
  const given = placed.map((i) => tiles[i]).join(' ')
  const stateRef = useRef({ placed, full, given, frozen, tiles, onSubmit })
  useEffect(() => {
    stateRef.current = { placed, full, given, frozen, tiles, onSubmit }
  })

  const rootRef = useRef<HTMLDivElement>(null)
  // Куда поставить фокус после следующего рендера (только когда плитку нажали с клавиатуры).
  const focusAfter = useRef<string | null>(null)
  useEffect(() => {
    const sel = focusAfter.current
    focusAfter.current = null
    if (sel) rootRef.current?.querySelector<HTMLElement>(sel)?.focus()
  }, [placed])

  /** С клавиатуры фокус идёт на следующую свободную плитку (или «Проверить»), мышью — на карточку: цифры и Enter работают сразу. */
  function place(i: number, fromKeyboard: boolean) {
    if (placed.includes(i)) return
    if (fromKeyboard) {
      const taken = new Set([...placed, i])
      const order = [...tiles.keys()]
      const next = [...order.slice(i + 1), ...order.slice(0, i)].find((j) => !taken.has(j))
      focusAfter.current = next === undefined ? '[data-check]' : `[data-bank="${next}"]`
    } else restoreFocus()
    setPlaced([...placed, i])
  }
  function remove(i: number, fromKeyboard: boolean) {
    if (fromKeyboard) {
      const at = placed.indexOf(i)
      const neighbour = placed[at + 1] ?? placed[at - 1]
      focusAfter.current = neighbour === undefined ? `[data-bank="${i}"]` : `[data-row="${neighbour}"]`
    } else restoreFocus()
    setPlaced(placed.filter((x) => x !== i))
  }

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const s = stateRef.current
      if (s.frozen || e.repeat || e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target))
        return
      if (/^[0-9]$/.test(e.key)) {
        const slot = e.key === '0' ? 9 : Number(e.key) - 1
        if (slot < s.tiles.length && !s.placed.includes(slot)) {
          e.preventDefault()
          setPlaced((p) => (p.includes(slot) ? p : [...p, slot]))
        }
      } else if (e.key === 'Backspace') {
        e.preventDefault()
        setPlaced((p) => p.slice(0, -1))
      } else if (e.key === 'Escape') {
        setPlaced([])
      } else if (e.key === 'Enter' && !isOtherControl(e.target)) {
        e.preventDefault()
        if (s.full) s.onSubmit(s.given)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const tileClass =
    'min-h-11 rounded-xl border bg-card px-3 py-1.5 font-semibold transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default'

  return (
    <div ref={rootRef} className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <p className="text-sm font-semibold" id="order-row-label">
          Твой ответ
        </p>
        <div
          className={cn(
            'flex min-h-14 flex-wrap items-center gap-2 rounded-xl border p-2',
            correct === undefined && 'border-dashed border-input',
            correct === true && 'border-grass-text',
            correct === false && 'border-coral',
          )}
        >
          <ol aria-labelledby="order-row-label" className="contents">
            {placed.map((i) => (
              <li key={i}>
                <button
                  type="button"
                  disabled={frozen}
                  data-row={i}
                  onClick={(e) => remove(i, e.detail === 0)}
                  aria-label={`убрать ${tiles[i]}`}
                  className={cn(
                    tileClass,
                    'border-input hover:bg-muted active:scale-[0.97]',
                    correct === false && 'border-coral',
                  )}
                >
                  <span lang="en" aria-hidden>
                    {tiles[i]}
                  </span>
                </button>
              </li>
            ))}
          </ol>
          {placed.length === 0 && (
            <span className="px-2 text-sm text-muted-foreground">Нажимай на слова ниже</span>
          )}
          {correct !== undefined && (
            <span className="ml-auto pr-1">
              {correct ? (
                <Check aria-hidden className="size-5 text-grass-text" strokeWidth={2} />
              ) : (
                <X aria-hidden className="size-5 text-coral-text" strokeWidth={2} />
              )}
            </span>
          )}
        </div>
      </div>

      <div role="group" aria-label="Слова" className="flex flex-wrap gap-2">
        {tiles.map((tile, i) => {
          const used = placed.includes(i)
          return (
            <button
              key={i}
              type="button"
              disabled={used || frozen}
              data-bank={i}
              onClick={(e) => place(i, e.detail === 0)}
              aria-keyshortcuts={keyOf(i)}
              className={cn(
                tileClass,
                'flex items-center gap-2 border-input hover:bg-muted active:scale-[0.97]',
                used &&
                  'border-dashed border-border bg-transparent text-transparent hover:bg-transparent',
              )}
            >
              <kbd
                aria-hidden
                className={cn(
                  'grid size-5 shrink-0 place-items-center rounded-md border border-border font-sans text-xs',
                  used && 'border-transparent',
                )}
              >
                {keyOf(i)}
              </kbd>
              <span lang="en">{tile}</span>
            </button>
          )
        })}
      </div>

      {!frozen || pending ? (
        <div className="flex flex-wrap items-center gap-3">
          <Button
            type="button"
            disabled={!full || pending || frozen}
            data-check
            onClick={() => onSubmit(given)}
            aria-keyshortcuts="Enter"
          >
            Проверить
          </Button>
          <span className="text-sm text-muted-foreground">
            Backspace — убрать последнее, Esc — очистить
          </span>
        </div>
      ) : null}
    </div>
  )
}
