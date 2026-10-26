import type { ReadText } from '@/api/client'

export const TEXT: ReadText = {
  id: 7, mode: 'read', title: 'Новые слова: dog, run', goal: 0.85, coverage: 0.86, cohesion: 0.5,
  targets: ['dog', 'run'],
  sentences: [
    { tatoebaId: 101, author: 'tom', ruTatoebaId: 201, ruAuthor: 'ivan', en: "The dog can't run fast.", ru: 'Собака не может быстро бегать.', tokens: ['The', 'dog', "can't", 'run', 'fast'] },
    { tatoebaId: 102, author: 'ann', ruTatoebaId: 202, ruAuthor: 'olga', en: 'I like my dog.', ru: 'Я люблю свою собаку.', tokens: ['I', 'like', 'my', 'dog'] },
  ],
  hints: [
    { token: 'dog', lexemeId: 11, lemma: 'dog', pos: 'noun', translations: ['собака'], ipa: 'dɔɡ', target: true },
    { token: 'run', lexemeId: 12, lemma: 'run', pos: 'verb', translations: ['бежать', 'управлять'], target: true },
  ],
}
