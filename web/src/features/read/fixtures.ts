import type { ReadText } from '@/api/client'

export const TEXT: ReadText = {
  id: 7, mode: 'read', title: 'Новые слова: dog, run', goal: 0.85, coverage: 0.86, cohesion: 0.5,
  targets: ['dog', 'run'],
  sentences: [
    { tatoebaId: 101, author: 'tom', ruTatoebaId: 201, ruAuthor: 'ivan', en: "The dog can't run fast.", ru: 'Собака не может быстро бегать.', tokens: ['The', 'dog', "can't", 'run', 'fast'] },
    { tatoebaId: 102, author: 'ann', ruTatoebaId: 202, ruAuthor: 'olga', en: 'I like my dog.', ru: 'Я люблю свою собаку.', tokens: ['I', 'like', 'my', 'dog'] },
  ],
  constructions: [
    { id: 'modals', title: 'Модальные глаголы', form: 'can / could / must + глагол без to', explanation: 'Модальные глаголы выражают умение и долг. После них начальная форма без to.', exampleEn: 'You should rest.', exampleRu: 'Тебе стоит отдохнуть.', sentences: [0] },
    { id: 'past_simple', title: 'Past Simple', form: 'глагол + -ed', explanation: 'Законченное действие в прошлом.', exampleEn: 'She visited her aunt.', exampleRu: 'Она навестила тётю.', sheetId: 'irregular-verbs', sentences: [1] },
  ],
  hints: [
    { token: 'dog', lexemeId: 11, lemma: 'dog', pos: 'noun', translations: ['собака'], ipa: 'dɔɡ', target: true },
    { token: 'run', lexemeId: 12, lemma: 'run', pos: 'verb', translations: ['бежать', 'управлять'], target: true },
  ],
}
