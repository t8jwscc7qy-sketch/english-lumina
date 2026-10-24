import type { components } from '@/api/schema.gen'

type TextRange = components['schemas']['TextRange']

/**
 * Текст с выделенным диапазоном. Границы приходят в символах (кодовых точках), а у строки JavaScript
 * единицы UTF-16, поэтому режем через Array.from: эмодзи и редкие знаки не сдвигают выделение.
 * Кривой диапазон ничего не выделяет. Всё выводится как текст, без разметки.
 */
export function HighlightedText({ text, range }: { text: string; range?: TextRange }) {
  const chars = Array.from(text)
  if (!range || range.start < 0 || range.start >= range.end || range.end > chars.length) return <>{text}</>
  return (
    <>
      {chars.slice(0, range.start).join('')}
      <mark className="rounded bg-muted px-0.5 font-semibold text-foreground">{chars.slice(range.start, range.end).join('')}</mark>
      {chars.slice(range.end).join('')}
    </>
  )
}
