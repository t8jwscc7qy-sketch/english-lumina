import type { CoursePath, Lesson } from '@/api/client'

/** Маленький урок для тестов: два слова, диалог, вопрос и три задания всех видов. */
export const lessonFixture: Lesson = {
  id: 'u1-l2',
  title: 'Как дела?',
  number: 2,
  unit: 1,
  unitTitle: 'Знакомство',
  done: false,
  words: [
    { lemma: 'how', pos: 'adverb', ru: 'как', forms: [], example: { speaker: 'Tom', en: 'How are you?', ru: 'Как дела?' } },
    { lemma: 'fine', pos: 'adjective', ru: 'хорошо', forms: [], example: { speaker: 'Anna', en: 'I am fine.', ru: 'Хорошо.' } },
  ],
  sentences: [
    { speaker: 'Tom', en: 'Hello, Anna! How are you?', ru: 'Привет, Анна! Как дела?' },
    { speaker: 'Anna', en: 'I am fine.', ru: 'Хорошо.' },
  ],
  questions: [{ prompt: 'Как дела у Анны?', options: ['Хорошо', 'Плохо'], answer: 0 }],
  drills: [
    { kind: 'gap', old: false, prompt: 'Как дела?', text: '___ are you?', options: ['How', 'Fine', 'I'], answer: 'How' },
    { kind: 'order', old: false, prompt: 'Я в порядке.', tiles: ['fine', 'I', 'am'], answer: 'I am fine' },
    {
      kind: 'match',
      old: true,
      pairs: [
        { en: 'how', ru: 'как' },
        { en: 'fine', ru: 'хорошо' },
        { en: 'hello', ru: 'привет' },
      ],
    },
  ],
  note: { title: 'How are you?', text: 'Так спрашивают «Как дела?».' },
  glossary: { hello: 'привет', how: 'как', are: 'есть', you: 'ты', i: 'я', am: 'есть', fine: 'хорошо' },
}

export const pathFixture: CoursePath = {
  units: [
    {
      unit: 1,
      title: 'Знакомство',
      lessons: [
        { id: 'u1-l1', title: 'Привет', number: 1, unit: 1, state: 'done', newWords: 5 },
        { id: 'u1-l2', title: 'Как дела?', number: 2, unit: 1, state: 'current', newWords: 4 },
        { id: 'u1-l3', title: 'Кто это?', number: 3, unit: 1, state: 'locked', newWords: 5 },
      ],
    },
  ],
  next: { id: 'u1-l2', title: 'Как дела?', number: 2, unit: 1, state: 'current', newWords: 4 },
}
