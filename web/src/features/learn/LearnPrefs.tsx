import { useState } from 'react'
import type { LearnPreferences } from '@/api/client'
import { Button } from '@/components/ui/button'
import { useLearnPreferences, useSaveLearnPreferences } from './learn'

const MIN_COVERAGE = 90
const MAX_COVERAGE = 99

function Form({ initial }: { initial: LearnPreferences }) {
  const save = useSaveLearnPreferences()
  const [mode, setMode] = useState(initial.newWordsMode)
  const [perDay, setPerDay] = useState(String(initial.newWordsPerDay))
  const [coverage, setCoverage] = useState(String(Math.round(initial.campCoverage * 100)))

  const perDayN = Number(perDay)
  const coverageN = Number(coverage)
  const perDayOk = Number.isInteger(perDayN) && perDayN >= 1 && perDayN <= 30
  const coverageOk = Number.isInteger(coverageN) && coverageN >= MIN_COVERAGE && coverageN <= MAX_COVERAGE

  return (
    <form
      className="surface flex flex-col gap-5 p-6"
      onSubmit={(e) => {
        e.preventDefault()
        if (!perDayOk || !coverageOk || save.isPending) return
        save.mutate({ newWordsMode: mode, newWordsPerDay: perDayN, campCoverage: coverageN / 100 })
      }}
    >
      <h2 className="text-title">Обучение</h2>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 font-semibold">Сколько новых слов в день</legend>
        <label className="flex items-center gap-2">
          <input type="radio" name="mode" checked={mode === 'auto'} onChange={() => setMode('auto')} className="accent-primary" />
          Подбирать самому по моим минутам и повторениям
        </label>
        <label className="flex items-center gap-2">
          <input type="radio" name="mode" checked={mode === 'manual'} onChange={() => setMode('manual')} className="accent-primary" />
          Столько, сколько я укажу
        </label>
      </fieldset>

      <div className="flex flex-col gap-1">
        <label htmlFor="learn-per-day" className="font-semibold">
          {mode === 'auto' ? 'Не больше слов в день' : 'Слов в день'}
        </label>
        <input
          id="learn-per-day"
          inputMode="numeric"
          value={perDay}
          onChange={(e) => setPerDay(e.target.value)}
          aria-invalid={!perDayOk}
          aria-describedby="learn-per-day-hint"
          className="h-10 w-28 border border-input bg-card px-3 tabular-nums"
        />
        <p id="learn-per-day-hint" className={perDayOk ? 'text-sm text-muted-foreground' : 'text-sm text-coral-text'}>
          От 1 до 30.
        </p>
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="learn-coverage" className="font-semibold">Покрытие текста для режима «Привал», %</label>
        <input
          id="learn-coverage"
          inputMode="numeric"
          value={coverage}
          onChange={(e) => setCoverage(e.target.value)}
          aria-invalid={!coverageOk}
          aria-describedby="learn-coverage-hint"
          className="h-10 w-28 border border-input bg-card px-3 tabular-nums"
        />
        <p id="learn-coverage-hint" className={coverageOk ? 'text-sm text-muted-foreground' : 'text-sm text-coral-text'}>
          От 90 до 99. Упражнения будут брать только изученные слова.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={!perDayOk || !coverageOk || save.isPending}>
          {save.isPending ? 'Сохраняем…' : 'Применить'}
        </Button>
        {save.isSuccess && <p role="status" className="text-sm text-muted-foreground">Сохранено</p>}
      </div>
      {save.isError && (
        <p role="alert" className="rounded-lg border border-coral bg-card p-3 text-sm font-semibold text-coral-text">
          {save.error.message}
        </p>
      )}
    </form>
  )
}

/** Блок «Обучение» в настройках: темп новых слов и покрытие. Пока настройки не загрузились, блока нет. */
export function LearnPrefs() {
  const prefs = useLearnPreferences()
  if (prefs.isPending) return <p role="status">Загружаем настройки обучения…</p>
  if (!prefs.data) return null
  return <Form initial={prefs.data} />
}
