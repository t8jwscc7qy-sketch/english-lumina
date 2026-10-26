import { useEffect } from 'react'
import { Link } from 'react-router'
import { Check, Lock } from 'lucide-react'
import type { CourseLessonRef, CoursePath } from '@/api/client'
import { Fox } from '@/components/brand/Fox'
import { FoxNote } from '@/components/brand/FoxNote'
import { XpBar } from '@/components/pixel/XpBar'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useCourse } from './course'
import { wordsWord } from './lessonFlow'

/** Строка урока на дороге: квадрат-веха с номером (галочка у пройденного, замок у закрытого) и название. */
function LessonRow({ lesson, last }: { lesson: CourseLessonRef; last: boolean }) {
  const done = lesson.state === 'done'
  const current = lesson.state === 'current'
  const locked = lesson.state === 'locked'
  const stateText = done ? 'Пройден' : current ? 'Сейчас' : 'Закрыт'
  const node = (
    <span
      aria-hidden
      className={cn(
        'relative z-10 grid size-14 shrink-0 place-items-center text-xl font-bold tabular-nums',
        done && 'pixel-frame bg-grass text-ink',
        current && 'pixel-frame bg-sky text-ink [--frame:var(--pixel-ink)]',
        locked && 'border-2 border-dashed border-input bg-muted text-muted-foreground',
      )}
    >
      {done ? (
        <Check className="size-6" strokeWidth={3} />
      ) : locked ? (
        <Lock className="size-5" />
      ) : (
        lesson.number
      )}
    </span>
  )
  const body = (
    <span className="flex min-w-0 flex-col">
      <span className={cn('text-lg font-semibold break-words', locked && 'text-muted-foreground')}>
        {`${lesson.number}. ${lesson.title}`}
      </span>
      <span className="text-sm text-muted-foreground">
        {`${lesson.newWords} ${wordsWord(lesson.newWords)} · ${stateText}`}
      </span>
    </span>
  )
  return (
    <li className="relative flex items-center gap-4 pb-6 last:pb-0">
      {/* Дорога между вехами: пройденная — трава, впереди — пунктир. */}
      {!last && (
        <span
          aria-hidden
          className={cn(
            'absolute top-14 left-[26px] h-[calc(100%-56px)] w-1',
            done ? 'bg-grass' : 'bg-[repeating-linear-gradient(to_bottom,var(--color-input)_0_4px,transparent_4px_8px)]',
          )}
        />
      )}
      {locked ? (
        <div
          className="flex items-center gap-4"
          aria-label={`Урок ${lesson.number}: ${lesson.title}, закрыт`}
        >
          {node}
          {body}
        </div>
      ) : (
        <Link
          to={current ? '/lesson' : `/lesson?id=${lesson.id}`}
          className="group flex min-w-0 items-center gap-4 rounded-none py-1 pr-3 hover:bg-muted"
          aria-label={`Урок ${lesson.number}: ${lesson.title}, ${done ? 'пройден, пройти ещё раз' : 'начать'}`}
        >
          {node}
          {body}
        </Link>
      )}
    </li>
  )
}

function UnitSection({ unit }: { unit: CoursePath['units'][number] }) {
  const done = unit.lessons.filter((l) => l.state === 'done').length
  return (
    <section aria-labelledby={`unit-${unit.unit}`} className="surface flex flex-col gap-5 p-6">
      <div className="flex flex-col gap-2">
        <h2 id={`unit-${unit.unit}`} className="text-title">
          {`Блок ${unit.unit}. ${unit.title}`}
        </h2>
        <div className="flex items-center gap-3">
          <XpBar
            value={done}
            total={unit.lessons.length}
            segments={unit.lessons.length}
            label={`Блок ${unit.unit}: пройдено уроков`}
            className="w-40"
          />
          <span className="text-sm text-muted-foreground tabular-nums">{`${done} из ${unit.lessons.length}`}</span>
        </div>
      </div>
      <ol className="flex flex-col">
        {unit.lessons.map((l, i) => (
          <LessonRow key={l.id} lesson={l} last={i === unit.lessons.length - 1} />
        ))}
      </ol>
    </section>
  )
}

/** Главный экран курса: следующий урок крупно с кнопкой «Продолжить», ниже дорога уроков по блокам. */
export function PathPage() {
  const course = useCourse()

  useEffect(() => {
    document.title = 'Путь — Verst'
  }, [])

  if (course.isPending)
    return (
      <p role="status" className="p-6">
        Загружаем путь…
      </p>
    )
  if (course.isError) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-3 p-6">
        <p role="alert" className="text-coral-text">
          {course.error.message}
        </p>
        <Button variant="secondary" className="w-fit" onClick={() => course.refetch()}>
          Повторить
        </Button>
      </div>
    )
  }
  const path = course.data
  const next = path.next
  const started = path.units.some((u) => u.lessons.some((l) => l.state === 'done'))

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 p-6 xl:p-8">
      <h1 className="sr-only">Путь</h1>
      {next ? (
        <section
          aria-label="Следующий урок"
          className="surface flex flex-col gap-5 p-6 sm:flex-row sm:items-center"
        >
          <Fox scale={4} mood={started ? 'happy' : 'neutral'} className="shrink-0 self-center" />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <p className="text-title break-words">{`Урок ${next.number}: ${next.title}`}</p>
            <p className="text-muted-foreground">{`${next.newWords} ${wordsWord(next.newWords)} · текст · около 10 минут`}</p>
          </div>
          <Button asChild size="lg" className="sm:self-center">
            <Link to="/lesson">{started ? 'Продолжить' : 'Начать'}</Link>
          </Button>
        </section>
      ) : (
        <section aria-label="Курс пройден" className="surface p-6">
          <FoxNote mood="happy">
            <p className="text-title">Все уроки пройдены</p>
            <p className="text-muted-foreground">
              Новые уроки скоро появятся. Можно повторить любой пройденный.
            </p>
          </FoxNote>
        </section>
      )}
      {path.units.map((u) => (
        <UnitSection key={u.unit} unit={u} />
      ))}
    </div>
  )
}
