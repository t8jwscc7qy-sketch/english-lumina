import { useEffect, useRef, useState } from 'react'
import type { ReadWordHint } from '@/api/client'
import { Button } from '@/components/ui/button'
import { useAddCard } from '@/features/cards/cards'
import { useKnowWord } from './read'

type Props = { token: string; hint: ReadWordHint | undefined; onClose: () => void }

/** Подсказка к слову: перевод, произношение, «Добавить в карточки» и «Знаю». Нет подсказки в базе — так и говорит. */
export function WordPanel({ token, hint, onClose }: Props) {
  const add = useAddCard()
  const know = useKnowWord()
  const [done, setDone] = useState<string | null>(null)
  const ref = useRef<HTMLElement>(null)

  // Панель открывается под текстом: переносим на неё фокус и прокручиваем к ней; Esc закрывает.
  useEffect(() => {
    ref.current?.focus()
    if (typeof ref.current?.scrollIntoView === 'function') ref.current.scrollIntoView({ block: 'nearest' })
  }, [])

  return (
    <section
      ref={ref}
      tabIndex={-1}
      aria-label={`Слово ${token}`}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onClose()
      }}
      className="surface flex flex-col gap-3 p-4 outline-none"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 lang="en" className="text-title break-words">
          {token}
        </h2>
        <Button size="sm" variant="secondary" onClick={onClose}>
          Закрыть
        </Button>
      </div>
      {hint ? (
        <>
          <p lang="ru" className="text-lg font-semibold break-words">
            {hint.translations.length > 0 ? hint.translations.join(', ') : 'перевода нет'}
          </p>
          <p className="text-sm text-muted-foreground">
            {hint.lemma !== token && <span lang="en">{`${hint.lemma} · `}</span>}
            {hint.pos}
            {hint.ipa && (
              <>
                {' · '}
                <span lang="en-fonipa">{`/${hint.ipa}/`}</span>
              </>
            )}
            {hint.target && ' · новое слово текста'}
          </p>
          <div className="flex flex-wrap gap-3">
            <Button
              size="sm"
              disabled={add.isPending || done === 'card'}
              onClick={() => add.mutate({ lemma: hint.lemma, pos: hint.pos }, { onSuccess: () => setDone('card') })}
            >
              {done === 'card' ? 'Добавлено в карточки' : 'Добавить в карточки'}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              disabled={know.isPending || done === 'known'}
              onClick={() => know.mutate(hint.lexemeId, { onSuccess: (marked) => setDone(marked ? 'known' : 'has') })}
            >
              {done === 'known' ? 'Отмечено как известное' : done === 'has' ? 'Слово уже учится' : 'Знаю'}
            </Button>
          </div>
        </>
      ) : (
        <p className="text-muted-foreground">Подсказки для этой формы пока нет. Посмотри перевод предложения.</p>
      )}
      {(add.isError || know.isError) && (
        <p role="alert" className="text-sm text-coral-text">
          {(add.error ?? know.error)?.message}
        </p>
      )}
    </section>
  )
}
