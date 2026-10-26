import { Link } from 'react-router'
import type { ReadSentence, ReadText } from '@/api/client'
import { Button } from '@/components/ui/button'

type Props = { construction: ReadText['constructions'][number]; sentences: ReadSentence[]; onClose: () => void }

/** Короткое пояснение к конструкции из текста: форма, объяснение, пример из этого же текста и общий пример. */
export function ConstructionPanel({ construction: c, sentences, onClose }: Props) {
  const own = c.sentences.map((i) => sentences[i]).filter(Boolean).slice(0, 2)
  return (
    <section aria-label={`Конструкция: ${c.title}`} onKeyDown={(e) => e.key === 'Escape' && onClose()} className="surface flex flex-col gap-3 p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-title">{c.title}</h3>
        <Button size="sm" variant="secondary" onClick={onClose}>
          Закрыть
        </Button>
      </div>
      <p lang="en" className="font-semibold break-words">
        {c.form}
      </p>
      <p className="max-w-prose">{c.explanation}</p>
      {own.length > 0 && (
        <div className="flex flex-col gap-1 border-l-2 border-input pl-4">
          <p className="text-sm text-muted-foreground">В этом тексте:</p>
          {own.map((s, i) => (
            <p key={`${i}-${s.tatoebaId ?? s.en}`} lang="en" className="break-words">
              {s.en}
            </p>
          ))}
        </div>
      )}
      <div className="flex flex-col gap-0.5 border-l-2 border-input pl-4">
        <p className="text-sm text-muted-foreground">Пример:</p>
        <p lang="en">{c.exampleEn}</p>
        <p className="text-muted-foreground">{c.exampleRu}</p>
      </div>
      {c.sheetId && (
        <p>
          <Link to={`/sheets/${c.sheetId}`} className="underline underline-offset-4">
            Шпаргалка по теме
          </Link>
        </p>
      )}
    </section>
  )
}
