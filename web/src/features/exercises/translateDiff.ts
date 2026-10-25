/** Отличие ответа от верного варианта в одну правку: что подсветить в ответе пользователя. */
export type Edit = {
  /** Границы в символах (по кодовым точкам) ответа; start == end — буква пропущена. */
  start: number
  end: number
  /** Пропущенная буква (только когда start == end). */
  missing?: string
}

const CONTRACTIONS: Record<string, string> = {
  "won't": 'will not',
  "can't": 'can not',
  cannot: 'can not',
  "shan't": 'shall not',
  "n't": ' not',
  "'m": ' am',
  "'re": ' are',
  "'ve": ' have',
  "'ll": ' will',
}
// Порядок как в Go (`strings.NewReplacer`): на каждой позиции берётся первое совпавшее; «'s» и «'d» двусмысленны и не раскрываются.
const CONTRACTION_RE = /won't|can't|cannot|shan't|n't|'m|'re|'ve|'ll/g

/** Нормализация как на сервере (`normTranslate`): регистр, апостроф, однозначные сокращения, дефис как пробел, без `, . ! ?`. */
export function lightNorm(s: string): string {
  return s
    .toLowerCase()
    .replace(/[’‘ʼ`´]/g, "'")
    .replace(/-/g, ' ')
    .replace(CONTRACTION_RE, (m) => CONTRACTIONS[m])
    .replace(/[,.!?]/g, '')
    .split(/\s+/)
    .filter(Boolean)
    .join(' ')
}

const same = (a: string[], b: string[]) => a.join('') === b.join('')

/**
 * Ищет единственную правку (замена, лишняя или пропущенная буква, перестановка соседних), которая превращает ответ
 * `given` в `expected`. Обе строки уже нормализованы. null — строки равны или правок больше одной.
 */
export function diffOneEdit(given: string, expected: string): Edit | null {
  const a = [...given]
  const b = [...expected]
  let i = 0
  while (i < a.length && i < b.length && a[i] === b[i]) i++
  if (a.length === b.length) {
    if (i === a.length) return null
    if (same(a.slice(i + 1), b.slice(i + 1))) return { start: i, end: i + 1 }
    if (a[i] === b[i + 1] && a[i + 1] === b[i] && same(a.slice(i + 2), b.slice(i + 2)))
      return { start: i, end: i + 2 }
    return null
  }
  if (a.length === b.length + 1)
    return same(a.slice(i + 1), b.slice(i)) ? { start: i, end: i + 1 } : null
  if (b.length === a.length + 1)
    return same(a.slice(i), b.slice(i + 1)) ? { start: i, end: i, missing: b[i] } : null
  return null
}
