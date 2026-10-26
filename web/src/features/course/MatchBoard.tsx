import { useMemo, useState } from 'react'
import { Check } from 'lucide-react'
import type { LessonDrill } from '@/api/client'
import { cn } from '@/lib/utils'

/** Перемешивание без случайности: одинаковое при каждом показе задания (сдвиг по кругу и разворот). */
function shuffled<T>(items: T[]): T[] {
  if (items.length < 2) return items
  const shift = Math.ceil(items.length / 2)
  return [...items.slice(shift), ...items.slice(0, shift)].reverse()
}

/**
 * Пары «слово — перевод»: нажми слово, потом перевод. Верная пара закрепляется зелёной рамкой и галочкой,
 * неверная отмечается коралловой рамкой у перевода и считается ошибкой. Когда собраны все пары — onDone.
 */
export function MatchBoard({ drill, onDone }: { drill: LessonDrill; onDone: (mistakes: number) => void }) {
  const pairs = useMemo(() => drill.pairs ?? [], [drill])
  const right = useMemo(() => shuffled(pairs.map((p) => p.ru)), [pairs])
  const [picked, setPicked] = useState<string | null>(null)
  const [matched, setMatched] = useState<Set<string>>(new Set())
  const [wrong, setWrong] = useState<string | null>(null)
  const [mistakes, setMistakes] = useState(0)

  function chooseRu(ru: string) {
    if (!picked) return
    const pair = pairs.find((p) => p.en === picked)
    if (pair?.ru === ru) {
      const next = new Set(matched).add(picked)
      setMatched(next)
      setPicked(null)
      setWrong(null)
      if (next.size === pairs.length) onDone(mistakes)
    } else {
      setWrong(ru)
      setMistakes((m) => m + 1)
    }
  }

  const ruMatched = new Set(pairs.filter((p) => matched.has(p.en)).map((p) => p.ru))
  const tile =
    'pixel-frame pixel-press flex min-h-14 w-full cursor-pointer items-center justify-between gap-2 bg-card px-4 py-2 text-left text-lg [--frame:var(--input)] hover:bg-muted disabled:cursor-default'

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">Нажми слово, потом его перевод.</p>
      <div className="grid grid-cols-2 gap-4">
        <ul className="flex flex-col gap-3">
          {pairs.map((p) => (
            <li key={p.en}>
              <button
                type="button"
                lang="en"
                disabled={matched.has(p.en)}
                aria-pressed={picked === p.en}
                onClick={() => {
                  setPicked(p.en)
                  setWrong(null)
                }}
                className={cn(tile, picked === p.en && 'bg-sun/30 [--frame:var(--pixel-ink)]', matched.has(p.en) && '[--frame:var(--grass-text)] opacity-70')}
              >
                {p.en}
                {matched.has(p.en) && <Check aria-label="пара собрана" className="size-5 text-grass-text" />}
              </button>
            </li>
          ))}
        </ul>
        <ul className="flex flex-col gap-3">
          {right.map((ru) => (
            <li key={ru}>
              <button
                type="button"
                lang="ru"
                disabled={ruMatched.has(ru)}
                onClick={() => chooseRu(ru)}
                className={cn(
                  tile,
                  'text-base',
                  wrong === ru && 'text-coral-text [--frame:var(--coral)]',
                  ruMatched.has(ru) && '[--frame:var(--grass-text)] opacity-70',
                )}
              >
                {ru}
                {ruMatched.has(ru) && <Check aria-hidden className="size-5 text-grass-text" />}
              </button>
            </li>
          ))}
        </ul>
      </div>
      <p role="status" className="sr-only">
        {wrong ? 'Неверная пара' : `Собрано пар: ${matched.size} из ${pairs.length}`}
      </p>
    </div>
  )
}
