import { useState } from 'react'
import type { ReadSentence, ReadWordHint } from '@/api/client'
import { segment } from './segments'

type Props = {
  sentences: ReadSentence[]
  hints: ReadWordHint[]
  selected: string | null
  onPick: (key: string, sentenceIndex: number) => void
  /** Предложение, которое сейчас читается вслух, и слово в нём. */
  playing?: { sentence: number; word: number } | null
  showAllTranslations: boolean
}

/**
 * Текст предложениями. Слова — кнопки (подсказка по нажатию); новые слова текста выделены подчёркиванием.
 * Перевод предложения открывается кнопкой. Все тексты выводятся как текст, без разметки.
 */
export function TextView({ sentences, hints, selected, onPick, playing, showAllTranslations }: Props) {
  const targets = new Set(hints.filter((h) => h.target).map((h) => h.token))
  const [open, setOpen] = useState<Set<number>>(new Set())

  function toggle(i: number) {
    setOpen((s) => {
      const next = new Set(s)
      if (!next.delete(i)) next.add(i)
      return next
    })
  }

  return (
    <ol className="flex flex-col gap-4">
      {sentences.map((s, i) => {
        let wordNo = -1
        const shown = showAllTranslations || open.has(i)
        return (
          <li key={s.tatoebaId} lang="en" className={`flex flex-col gap-1 ${playing?.sentence === i ? 'border-l-4 border-primary pl-3' : 'border-l-4 border-transparent pl-3'}`}>
            <p className="text-xl leading-relaxed break-words">
              {segment(s.en).map((seg, k) => {
                if (!seg.word) return <span key={k}>{seg.text}</span>
                wordNo++
                const isTarget = targets.has(seg.key)
                const reading = playing?.sentence === i && playing.word === wordNo
                return (
                  <button
                    key={k}
                    type="button"
                    onClick={() => onPick(seg.key, i)}
                    aria-pressed={selected === seg.key}
                    className={`rounded px-0.5 ${isTarget ? 'underline decoration-primary decoration-2 underline-offset-4' : ''} ${reading ? 'bg-primary text-primary-foreground' : ''} ${selected === seg.key ? 'bg-muted' : ''} hover:bg-muted`}
                  >
                    {seg.text}
                  </button>
                )
              })}
            </p>
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <button type="button" aria-expanded={shown} onClick={() => toggle(i)} className="text-sm underline underline-offset-4">
                {shown ? 'Скрыть перевод' : 'Перевод'}
              </button>
              {shown && (
                <span lang="ru" className="text-muted-foreground break-words">
                  {s.ru}
                </span>
              )}
            </div>
          </li>
        )
      })}
    </ol>
  )
}
