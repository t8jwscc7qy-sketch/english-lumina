import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Check, X } from 'lucide-react'
import type { Lesson } from '@/api/client'
import { Fox } from '@/components/brand/Fox'
import { FoxNote } from '@/components/brand/FoxNote'
import { XpBar } from '@/components/pixel/XpBar'
import { Button } from '@/components/ui/button'
import { stopSpeaking } from '@/features/read/speech'
import { cn } from '@/lib/utils'
import { useCompleteLesson, useLesson } from './course'
import {
  buildSteps,
  lessonMinutes,
  orderCorrect,
  stageLabel,
  withRetry,
  wordsWord,
  type Step,
} from './lessonFlow'
import { Choices, GapSentence, OrderBoard, TextStep, WordStep } from './LessonSteps'
import { MatchBoard } from './MatchBoard'

type Result = { correct: boolean; answer?: string }

/** Итог урока: отправляет «урок пройден» один раз (повтор с тем же ключом безопасен), показывает слова и время. */
function Summary({
  lesson,
  seconds,
  mistakes,
}: {
  lesson: Lesson
  seconds: number
  mistakes: number
}) {
  const complete = useCompleteLesson()
  const [key] = useState(() => crypto.randomUUID())
  const navigate = useNavigate()
  const heading = useRef<HTMLHeadingElement>(null)
  const sent = useRef(false)
  const send = () => complete.mutate({ lessonId: lesson.id, key, seconds, mistakes })
  useEffect(() => {
    heading.current?.focus()
    if (sent.current) return // в StrictMode эффект срабатывает дважды: отправляем один раз
    sent.current = true
    complete.mutate({ lessonId: lesson.id, key, seconds, mistakes })
    // eslint-disable-next-line react-hooks/exhaustive-deps -- отправка ровно один раз при показе итога
  }, [])

  const next = complete.data?.next
  return (
    <article aria-labelledby="summary-title" className="flex flex-col gap-6">
      <FoxNote mood="happy">
        <h2 id="summary-title" ref={heading} tabIndex={-1} className="text-title outline-none">
          {lesson.done ? 'Урок повторён' : 'Урок пройден'}
        </h2>
        <p className="text-muted-foreground">{`${lessonMinutes(seconds)} мин · ошибок: ${mistakes}`}</p>
      </FoxNote>
      <section className="surface flex flex-col gap-3 p-5">
        <h3 className="font-semibold">{`Новые слова: ${lesson.words.length} ${wordsWord(lesson.words.length)}`}</h3>
        <ul className="grid gap-2 sm:grid-cols-2">
          {lesson.words.map((w) => (
            <li key={w.lemma + w.pos} className="flex gap-2">
              <span lang="en" className="font-semibold">
                {w.lemma}
              </span>
              <span className="text-muted-foreground">{`— ${w.ru}`}</span>
            </li>
          ))}
        </ul>
        {!lesson.done && (
          <p className="text-sm text-muted-foreground">
            Эти слова теперь будут возвращаться на повторение.
          </p>
        )}
      </section>
      {lesson.note && (
        <section className="surface flex flex-col gap-2 p-5">
          <h3 lang="en" className="font-semibold">
            {lesson.note.title}
          </h3>
          <p>{lesson.note.text}</p>
        </section>
      )}
      {complete.isError && (
        <div className="flex flex-wrap items-center gap-3">
          <p
            role="alert"
            className="text-coral-text"
          >{`Урок не сохранился: ${complete.error.message}`}</p>
          <Button variant="secondary" size="sm" onClick={send}>
            Повторить
          </Button>
        </div>
      )}
      <div className="flex flex-wrap gap-3">
        {next ? (
          <Button
            size="lg"
            disabled={!complete.isSuccess}
            onClick={() => navigate(`/lesson?id=${next.id}`, { replace: true })}
          >
            {`Следующий урок: ${next.title}`}
          </Button>
        ) : (
          complete.isSuccess && (
            <p className="self-center">Это был последний урок. Новые скоро появятся.</p>
          )
        )}
        <Button asChild variant="secondary" size="lg">
          <Link to="/path">К пути</Link>
        </Button>
      </div>
      <p role="status" className="sr-only">
        {complete.isPending ? 'Сохраняем урок…' : complete.isSuccess ? 'Урок сохранён' : ''}
      </p>
    </article>
  )
}

