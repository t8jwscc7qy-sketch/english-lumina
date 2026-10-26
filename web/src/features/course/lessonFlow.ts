import type { Lesson } from '@/api/client'

/** Шаг урока. Задание с ошибкой возвращается один раз в конце (retry), как у Duolingo. */
export type Step =
  | { kind: 'word'; index: number }
  | { kind: 'text' }
  | { kind: 'question'; index: number }
  | { kind: 'drill'; index: number; retry?: boolean }
  | { kind: 'summary' }

/** Шаги урока по порядку: новые слова, текст, вопросы «понял ли», закрепление, итог. */
export function buildSteps(lesson: Lesson): Step[] {
  return [
    ...lesson.words.map((_, index) => ({ kind: 'word', index }) as const),
    { kind: 'text' } as const,
    ...lesson.questions.map((_, index) => ({ kind: 'question', index }) as const),
    ...lesson.drills.map((_, index) => ({ kind: 'drill', index }) as const),
    { kind: 'summary' } as const,
  ]
}

/** Вставляет повтор задания с ошибкой перед итогом (один раз на задание: повтор повтора не добавляется). */
export function withRetry(steps: Step[], failed: Step): Step[] {
  if (failed.kind !== 'drill' || failed.retry) return steps
  const at = steps.findIndex((s) => s.kind === 'summary')
  const retry: Step = { kind: 'drill', index: failed.index, retry: true }
  return [...steps.slice(0, at), retry, ...steps.slice(at)]
}

/** Название этапа для шапки. */
export function stageLabel(step: Step): string {
  switch (step.kind) {
    case 'word':
      return 'Новые слова'
    case 'text':
      return 'Текст'
    case 'question':
      return 'Понял ли?'
    case 'drill':
      return step.retry ? 'Ещё раз' : 'Закрепление'
    case 'summary':
      return 'Итог'
  }
}

const norm = (s: string) => s.toLowerCase().replace(/’/g, "'").replace(/[.,!?]/g, '').replace(/\s+/g, ' ').trim()

/** Собранное из плиток предложение совпадает с ответом (регистр и знаки не важны). */
export function orderCorrect(chosen: string[], answer: string): boolean {
  return norm(chosen.join(' ')) === norm(answer)
}

/** Минуты урока для итога: от 1, с округлением. */
export function lessonMinutes(seconds: number): number {
  return Math.max(1, Math.round(seconds / 60))
}

/** Склонение «слово»: 1 слово, 2 слова, 5 слов. */
export function wordsWord(n: number): string {
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return 'слово'
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'слова'
  return 'слов'
}
