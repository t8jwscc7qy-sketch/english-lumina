import { useId, useMemo, useRef, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { ApiError, type Settings } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LEVELS, type Level } from '@/features/today/levels'
import { cn } from '@/lib/utils'
import { ForecastCounter } from './ForecastCounter'
import { LevelPicker } from './LevelPicker'
import { useSaveSettings } from './settings'

const GENERIC_ERROR = 'Не получилось сохранить. Попробуй ещё раз.'
const MINUTE_PRESETS = [10, 15, 30, 60]
const MIN_MINUTES = 5
const MAX_MINUTES = 240
const GOAL_YEARS = 5
const STEPS = ['Где ты сейчас?', 'Куда идём?', 'Сколько занимаешься?'] as const

function nextLevel(level: Level): Level {
  return LEVELS[Math.min(LEVELS.indexOf(level) + 1, LEVELS.length - 1)]
}

function isoDay(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function timezones(selected: string): string[] {
  const list = typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : []
  return list.includes(selected) ? list : [selected, ...list]
}

/**
 * Форма настроек обучения: в онбординге три шага, в настройках одна страница.
 * Рядом всегда живой счётчик срока.
 */
export function SettingsForm({
  initial,
  mode,
  onSaved,
}: {
  initial: Settings | null
  mode: 'onboarding' | 'settings'
  onSaved: () => void
}) {
  const save = useSaveSettings()
  const sending = useRef(false)
  const errorId = useId()
  const [step, setStep] = useState(0)
  const [error, setError] = useState<string | null>(null)

  const [current, setCurrent] = useState<Level | null>(initial?.currentLevel ?? null)
  const [target, setTarget] = useState<Level | null>(initial?.targetLevel ?? null)
  const [targetDate, setTargetDate] = useState(initial?.targetDate ?? '')
  const [minutesText, setMinutesText] = useState(String(initial?.dailyMinutes ?? 15))
  const [timezone, setTimezone] = useState(
    initial?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone ?? 'UTC',
  )

  const zones = useMemo(() => timezones(timezone), [timezone])
  const [today] = useState(() => new Date())
  const minutes = Number(minutesText)
  const minutesValid = minutesText.trim() !== '' && Number.isInteger(minutes) && minutes >= MIN_MINUTES && minutes <= MAX_MINUTES
  const canSave = current !== null && target !== null && minutesValid

  function chooseCurrent(level: Level) {
    setCurrent(level)
    // Цель не бывает ниже уровня: подтягиваем. Первый выбор сразу предлагает следующий уровень.
    if (target === null || LEVELS.indexOf(target) < LEVELS.indexOf(level)) setTarget(nextLevel(level))
  }

  async function submit() {
    if (sending.current || !canSave) return
    sending.current = true
    setError(null)
    const payload: Settings = {
      currentLevel: current,
      targetLevel: target,
      dailyMinutes: minutes,
      timezone,
      ...(targetDate ? { targetDate } : {}),
    }
    try {
      await save.mutateAsync(payload)
      if (mode === 'settings') toast.success('Настройки сохранены')
      onSaved()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : GENERIC_ERROR)
    } finally {
      sending.current = false
    }
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    // Enter на первых шагах онбординга ведёт дальше, а не сохраняет недозаполненное.
    if (mode === 'onboarding' && step < STEPS.length - 1) {
      if (step > 0 || current) setStep(step + 1)
      return
    }
    void submit()
  }

  const showLevel = mode === 'settings' || step === 0
  const showGoal = mode === 'settings' || step === 1
  const showPace = mode === 'settings' || step === 2
  const isOnboarding = mode === 'onboarding'
  const lastStep = step === STEPS.length - 1

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem] lg:items-start">
      <form
        onSubmit={onSubmit}
        noValidate
        aria-describedby={error ? errorId : undefined}
        className="surface flex min-w-0 flex-col gap-6 p-6"
      >
        {isOnboarding && (
          <header className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
            <div className="flex min-w-0 flex-1 basis-56 flex-col gap-1">
              <h2 className="text-title">{STEPS[step]}</h2>
              <p className="text-sm text-muted-foreground">
                {step === 0 &&
                  'Выбери самое близкое описание. Точнее оценю по упражнениям, а сменить уровень можно в любой момент.'}
                {step === 1 && 'Цель и необязательная дата: от них зависит прогноз.'}
                {step === 2 && 'Минуты в день и часовой пояс: по нему считаются твои дни.'}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-2 pt-2 text-sm text-muted-foreground">
              <span className="flex gap-1" aria-hidden>
                {STEPS.map((_, i) => (
                  <span
                    key={i}
                    className={cn('h-1.5 w-6 rounded-full', i < step ? 'bg-grass' : i === step ? 'bg-sky' : 'bg-muted')}
                  />
                ))}
              </span>
              Шаг {step + 1} из {STEPS.length}
            </div>
          </header>
        )}

        {showLevel && (
          <LevelPicker
            legend="Текущий уровень"
            hideLegend={isOnboarding}
            compact={!isOnboarding}
            name="current"
            value={current}
            onChange={chooseCurrent}
          />
        )}

        {showGoal && (
          <>
            <LevelPicker
              legend="Цель"
              hideLegend={isOnboarding}
              compact={!isOnboarding}
              name="target"
              value={target}
              onChange={setTarget}
              minLevel={current}
            />
            <div className="flex flex-col gap-2">
              <Label htmlFor="target-date">Дата цели (необязательно)</Label>
              <div className="flex gap-2">
                <Input
                  id="target-date"
                  type="date"
                  value={targetDate}
                  min={isoDay(today)}
                  max={isoDay(new Date(today.getFullYear() + GOAL_YEARS, today.getMonth(), today.getDate()))}
                  onChange={(e) => setTargetDate(e.target.value)}
                  className="max-w-56"
                />
                {targetDate && (
                  <Button type="button" variant="ghost" onClick={() => setTargetDate('')}>
                    Убрать дату
                  </Button>
                )}
              </div>
              <p className="text-sm text-muted-foreground">
                Без даты просто покажу, сколько это займёт. С датой — сколько минут в день нужно, чтобы успеть.
              </p>
            </div>
          </>
        )}

        {showPace && (
          <>
            <div role="group" aria-label="Минут в день" className="flex flex-col gap-2">
              <span className="text-sm font-semibold">Минут в день</span>
              <div className="flex flex-wrap gap-2">
                {MINUTE_PRESETS.map((m) => (
                  <Button
                    key={m}
                    type="button"
                    variant="secondary"
                    aria-pressed={minutesValid && minutes === m}
                    onClick={() => setMinutesText(String(m))}
                    className="aria-pressed:border-foreground aria-pressed:bg-muted"
                  >
                    {m}
                  </Button>
                ))}
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="minutes">Или своё число минут в день (5–240)</Label>
              <Input
                id="minutes"
                type="number"
                inputMode="numeric"
                min={MIN_MINUTES}
                max={MAX_MINUTES}
                value={minutesText}
                aria-invalid={!minutesValid}
                onChange={(e) => setMinutesText(e.target.value)}
                className="max-w-32"
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="timezone">Часовой пояс</Label>
              <select
                id="timezone"
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                className="h-11 w-full max-w-sm rounded-lg border border-input bg-card px-3 text-base text-card-foreground"
              >
                {zones.map((z) => (
                  <option key={z} value={z}>
                    {z}
                  </option>
                ))}
              </select>
            </div>
          </>
        )}

        {error && (
          <div
            id={errorId}
            role="alert"
            className="rounded-lg border border-coral bg-card p-3 text-sm font-semibold text-coral-text"
          >
            {error}
          </div>
        )}

        <div className="flex flex-wrap items-center gap-3">
          {isOnboarding && step > 0 && (
            <Button type="button" variant="secondary" onClick={() => setStep(step - 1)}>
              Назад
            </Button>
          )}
          {isOnboarding && !lastStep ? (
            <Button type="submit" disabled={step === 0 && !current}>
              Дальше
            </Button>
          ) : (
            <Button type="submit" size="lg" disabled={!canSave || save.isPending}>
              {save.isPending ? 'Сохраняем…' : isOnboarding ? 'Начать' : 'Сохранить'}
            </Button>
          )}
        </div>
      </form>

      <div className="lg:sticky lg:top-6">
        <ForecastCounter
          current={current}
          target={target}
          minutesPerDay={minutesValid ? minutes : null}
          targetDate={targetDate}
          now={today}
        />
      </div>
    </div>
  )
}
