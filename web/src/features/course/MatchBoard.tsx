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
  // Справа номера пар в перемешанном порядке: работаем с номерами, а не со строками — одинаковые переводы не путаются.
  const right = useMemo(() => shuffled(pairs.map((_, i) => i)), [pairs])
  const [picked, setPicked] = useState<number | null>(null)
  const [leftDone, setLeftDone] = useState<Set<number>>(new Set())
  const [rightDone, setRightDone] = useState<Set<number>>(new Set())
  const [wrong, setWrong] = useState<number | null>(null)
  const [mistakes, setMistakes] = useState(0)

  function chooseRu(i: number) {
    if (picked === null) return
    if (pairs[i].ru === pairs[picked].ru) {
      const left = new Set(leftDone).add(picked)
      setLeftDone(left)
      setRightDone(new Set(rightDone).add(i))
      setPicked(null)
      setWrong(null)
      if (left.size === pairs.length) onDone(mistakes)
    } else {
      setWrong(i)
      setMistakes((m) => m + 1)
    }
  }

  const tile =
    'pixel-frame pixel-press flex min-h-14 w-full cursor-pointer items-center justify-between gap-2 bg-card px-4 py-2 text-left text-lg [--frame:var(--input)] hover:bg-muted disabled:cursor-default'

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">Нажми слово, потом его перевод.</p>
      <div className="grid grid-cols-2 gap-4">
        <ul className="flex flex-col gap-3">
          {pairs.map((p, i) => (
            <li key={i}>
              <button
                type="button"
                lang="en"
                disabled={leftDone.has(i)}
                aria-pressed={picked === i}
                onClick={() => {
                  setPicked(i)
                  setWrong(null)
                }}
                className={cn(tile, picked === i && 'bg-sun/30 [--frame:var(--pixel-ink)]', leftDone.has(i) && '[--frame:var(--grass-text)] opacity-70')}
              >
                {p.en}
                {leftDone.has(i) && <Check aria-label="пара собрана" className="size-5 text-grass-text" />}
              </button>
            </li>
          ))}
        </ul>
        <ul className="flex flex-col gap-3">
          {right.map((i) => (
            <li key={i}>
              <button
                type="button"
                lang="ru"
                disabled={rightDone.has(i)}
                onClick={() => chooseRu(i)}
                className={cn(tile, 'text-base', wrong === i && 'text-coral-text [--frame:var(--coral)]', rightDone.has(i) && '[--frame:var(--grass-text)] opacity-70')}
              >
                {pairs[i].ru}
                {rightDone.has(i) && <Check aria-hidden className="size-5 text-grass-text" />}
              </button>
            </li>
          ))}
        </ul>
      </div>
      <p role="status" className="sr-only">
        {wrong !== null ? 'Неверная пара' : `Собрано пар: ${leftDone.size} из ${pairs.length}`}
      </p>
    </div>
  )
}
