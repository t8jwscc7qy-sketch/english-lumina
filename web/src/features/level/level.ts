import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { dismissLevelSuggestion, getLevelSuggestion, type LevelSuggestion, type Settings } from '@/api/client'
import { useSaveSettings } from '@/features/settings/settings'
import { nextExerciseKey } from '@/features/exercises/exercises'
import { progressKey } from '@/features/today/progress'
import { LEVELS } from '@/features/today/levels'

export const levelSuggestionKey = ['level-suggestion'] as const

/** Предложение сменить уровень. Ошибка не мешает странице: карточка просто не показывается. */
export function useLevelSuggestion() {
  return useQuery({
    queryKey: levelSuggestionKey,
    queryFn: getLevelSuggestion,
    retry: false,
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: false,
  })
}

export function useDismissLevelSuggestion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: dismissLevelSuggestion,
    onSuccess: () => qc.setQueryData(levelSuggestionKey, {}),
  })
}

/**
 * Настройки после принятия предложения: уровень меняется на `to`. Цель не может быть ниже уровня,
 * поэтому при повышении выше цели она поднимается вместе с уровнем (карточка об этом говорит).
 */
export function settingsAfterAccept(settings: Settings, to: LevelSuggestion['to']): Settings {
  const target = LEVELS.indexOf(settings.targetLevel) < LEVELS.indexOf(to) ? to : settings.targetLevel
  return { ...settings, currentLevel: to, targetLevel: target }
}

/** Цель поднимется вместе с уровнем (повышение выше текущей цели). */
export function goalFollows(settings: Settings, to: LevelSuggestion['to']): boolean {
  return LEVELS.indexOf(settings.targetLevel) < LEVELS.indexOf(to)
}

export function useAcceptLevelSuggestion() {
  const qc = useQueryClient()
  const save = useSaveSettings()
  return {
    ...save,
    accept: (settings: Settings, to: LevelSuggestion['to']) =>
      save.mutateAsync(settingsAfterAccept(settings, to)).then(() => {
        qc.setQueryData(levelSuggestionKey, {})
        // уровень сменился: прогноз, упражнения и сцена должны перечитать данные
        void qc.invalidateQueries({ queryKey: progressKey })
        void qc.invalidateQueries({ queryKey: nextExerciseKey })
      }),
  }
}
