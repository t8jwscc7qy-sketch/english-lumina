import { HighlightedText } from '@/features/words/HighlightedText'
import type { components } from '@/api/schema.gen'

type TextRange = components['schemas']['TextRange']

/**
 * Предложение карточки: слово скрыто пропуском (лицо) или подсвечено (оборот). Диапазон в символах, не в единицах
 * UTF-16, поэтому режем через Array.from. Кривой диапазон ничего не скрывает. Всё выводится как текст.
 */
export function Cloze({ text, gap, revealed }: { text: string; gap: TextRange; revealed: boolean }) {
  if (revealed) return <HighlightedText text={text} range={gap} />
  const chars = Array.from(text)
  if (gap.start < 0 || gap.start >= gap.end || gap.end > chars.length) return <>{text}</>
  return (
    <>
      {chars.slice(0, gap.start).join('')}
      <span role="img" aria-label="пропуск" className="mx-0.5 inline-block rounded-md bg-muted px-2 text-muted-foreground">
        _____
      </span>
      {chars.slice(gap.end).join('')}
    </>
  )
}
