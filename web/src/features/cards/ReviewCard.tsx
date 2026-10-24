import { useCallback, useEffect, useRef, useState } from 'react'
import type { QueueCard } from '@/api/client'
import { Button } from '@/components/ui/button'
import { TatoebaCredit } from '@/features/words/TatoebaCredit'
import { useReviewCard } from './cards'
import { Cloze } from './Cloze'
import { formatInterval } from './format'

type Rating = 1 | 2 | 3 | 4

const RATINGS: { rating: Rating; label: string; field: keyof QueueCard['intervals'] }[] = [
  { rating: 1, label: 'Забыл', field: 'again' },
  { rating: 2, label: 'Трудно', field: 'hard' },
  { rating: 3, label: 'Хорошо', field: 'good' },
  { rating: 4, label: 'Легко', field: 'easy' },
]

/** Ключ идемпотентности: новый для каждой показанной карточки. Повтор оценки с тем же ключом сервер не применяет дважды. */
function newKey(): string {
  return typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

/** Кнопка, ссылка или другой управляющий элемент (кроме «Показать ответ»): Пробел и Enter на них принадлежат им, а не карточке. */
function isOtherControl(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement) || t === document.body || t.closest('[data-reveal]')) return false
  return t.closest('a[href], button, summary, [role="button"], [role="link"], [role="menuitem"], [role="tab"]') !== null
}

function isTypingTarget(t: EventTarget | null): boolean {
  return t instanceof HTMLElement && (t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))
}

type ReviewCardProps = {
  card: QueueCard
  /** Вызывается один раз, когда сервер принял оценку. */
  onRated?: () => void
}

export function ReviewCard({ card, onRated }: ReviewCardProps) {
  const review = useReviewCard()
  const [revealed, setRevealed] = useState(false)
  const keyRef = useRef(newKey())
  const articleRef = useRef<HTMLElement>(null)
  const busyRef = useRef(false)
  const hasSentence = !!card.sentence
  const cardId = card.cardId

  // Новая карточка забирает фокус: клавиши Пробел/Enter/1–4 работают сразу (фокус мог остаться на ссылке меню),
  // а после оценки он не теряется вместе с исчезнувшей кнопкой.
  useEffect(() => {
    articleRef.current?.focus()
  }, [cardId])

  const rate = useCallback(
    (rating: Rating) => {
      if (busyRef.current) return // защита от двойного нажатия: пока идёт запрос, новых оценок нет
      busyRef.current = true
      review.mutate(
        { cardId: card.cardId, rating, key: keyRef.current },
        {
          onSuccess: () => {
            keyRef.current = newKey() // следующая карточка — новый ключ; при ошибке ключ остаётся, и повтор безопасен
            setRevealed(false)
            onRated?.()
          },
          onSettled: () => {
            busyRef.current = false
          },
        },
      )
    },
    [card.cardId, review, onRated],
  )

  // Клавиатура: Пробел или Enter открывает ответ, 1–4 оценивают (только когда ответ открыт).
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target)) return
      if (!revealed && hasSentence && (e.key === ' ' || e.key === 'Enter')) {
        if (isOtherControl(e.target)) return // навигация с клавиатуры по меню и кнопкам не должна ломаться
        e.preventDefault()
        setRevealed(true)
        return
      }
      if ((revealed || !hasSentence) && ['1', '2', '3', '4'].includes(e.key)) {
        e.preventDefault()
        rate(Number(e.key) as Rating)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [revealed, hasSentence, rate])

  return (
    <article ref={articleRef} tabIndex={-1} aria-label="Карточка" className="surface flex flex-col gap-6 p-6 outline-none">
      <p className="text-sm text-muted-foreground">{`${card.level} · ${card.pos}`}</p>

      {card.sentence ? (
        <div className="flex flex-col gap-3">
          <p className="text-xl leading-relaxed break-words">
            <Cloze text={card.sentence.en.text} gap={card.sentence.gap} revealed={revealed} />
          </p>
          <p className="break-words text-muted-foreground">{card.sentence.ru.text}</p>
          {revealed && <TatoebaCredit en={card.sentence.en} ru={card.sentence.ru} />}
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          <p className="text-display break-words">{card.lemma}</p>
          <p className="text-muted-foreground">Предложений для этого слова не нашлось. Вспомни значение и оцени себя.</p>
        </div>
      )}

      {card.sentence && !revealed ? (
        <Button size="lg" onClick={() => setRevealed(true)} aria-keyshortcuts="Space Enter" data-reveal>
          Показать ответ
        </Button>
      ) : (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {RATINGS.map((r) => (
            <Button
              key={r.rating}
              variant={r.rating === 3 ? 'default' : 'secondary'}
              size="lg"
              disabled={review.isPending}
              onClick={() => rate(r.rating)}
              aria-keyshortcuts={String(r.rating)}
              className="h-auto flex-col gap-0.5 py-3"
            >
              <span>{r.label}</span>
              <span className="text-xs font-normal">{`через ${formatInterval(card.intervals[r.field])}`}</span>
            </Button>
          ))}
        </div>
      )}

      {review.isError && (
        <p role="alert" className="rounded-lg border border-coral bg-card p-3 text-sm font-semibold text-coral-text">
          {review.error.message}
        </p>
      )}
    </article>
  )
}
