import { FoxNote } from '@/components/brand/FoxNote'
import { Button } from '@/components/ui/button'
import { useSettings } from '@/features/settings/settings'
import {
  goalFollows,
  useAcceptLevelSuggestion,
  useDismissLevelSuggestion,
  useLevelSuggestion,
} from './level'

/**
 * Предложение сменить уровень: цифры из последних 30 ответов и две кнопки. Решает пользователь:
 * без нажатия ничего не меняется. Нет предложения или оно не загрузилось — карточки нет.
 */
export function LevelSuggestionCard() {
  const suggestion = useLevelSuggestion()
  const settings = useSettings()
  const dismiss = useDismissLevelSuggestion()
  const accept = useAcceptLevelSuggestion()

  const s = suggestion.data?.suggestion
  const current = settings.data
  if (!s || !current) return null

  const percent = Math.round((s.correct / s.total) * 100)
  const up = s.direction === 'up'
  const busy = dismiss.isPending || accept.isPending
  const error = dismiss.error ?? accept.error

  return (
    <section aria-labelledby="level-title" className="surface p-6">
      <FoxNote mood={up ? 'happy' : 'thinking'}>
        <h2 id="level-title" className="text-title">
          {up ? 'Пора на уровень выше?' : `Может, закрепить ${s.to}?`}
        </h2>
        <p>
          {`За последние ${s.total} ответов на ${s.from} ты ответил верно ${s.correct} (${percent}%). `}
          {up
            ? `Попробовать ${s.to}?`
            : `Можно вернуться на ${s.to}, закрепить и снова подняться.`}
        </p>
        {up && goalFollows(current, s.to) && (
          <p className="text-sm text-muted-foreground">
            {`Цель сейчас ${current.targetLevel}: она тоже станет ${s.to}, поднять её можно в настройках.`}
          </p>
        )}
        <p className="text-sm text-muted-foreground">Решать тебе: пока ты не нажмёшь, ничего не изменится.</p>
        <div className="mt-2 flex flex-wrap gap-3">
          <Button disabled={busy} onClick={() => accept.accept(current, s.to).catch(() => {})}>
            {`Перейти на ${s.to}`}
          </Button>
          <Button variant="secondary" disabled={busy} onClick={() => dismiss.mutate()}>
            {up ? 'Пока нет' : `Оставить ${s.from}`}
          </Button>
        </div>
        {error && (
          <p role="alert" className="text-sm text-coral-text">
            {error.message}
          </p>
        )}
      </FoxNote>
    </section>
  )
}
