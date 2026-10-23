/** Сколько слов в частотном списке (en_50k): слово не из списка реже последнего места. */
const LIST_SIZE = 50_000

const number = new Intl.NumberFormat('ru-RU')

/** Словесная оценка по месту: до 1 000 очень частое, до 5 000 частое, до 20 000 обычное, дальше редкое. */
function band(rank: number): string {
  if (rank <= 1000) return 'очень частое'
  if (rank <= 5000) return 'частое'
  if (rank <= 20_000) return 'обычное'
  return 'редкое'
}

/** Фраза о частоте слова для результата поиска; rank — место в списке (1 — самое частое), нет — слова нет в списке. */
export function describeFrequency(rank?: number): string {
  if (rank === undefined) return `редкое слово, реже № ${number.format(LIST_SIZE)}`
  return `${band(rank)} слово, № ${number.format(rank)} из ${number.format(LIST_SIZE)}`
}
