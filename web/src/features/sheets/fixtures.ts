import type { Sheet, SheetSummary } from '@/api/client'

export const SUMMARIES: SheetSummary[] = [
  { id: 'to-be', title: 'Глагол to be', summary: 'Когда нужно сказать «я есть»', level: 'A1', tags: ['глаголы', 'основы'], favorite: true, status: 'draft', weakCount: 0, openedAt: '2026-10-26T10:00:00Z' },
  { id: 'articles', title: 'Артикли a / an / the', summary: 'Как выбрать артикль', level: 'A1', tags: ['артикли', 'основы'], favorite: true, status: 'reviewed', weakCount: 1, openedAt: '2026-10-26T12:00:00Z' },
  { id: 'irregular-verbs', title: 'Неправильные глаголы', summary: 'Три формы частых глаголов', level: 'A2', tags: ['глаголы', 'таблицы'], favorite: false, status: 'draft', weakCount: 2 },
]

export const SHEET: Sheet = {
  id: 'irregular-verbs', title: 'Неправильные глаголы', summary: 'Три формы частых глаголов', level: 'A2', tags: ['глаголы'],
  favorite: false, status: 'draft', weakCount: 1, sources: ['собственные знания автора'],
  blocks: [
    { type: 'text', text: 'Нужно запомнить.' },
    {
      type: 'table', bind: 'word', title: 'Формы', columns: [{ title: 'Глагол' }, { title: 'Past' }],
      rows: [
        { key: 'go', cells: ['go', 'went'], status: 'hard' },
        { key: 'see', cells: ['see', 'saw'], status: 'unseen' },
        { key: 'take', cells: ['take', 'took'], status: 'learned' },
      ],
    },
    { type: 'tip', text: 'Past Simple: «I **wrote**».' },
    { type: 'mistakes', pairs: [{ wrong: 'I goed', right: 'I went', note: 'go неправильный' }] },
    { type: 'examples', examples: [{ en: 'I **went** home.', ru: 'Я пошёл домой.' }] },
  ],
}
