/** Часть предложения: слово (его можно нажать) или всё остальное (пробелы, знаки). */
export type Segment = { text: string; word: boolean; key: string }

// Слова режутся так же, как на сервере (CasedTokens): буквы, цифры, апостроф и дефис внутри слова.
const WORD = /[\p{L}\p{N}]+(?:['’-][\p{L}\p{N}]+)*/gu

/** Разбор предложения на слова и промежутки; key — слово в нижнем регистре с обычным апострофом. */
export function segment(text: string): Segment[] {
  const out: Segment[] = []
  let last = 0
  for (const m of text.matchAll(WORD)) {
    if (m.index > last) out.push({ text: text.slice(last, m.index), word: false, key: '' })
    out.push({ text: m[0], word: true, key: m[0].toLowerCase().replace(/’/g, "'") })
    last = m.index + m[0].length
  }
  if (last < text.length) out.push({ text: text.slice(last), word: false, key: '' })
  return out
}

/** Номер слова (по порядку слов предложения), на которое приходится символ charIndex; -1, если вне слов. */
export function wordAtChar(text: string, charIndex: number): number {
  let n = -1
  for (const m of text.matchAll(WORD)) {
    n++
    if (charIndex >= m.index && charIndex < m.index + m[0].length) return n
    if (m.index > charIndex) return -1
  }
  return -1
}
