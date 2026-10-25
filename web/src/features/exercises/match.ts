/** Логика «сопоставления»: чистые функции без экрана. Слова и переводы — индексы, состояние неизменяемое. */

export type MatchState = {
  /** Выбранное слово, ждущее перевода. */
  selected: number | null
  /** Для каждого слова индекс его перевода или null. */
  pairs: (number | null)[]
  /** Номера слов в порядке соединения (для «отменить последнюю»). */
  history: number[]
}

export const emptyState = (n: number): MatchState => ({ selected: null, pairs: Array<number | null>(n).fill(null), history: [] })

const inRange = (s: MatchState, i: number) => Number.isInteger(i) && i >= 0 && i < s.pairs.length

/** Нажали слово: у соединённого рвёт пару; у выбранного снимает выбор; иначе выбирает. */
export function selectWord(s: MatchState, i: number): MatchState {
  if (!inRange(s, i)) return s
  if (s.pairs[i] !== null) return unpair(s, i)
  return { ...s, selected: s.selected === i ? null : i }
}

/** Нажали перевод: при выбранном слове соединяет (занятый перевод переносится), иначе разрывает его пару. */
export function selectOption(s: MatchState, j: number): MatchState {
  if (!inRange(s, j)) return s
  const owner = s.pairs.indexOf(j)
  if (s.selected === null) return owner >= 0 ? unpair(s, owner) : s
  let next = owner >= 0 ? unpair(s, owner) : s
  const pairs = next.pairs.slice()
  pairs[s.selected] = j
  next = { selected: null, pairs, history: [...next.history.filter((w) => w !== s.selected), s.selected] }
  return next
}

/** Разорвать пару слова (если она есть). Выбор слова не трогаем. */
export function unpair(s: MatchState, i: number): MatchState {
  if (!inRange(s, i) || s.pairs[i] === null) return s
  const pairs = s.pairs.slice()
  pairs[i] = null
  return { ...s, selected: null, pairs, history: s.history.filter((w) => w !== i) }
}

/** Разорвать последнюю собранную пару; пар нет — снять выбор слова. */
export function undoLast(s: MatchState): MatchState {
  const last = s.history[s.history.length - 1]
  if (last === undefined) return s.selected === null ? s : { ...s, selected: null }
  return unpair(s, last)
}

export const isComplete = (s: MatchState): boolean => s.pairs.length > 0 && s.pairs.every((p) => p !== null)

/** Ответ для сервера: «слово=перевод;…» в порядке слов, только собранные пары. */
export function toGiven(words: string[], options: string[], s: MatchState): string {
  const out: string[] = []
  s.pairs.forEach((o, i) => {
    if (o !== null) out.push(`${words[i]}=${options[o]}`)
  })
  return out.join(';')
}

/** Цвет пары по номеру слова: пять цветов по кругу. */
export const pairColor = (wordIndex: number): number => wordIndex % 5

/** Буква на плитке перевода: a–e. */
export const optionLabel = (j: number): string => 'abcdefghij'[j] ?? String(j + 1)
