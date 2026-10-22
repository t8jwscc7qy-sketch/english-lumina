import type { Level } from './levels'

/**
 * ДЕМО-ДАННЫЕ. Настоящие появятся, когда будут готовы настройки (уровень и цель),
 * словарь, упражнения и журнал событий. Экран честно помечает их плашкой «Демо-данные».
 */
export const DEMO_TODAY = {
  current: 'B1' as Level,
  target: 'B2' as Level,
  minutesPerDay: 15,
  cardsDue: 24,
  exercises: { count: 12, topic: 'Времена' },
  mistakes: 5,
  streakDays: 3,
  minimumDay: { done: 4, total: 10 },
  levelXp: { value: 36, total: 100 },
}
