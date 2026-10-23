import { LEVELS, type Level } from '@/features/today/levels'
import { cn } from '@/lib/utils'
import { LEVEL_INFO } from './levelInfo'

/**
 * Выбор уровня: группа обычных радиокнопок, поэтому работает с клавиатуры и читается скринридером.
 * Уровни ниже `minLevel` недоступны (цель не бывает ниже текущего уровня).
 */
export function LevelPicker({
  legend,
  hideLegend = false,
  value,
  onChange,
  minLevel,
  name,
}: {
  legend: string
  hideLegend?: boolean
  value: Level | null
  onChange: (level: Level) => void
  minLevel?: Level | null
  name: string
}) {
  const minIndex = minLevel ? LEVELS.indexOf(minLevel) : 0
  return (
    <fieldset className="flex min-w-0 flex-col gap-2">
      <legend className={cn('mb-2 text-sm font-semibold', hideLegend && 'sr-only')}>{legend}</legend>
      {LEVELS.map((level, i) => (
        <label
          key={level}
          className="flex cursor-pointer items-start gap-4 rounded-lg border border-input bg-card p-3 transition-colors hover:bg-muted has-[:checked]:border-foreground has-[:checked]:bg-muted has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-50 has-[:disabled]:hover:bg-card has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring"
        >
          <input
            type="radio"
            name={name}
            value={level}
            checked={value === level}
            disabled={i < minIndex}
            onChange={() => onChange(level)}
            className="peer sr-only"
          />
          <span
            aria-hidden
            className="mt-1 grid size-5 shrink-0 place-items-center rounded-full border-2 border-input after:size-2.5 after:scale-0 after:rounded-full after:bg-foreground after:transition-transform peer-checked:border-foreground peer-checked:after:scale-100"
          />
          <span className="w-9 shrink-0 text-title tabular-nums">{level}</span>
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="font-semibold">{LEVEL_INFO[level].title}</span>
            <span className="text-sm text-muted-foreground">{LEVEL_INFO[level].hint}</span>
          </span>
        </label>
      ))}
    </fieldset>
  )
}
