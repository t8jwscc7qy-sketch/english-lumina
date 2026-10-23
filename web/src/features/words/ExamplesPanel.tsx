import { Button } from '@/components/ui/button'
import { HighlightedText } from './HighlightedText'
import { useWordExamples } from './words'

const tatoebaUrl = (id: number) => `https://tatoeba.org/en/sentences/show/${id}`

function TatoebaLink({ id }: { id: number }) {
  return (
    <a href={tatoebaUrl(id)} target="_blank" rel="noreferrer noopener" className="underline underline-offset-4 hover:no-underline">
      #{id}
    </a>
  )
}

/** Примеры предложений для слова. Лицензия CC BY требует автора: у каждого примера номер и автор обеих сторон со ссылкой на tatoeba.org. */
export function ExamplesPanel({ lemma, pos }: { lemma: string; pos: string }) {
  const examples = useWordExamples(lemma, pos)

  if (examples.isPending) return <p role="status" className="text-sm text-muted-foreground">Загружаем примеры…</p>

  if (examples.isError) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p role="alert" className="text-sm text-muted-foreground">
          {examples.error.message}
        </p>
        <Button size="sm" variant="secondary" onClick={() => examples.refetch()}>
          Повторить
        </Button>
      </div>
    )
  }

  if (examples.data.examples.length === 0) {
    return <p className="text-sm text-muted-foreground">Примеров нет: в Tatoeba не нашлось подходящих предложений с этим словом.</p>
  }

  return (
    <ul className="flex flex-col gap-4">
      {examples.data.examples.map((ex) => (
        <li key={`${ex.en.tatoebaId}-${ex.ru.tatoebaId}`} className="flex flex-col gap-1">
          <p className="break-words">
            <HighlightedText text={ex.en.text} range={ex.highlight} />
          </p>
          <p className="text-sm break-words text-muted-foreground">{ex.ru.text}</p>
          <p className="text-xs text-muted-foreground">
            Tatoeba: <TatoebaLink id={ex.en.tatoebaId} /> {ex.en.author} · <TatoebaLink id={ex.ru.tatoebaId} /> {ex.ru.author}
          </p>
        </li>
      ))}
    </ul>
  )
}
