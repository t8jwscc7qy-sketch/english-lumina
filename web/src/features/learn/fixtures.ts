import type { LearnBlock, LearnStatus, LearnWord } from '@/api/client'

export const WORD_APPLE: LearnWord = {
  lexemeId: 1, lemma: 'apple', pos: 'noun', level: 'A1', rank: 1200, translations: ['яблоко'], ipa: 'ˈæpəl',
  audioUrl: 'https://commons.wikimedia.org/wiki/Special:FilePath/En-us-apple.ogg', forms: ['apples'], synonyms: [],
  examples: [{
    en: { text: 'I eat an apple every day.', author: 'CK', tatoebaId: 11 },
    ru: { text: 'Я ем яблоко каждый день.', author: 'ivan', tatoebaId: 12 },
    highlight: { start: 9, end: 14 },
  }],
}

export const WORD_RUN: LearnWord = {
  lexemeId: 2, lemma: 'run', pos: 'verb', level: 'A1', translations: ['бежать', 'управлять'], forms: ['runs', 'ran', 'running'], synonyms: ['jog'],
  examples: [{
    en: { text: 'I run every morning.', author: 'tom', tatoebaId: 21 },
    ru: { text: 'Я бегаю каждое утро.', author: 'anna', tatoebaId: 22 },
    highlight: { start: 2, end: 5 },
  }],
}

export const REASON = { code: 'base', text: 'Столько слов поместится в твои минуты' } as const

export const BLOCK: LearnBlock = { words: [WORD_APPLE, WORD_RUN], limit: 5, remaining: 5, reason: REASON }

export const STATUS: LearnStatus = {
  learned: 3, introduced: 2, knownAtStart: 1, newToday: 1, limit: 5, remaining: 4, reason: REASON, mode: 'auto', perDay: 12, kbReady: true,
  campWords: 4, campMinWords: 50, campOpen: false, trackedWords: 6, textMinWords: 20,
}
