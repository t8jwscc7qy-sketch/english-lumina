import type { Rating } from './exercises'

const NAMES: Record<string, string> = {
  'grammar:articles': 'Артикли',
  'grammar:prepositions': 'Предлоги',
  'grammar:future': 'Будущее время',
  'grammar:modals': 'Модальные глаголы',
  'grammar:past_simple': 'Прошедшее простое',
  'skill:order': 'Порядок слов',
  'skill:form': 'Выбор формы',
  'pos:noun': 'Существительные',
  'pos:verb': 'Глаголы',
  'pos:adjective': 'Прилагательные',
  'pos:adverb': 'Наречия',
  'pos:pronoun': 'Местоимения',
  'pos:preposition': 'Предлоги (слова)',
  'pos:conjunction': 'Союзы',
  'pos:determiner': 'Определители',
  'pos:number': 'Числительные',
  'pos:interjection': 'Междометия',
  'pos:modal auxiliary': 'Модальные слова',
  'pos:be-verb': 'Глагол be',
  'pos:have-verb': 'Глагол have',
  'pos:do-verb': 'Глагол do',
  'pos:infinitive-to': 'Частица to',
}

/** Название темы по-русски; незнакомый ключ возвращается как есть. */
export function topicName(key: string): string {
  const known = NAMES[key]
  if (known) return known
  const level = /^level:(A1|A2|B1|B2|C1|C2)$/.exec(key)
  return level ? `Уровень ${level[1]}` : key
}

/** Пять тем с наименьшим рейтингом, слабые первыми. Исходный список не меняется. */
export function weakestTopics(topics: Rating[], n = 5): Rating[] {
  return [...topics].sort((a, b) => a.rating - b.rating).slice(0, n)
}

/** Надёжность оценки простыми словами: отклонение 350 у новой темы, у изученной оно падает. */
export function reliabilityLabel(deviation: number): 'высокая' | 'средняя' | 'низкая' {
  if (deviation <= 100) return 'высокая'
  if (deviation <= 200) return 'средняя'
  return 'низкая'
}