/** Урок «Пути»: шаги по одному, полоска сверху, кнопка действия снизу. Enter — кнопка, цифры 1–4 — варианты. */
function LessonRun({ lesson }: { lesson: Lesson }) {
  const [steps, setSteps] = useState<Step[]>(() => buildSteps(lesson))
  const [at, setAt] = useState(0)
  const [selected, setSelected] = useState<string | null>(null)
  const [chosen, setChosen] = useState<number[]>([])
  const [result, setResult] = useState<Result | null>(null)
  const [mistakes, setMistakes] = useState(0)
  const [started] = useState(() => Date.now())
  const [finishedSeconds, setFinishedSeconds] = useState<number | null>(null)
  const main = useRef<HTMLDivElement>(null)

  const step = steps[at]
  const drill = step.kind === 'drill' ? lesson.drills[step.index] : null
  const question = step.kind === 'question' ? lesson.questions[step.index] : null
  const needsCheck = question !== null || drill?.kind === 'gap' || drill?.kind === 'order'
  const isMatch = drill?.kind === 'match'
  const ready =
    question || drill?.kind === 'gap'
      ? selected !== null
      : drill?.kind === 'order'
        ? chosen.length > 0
        : true

  useEffect(() => () => stopSpeaking(), [])

  function next() {
    stopSpeaking()
    setSelected(null)
    setChosen([])
    setResult(null)
    const after = at + 1
    if (steps[after]?.kind === 'summary') setFinishedSeconds((Date.now() - started) / 1000)
    setAt(after)
    main.current?.focus()
  }

  function check() {
    let r: Result
    if (question) {
      const answer = question.options[question.answer]
      r = { correct: selected === answer, answer }
    } else if (drill?.kind === 'gap') {
      r = { correct: selected === drill.answer, answer: drill.answer }
    } else if (drill?.kind === 'order') {
      r = {
        correct: orderCorrect(
          chosen.map((i) => drill.tiles?.[i] ?? ''),
          drill.answer ?? '',
        ),
        answer: drill.answer,
      }
    } else return
    setResult(r)
    if (!r.correct) {
      setMistakes((m) => m + 1)
      setSteps((s) => withRetry(s, step))
    }
  }

  const action = !needsCheck || result ? next : check
  const actionLabel =
    needsCheck && !result ? 'Проверить' : steps[at + 1]?.kind === 'summary' ? 'Завершить' : 'Дальше'
  const actionDisabled = (needsCheck && !result && !ready) || (isMatch && !result)

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const t = e.target as HTMLElement
      if (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA') return
      if (
        e.key === 'Enter' &&
        step.kind !== 'summary' &&
        !actionDisabled &&
        t.tagName !== 'BUTTON' &&
        t.tagName !== 'A'
      ) {
        e.preventDefault()
        action()
      }
      const opts = question?.options ?? (drill?.kind === 'gap' ? drill.options : undefined)
      const n = Number(e.key)
      if (opts && !result && n >= 1 && n <= opts.length) setSelected(opts[n - 1])
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const total = steps.length
  return (
    <div className="flex min-h-dvh flex-col">
      <header className="sticky top-0 z-20 border-b border-border bg-background">
        <div className="mx-auto flex w-full max-w-2xl items-center gap-4 px-6 py-3">
          <Button asChild variant="ghost" size="icon-sm" aria-label="Выйти из урока">
            <Link to="/path">
              <X className="size-5" />
            </Link>
          </Button>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <div className="flex justify-between gap-2 text-sm">
              <span className="truncate font-semibold">{`Урок ${lesson.number}. ${lesson.title}`}</span>
              <span className="shrink-0 text-muted-foreground">{stageLabel(step)}</span>
            </div>
            <XpBar
              value={at}
              total={total - 1}
              segments={Math.min(total - 1, 24)}
              label={`Шаг ${at + 1} из ${total}`}
            />
          </div>
        </div>
      </header>

      <div
        ref={main}
        tabIndex={-1}
        className="mx-auto w-full max-w-2xl flex-1 px-6 py-8 outline-none"
      >
        <h1 className="sr-only">{`Урок ${lesson.number}: ${lesson.title}`}</h1>
        {step.kind === 'word' && (
          <WordStep
            key={at}
            word={lesson.words[step.index]}
            index={step.index}
            total={lesson.words.length}
          />
        )}
        {step.kind === 'text' && <TextStep lesson={lesson} />}
        {question && (
          <section key={at} className="flex flex-col gap-6">
            <h2 className="text-title">{question.prompt}</h2>
            <Choices
              options={question.options}
              selected={selected}
              answer={question.options[question.answer]}
              checked={!!result}
              onSelect={setSelected}
            />
          </section>
        )}
        {drill?.kind === 'gap' && (
          <section key={at} className="flex flex-col gap-6">
            <p className="text-sm font-semibold text-muted-foreground">
              {drill.old ? 'Вставь слово (повторяем прошлые уроки)' : 'Вставь слово'}
            </p>
            <div className="flex flex-col gap-2">
              <GapSentence drill={drill} fill={result ? (drill.answer ?? null) : selected} />
              {drill.prompt && (
                <p lang="ru" className="text-lg text-muted-foreground">
                  {drill.prompt}
                </p>
              )}
            </div>
            <Choices
              options={drill.options ?? []}
              selected={selected}
              answer={drill.answer ?? ''}
              checked={!!result}
              onSelect={setSelected}
              lang="en"
            />
          </section>
        )}
        {drill?.kind === 'order' && (
          <section key={at} className="flex flex-col gap-6">
            <p className="text-sm font-semibold text-muted-foreground">Собери предложение</p>
            <p lang="ru" className="text-[26px] leading-9">
              {drill.prompt}
            </p>
            <OrderBoard
              tiles={drill.tiles ?? []}
              chosen={chosen}
              checked={!!result}
              onChange={setChosen}
            />
          </section>
        )}
        {isMatch && drill && (
          <section key={at} className="flex flex-col gap-4">
            <h2 className="text-title">Собери пары</h2>
            <MatchBoard
              drill={drill}
              onDone={(m) => {
                setMistakes((x) => x + m)
                setResult({ correct: m === 0 })
              }}
            />
          </section>
        )}
        {step.kind === 'summary' && (
          <Summary
            lesson={lesson}
            seconds={finishedSeconds ?? 0}
            mistakes={mistakes}
          />
        )}
      </div>

      {step.kind !== 'summary' && (
        <footer
          className={cn(
            'sticky bottom-0 z-20 border-t border-border bg-background',
            result && (result.correct ? 'bg-muted' : 'bg-coral-soft/40'),
          )}
        >
          <div className="mx-auto flex w-full max-w-2xl flex-wrap items-center justify-between gap-4 px-6 py-4">
            <div role="status" aria-label="Результат" className="min-h-6 min-w-0 flex-1">
              {result &&
                (result.correct ? (
                  <p className="flex items-center gap-2 font-semibold text-grass-text">
                    <Fox scale={1} mood="happy" bob={false} />
                    <Check className="size-5" /> {isMatch ? 'Все пары собраны' : 'Верно!'}
                  </p>
                ) : isMatch ? (
                  <p className="font-semibold">Все пары собраны. С ошибками — это нормально.</p>
                ) : (
                  <div className="flex flex-col gap-1">
                    <p className="flex items-center gap-2 font-semibold text-coral-text">
                      <X className="size-5" /> Неверно
                    </p>
                    {result.answer && (
                      <p>
                        {'Правильно: '}
                        <span lang="en" className="font-semibold">
                          {result.answer}
                        </span>
                        {drill ? '. Это задание вернётся в конце урока.' : ''}
                      </p>
                    )}
                  </div>
                ))}
            </div>
            <Button size="lg" onClick={action} disabled={actionDisabled} className="min-w-40">
              {actionLabel}
            </Button>
          </div>
        </footer>
      )}
    </div>
  )
}

/** Страница урока: `?id=` — конкретный (пройденный) урок, без него — текущий. */
export function LessonPage() {
  const [params] = useSearchParams()
  const id = params.get('id')
  const lesson = useLesson(id)

  useEffect(() => {
    document.title = lesson.data ? `${lesson.data.title} — Verst` : 'Урок — Verst'
  }, [lesson.data])

  if (lesson.isPending)
    return (
      <p role="status" className="p-6">
        Загружаем урок…
      </p>
    )
  if (lesson.isError) {
    return (
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-4 p-6">
        <p role="alert" className="text-coral-text">
          {lesson.error.message}
        </p>
        <div className="flex gap-3">
          <Button variant="secondary" onClick={() => lesson.refetch()}>
            Повторить
          </Button>
          <Button asChild variant="secondary">
            <Link to="/path">К пути</Link>
          </Button>
        </div>
      </div>
    )
  }
  return <LessonRun key={lesson.data.id} lesson={lesson.data} />
}
