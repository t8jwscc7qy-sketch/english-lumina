import { CircleAlert, Layers, Pencil } from 'lucide-react'
import { useEffect, type ReactNode } from 'react'
import { Link } from 'react-router'
import { XpBar } from '@/components/pixel/XpBar'
import { Button } from '@/components/ui/button'
import { useMe } from '@/features/auth/auth'
import { DEMO_TODAY } from './demo'
import { StageMap } from './StageMap'

/** Карточка задачи дня: ссылка на свой раздел. Целиком кликабельна, при наведении «приподнимается». */
function TaskCard({
  to,
  title,
  value,
  hint,
  icon,
  tone = 'default',
}: {
  to: string
  title: string
  value: number
  hint: string
  icon: ReactNode
  tone?: 'default' | 'error'
}) {
  return (
    <Link
      to={to}
      className="block rounded-xl transition-[translate,box-shadow] duration-150 hover:-translate-y-0.5 hover:[--shadow-card:var(--shadow-card-hover)] active:translate-y-0"
    >
      <article className="surface flex h-full flex-col gap-2 p-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
          <span className={tone === 'error' ? 'text-coral-text' : undefined}>{icon}</span>
          <h3 className="font-sans text-sm font-semibold">{title}</h3>
        </div>
        <p className="text-display tabular-nums">{value}</p>
        <p className="text-sm text-muted-foreground">{hint}</p>
      </article>
    </Link>
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
      <header className="flex flex-col gap-2 xl:col-start-1">
        <h1 className="text-title break-words">Привет, {me.data?.login}</h1>
        <p className="text-muted-foreground">
          Цель: {d.current} → {d.target} · {d.minutesPerDay}&nbsp;мин в день
        </p>
        {/* Честная пометка. Жёлтый в этом мире значит «опыт и главное действие», поэтому рамка пунктирная нейтральная. */}
        <p
          role="note"
          className="mt-1 w-fit rounded-lg border border-dashed border-input bg-card px-3 py-1 text-sm font-semibold"
        >
          Демо-данные: настоящие появятся после подключения словаря и упражнений.
        </p>
      </header>

      {/* Серия и главное действие. Ниже xl идут сразу под приветствием, на широком экране — правой колонкой. */}
      <aside
        aria-label="Серия и сессия"
        className="grid grid-cols-[minmax(0,1fr)] gap-4 md:grid-cols-3 xl:col-start-2 xl:row-span-3 xl:row-start-1 xl:flex xl:flex-col xl:gap-6"
      >
        <section className="surface flex flex-col gap-1 p-4">
          <h2 className="font-sans text-sm font-semibold text-muted-foreground">Серия</h2>
          <p className="text-title md:text-display">
            {d.streakDays} {dayWord}
          </p>
        </section>

        <section className="surface flex flex-col gap-3 p-4">
          <h2 className="font-sans text-sm font-semibold text-muted-foreground">
            Минимальный день
          </h2>
          <XpBar
            value={d.minimumDay.done}
            total={d.minimumDay.total}
            label="Минимальный день"
            segments={10}
          />
          <p className="text-sm tabular-nums">
            {`${d.minimumDay.done} из ${d.minimumDay.total} мин`}
          </p>
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

      <section
        aria-labelledby="path-title"
        className="surface flex flex-col gap-6 p-6 xl:col-start-1"
      >
        <h2 id="path-title" className="text-title">
          Твой путь
        </h2>
        <StageMap current={d.current} target={d.target} forecast="collecting" />
        <div className="flex flex-col gap-2">
          <p className="flex justify-between text-sm tabular-nums">
            <span className="font-semibold">Опыт уровня {d.current}</span>
            <span className="text-muted-foreground">
              {d.levelXp.value}&nbsp;из&nbsp;{d.levelXp.total}
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

      <section aria-labelledby="tasks-title" className="grid gap-4 md:grid-cols-3 xl:col-start-1">
        <h2 id="tasks-title" className="sr-only">
          Задачи на сегодня
        </h2>
        <TaskCard
          to="/cards"
          title="Карточки к повторению"
          value={d.cardsDue}
          hint="Слова, которые пора вспомнить"
          icon={<Layers className="size-5" strokeWidth={1.75} aria-hidden />}
        />
        <TaskCard
          to="/exercises"
          title="Упражнения на слабые темы"
          value={d.exercises.count}
          hint={`Тема: ${d.exercises.topic}`}
          icon={<Pencil className="size-5" strokeWidth={1.75} aria-hidden />}
        />
        <TaskCard
          to="/mistakes"
          title="Работа над ошибками"
          value={d.mistakes}
          hint="Вернутся, пока не исчезнут"
          tone="error"
          icon={<CircleAlert className="size-5" strokeWidth={1.75} aria-hidden />}
        />
      </section>
    </div>
  )
}
