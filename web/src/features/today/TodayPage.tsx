import { useEffect, type ReactNode } from 'react'
import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { XpBar } from '@/components/pixel/XpBar'
import { icons } from '@/components/pixel/icons'
import { Button } from '@/components/ui/button'
import { useMe } from '@/features/auth/auth'
import { DEMO_TODAY } from './demo'
import { StageMap } from './StageMap'

function TaskCard({
  title,
  value,
  hint,
  icon,
  tone = 'default',
}: {
  title: string
  value: number
  hint: string
  icon: ReactNode
  tone?: 'default' | 'error'
}) {
  return (
    <article className="pixel-box flex flex-col gap-2 p-4">
      <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
        <span className={tone === 'error' ? 'text-coral-text' : undefined}>{icon}</span>
        <h3 className="font-sans text-sm font-semibold">{title}</h3>
      </div>
      <p className="font-pixel text-display">{value}</p>
      <p className="text-sm text-muted-foreground">{hint}</p>
    </article>
  )
}

export function TodayPage() {
  const me = useMe()
  const d = DEMO_TODAY

  useEffect(() => {
    document.title = 'Сегодня — Verst'
  }, [])

  const dayWord = d.streakDays === 1 ? 'день' : d.streakDays < 5 ? 'дня' : 'дней'

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-8 p-6 xl:grid-cols-[minmax(0,1fr)_20rem] xl:p-8">
      <div className="flex min-w-0 flex-col gap-8">
        <header className="flex flex-col gap-2">
          <h1 className="font-pixel text-title break-words md:text-display">
            Привет, {me.data?.login}
          </h1>
          <p className="text-muted-foreground">
            Цель: {d.current} → {d.target} · {d.minutesPerDay} мин в день
          </p>
          <p
            role="note"
            className="mt-1 w-fit border-4 border-sun bg-card px-3 py-1 text-sm font-semibold"
          >
            Демо-данные: настоящие появятся после подключения словаря и упражнений.
          </p>
        </header>

        <section aria-labelledby="path-title" className="pixel-box flex flex-col gap-6 p-6">
          <h2 id="path-title" className="font-pixel text-title">
            Твой путь
          </h2>
          <StageMap current={d.current} target={d.target} forecast="collecting" />
          <div className="flex flex-col gap-2">
            <p className="flex justify-between text-sm">
              <span className="font-semibold">Опыт уровня {d.current}</span>
              <span className="text-muted-foreground">
                {d.levelXp.value} из {d.levelXp.total}
              </span>
            </p>
            <XpBar
              value={d.levelXp.value}
              total={d.levelXp.total}
              label={`Опыт уровня ${d.current}`}
              segments={20}
            />
          </div>
        </section>

        <section aria-labelledby="tasks-title" className="grid gap-4 md:grid-cols-3">
          <h2 id="tasks-title" className="sr-only">
            Задачи на сегодня
          </h2>
          <TaskCard
            title="Карточки к повторению"
            value={d.cardsDue}
            hint="Слова, которые пора вспомнить"
            icon={<PixelArtSvg art={icons.cards} scale={3} />}
          />
          <TaskCard
            title="Упражнения на слабые темы"
            value={d.exercises.count}
            hint={`Тема: ${d.exercises.topic}`}
            icon={<PixelArtSvg art={icons.exercises} scale={3} />}
          />
          <TaskCard
            title="Работа над ошибками"
            value={d.mistakes}
            hint="Вернутся, пока не исчезнут"
            tone="error"
            icon={<PixelArtSvg art={icons.mistakes} scale={3} />}
          />
        </section>
      </div>

      <aside className="flex flex-col gap-6">
        <section className="pixel-box flex flex-col gap-1 p-4">
          <h2 className="font-sans text-sm font-semibold text-muted-foreground">Серия</h2>
          <p className="font-pixel text-title md:text-display">
            {d.streakDays} {dayWord}
          </p>
        </section>

        <section className="pixel-box flex flex-col gap-3 p-4">
          <h2 className="font-sans text-sm font-semibold text-muted-foreground">
            Минимальный день
          </h2>
          <XpBar
            value={d.minimumDay.done}
            total={d.minimumDay.total}
            label="Минимальный день"
            segments={10}
          />
          <p className="text-sm">{`${d.minimumDay.done} из ${d.minimumDay.total} мин`}</p>
        </section>

        <div className="flex flex-col gap-2">
          <Button size="lg" disabled aria-describedby="start-hint">
            Начать сессию
          </Button>
          <p id="start-hint" className="text-sm text-muted-foreground">
            Словарь и упражнения ещё не подключены, поэтому сессию начать пока нельзя.
          </p>
        </div>
      </aside>
    </div>
  )
}
