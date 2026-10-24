import { useCallback, useEffect, useRef, useState } from 'react'
import { Check, X } from 'lucide-react'
import { Link, Navigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { cn } from '@/lib/utils'
import { useAnswer, useNextExercise, useTopics, type AnswerResult, type Exercise } from './exercises'
import { topicName } from './topics'
import { WeakTopics } from './WeakTopics'

/** Ключ идемпотентности: новый на каждое упражнение. Повтор с тем же ключом сервер не применяет дважды. */
function newKey(): string {
  return typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function isOtherControl(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement) || t === document.body) return false
  return t.closest('a[href], button, summary, [role="button"], [role="link"], [role="menuitem"], [role="tab"]') !== null
}

function isTypingTarget(t: EventTarget | null): boolean {
  return t instanceof HTMLElement && (t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))
}

const GAP = '_____'

/** Предложение с пропуском: до ответа плашка `_____`, после — верный ответ. Всё выводится как текст. */
function Prompt({ text, answer }: { text: string; answer?: string }) {
  const i = text.indexOf(GAP)
  if (i < 0) return <>{text}</>
  return (
    <>
      {text.slice(0, i)}
      {answer === undefined ? (
        <span role="img" aria-label="пропуск" className="mx-0.5 inline-block rounded-md bg-muted px-2 text-muted-foreground">
          {GAP}
        </span>
      ) : (
        <mark className="mx-0.5 rounded-md bg-muted px-2 font-semibold text-foreground">{answer}</mark>
      )}
      {text.slice(i + GAP.length)}
    </>
  )
}

export function ExercisesPage() {
  const next = useNextExercise()
  const topics = useTopics()
  const qc = useQueryClient()

  useEffect(() => {
    document.title = 'Упражнения — Verst'
  }, [])

  const unauthorized = next.error instanceof ApiError && next.error.code === 'unauthorized'
  useEffect(() => {
    if (unauthorized) qc.setQueryData(meKey, null)
  }, [unauthorized, qc])
  if (unauthorized) return <Navigate to="/login" replace />

  const noExercise = next.error instanceof ApiError && next.error.code === 'no_exercise'

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header>
        <h1 className="text-title">Упражнения</h1>
      </header>

      {next.isPending && <p role="status">Подбираем упражнение…</p>}

      {next.isError && noExercise && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <h2 className="text-title">Подходящих упражнений пока нет</h2>
          <p className="text-muted-foreground">Попробуй позже или поменяй уровень в настройках.</p>
          <Button asChild variant="secondary">
            <Link to="/settings">Настройки</Link>
          </Button>
        </section>
      )}

      {next.isError && !noExercise && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">
            {next.error.message}
          </p>
          <Button onClick={() => next.refetch()}>Повторить</Button>
        </div>
      )}

      {!next.isError && next.data && (
        <ExerciseCard key={`${next.data.id}-${next.dataUpdatedAt}`} exercise={next.data} onNext={() => void next.refetch()} loadingNext={next.isFetching} />
      )}

      {topics.data && <WeakTopics topics={topics.data.topics} />}
    </div>
  )
}

function ExerciseCard({ exercise, onNext, loadingNext }: { exercise: Exercise; onNext: () => void; loadingNext: boolean }) {
  const answer = useAnswer()
  const qc = useQueryClient()
  const [result, setResult] = useState<AnswerResult>()
  const [given, setGiven] = useState<string>()
  const [typed, setTyped] = useState('')
  const keyRef = useRef(newKey())
  const busyRef = useRef(false)
  const articleRef = useRef<HTMLElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const manual = exercise.choices.length === 0

  // Новое упражнение (и результат ответа) забирает фокус: клавиши работают сразу.
  // Фокус остаётся на контейнере и после ответа: тогда 1–4, Пробел и Enter работают без лишних Tab. В ручном вводе до ответа фокус в поле.
  useEffect(() => {
    if (manual && !result) inputRef.current?.focus()
    else articleRef.current?.focus()
  }, [exercise.id, result, manual])

  const unauthorized = answer.error instanceof ApiError && answer.error.code === 'unauthorized'
  // После сбоя без ответа сервера (сеть, 5xx) неизвестно, засчитан ли ответ: менять вариант нельзя, повтор идёт с тем же ключом.
  const locked = answer.isError && answer.error instanceof ApiError && (answer.error.status === 0 || answer.error.status >= 500) ? given : undefined

  const submit = useCallback(
    (value: string) => {
      const text = value.trim()
      if (!text || busyRef.current || result) return // защита от двойного нажатия: пока идёт запрос, новых ответов нет
      if (locked !== undefined && text !== locked) return
      // Сервер отклонил прошлый ответ (4xx) и ничего не засчитал: другой вариант идёт с новым ключом.
      if (answer.isError && given !== text) keyRef.current = newKey()
      busyRef.current = true
      setGiven(text)
      answer.mutate(
        { exerciseId: exercise.id, given: text, key: keyRef.current },
        {
          onSuccess: (r) => setResult(r),
          onError: (e) => {
            // Сначала обнуляем «кто я» (синхронно), потом навигация: иначе защита экрана входа вернёт на главную.
            if (e instanceof ApiError && e.code === 'unauthorized') qc.setQueryData(meKey, null)
          },
          onSettled: () => {
            busyRef.current = false
          },
        },
      )
    },
    [answer, exercise.id, result, locked, qc, given],
  )

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.repeat || e.ctrlKey || e.metaKey || e.altKey || isTypingTarget(e.target)) return
      if (result) {
        if ((e.key === ' ' || e.key === 'Enter') && !isOtherControl(e.target) && !loadingNext) {
          e.preventDefault()
          onNext()
        }
        return
      }
      const n = Number(e.key)
      if (!manual && n >= 1 && n <= exercise.choices.length) {
        e.preventDefault()
        submit(exercise.choices[n - 1])
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [result, manual, exercise.choices, submit, onNext, loadingNext])

  if (unauthorized) return <Navigate to="/login" replace />

  const canSkip = answer.error instanceof ApiError && (answer.error.code === 'already_answered' || answer.error.code === 'not_found')

  return (
    <article ref={articleRef} tabIndex={-1} aria-label="Упражнение" className="surface flex flex-col gap-6 p-6 outline-none">
      <p className="text-sm text-muted-foreground">{topicName(exercise.topic)}</p>

      <div className="flex flex-col gap-3">
        <p lang="en" className="break-words text-xl leading-relaxed">
          <Prompt text={exercise.prompt} answer={result?.answer} />
        </p>
        <p lang="ru" className="break-words text-muted-foreground">{exercise.translation}</p>
      </div>

      {manual && !result && (
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            submit(typed)
          }}
        >
          <label className="flex min-w-48 flex-1 flex-col gap-1.5 text-sm font-semibold">
            Твой ответ
            <input
              ref={inputRef}
              readOnly={locked !== undefined}
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              maxLength={200}
              autoComplete="off"
              spellCheck={false}
              lang="en"
              className="h-11 rounded-xl border border-input bg-card px-4 text-base font-normal"
            />
          </label>
          <Button type="submit" disabled={answer.isPending}>
            Ответить
          </Button>
        </form>
      )}

      {!manual && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {exercise.choices.map((choice, i) => {
            const isRight = !!result && choice === result.answer
            const isWrong = !!result && !result.correct && choice === given
            return (
              <button
                key={choice}
                type="button"
                disabled={!!result || answer.isPending || (locked !== undefined && choice !== locked)}
                onClick={() => submit(choice)}
                aria-keyshortcuts={String(i + 1)}
                className={cn(
                  'flex min-h-12 items-center gap-3 rounded-xl border bg-card px-4 py-2 text-left font-semibold transition-colors',
                  'hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default',
                  !result && 'border-input',
                  isRight && 'border-grass-text bg-muted',
                  isWrong && 'border-coral bg-muted text-coral-text',
                  result && !isRight && !isWrong && 'border-border opacity-60',
                )}
              >
                <kbd aria-hidden className="grid size-6 shrink-0 place-items-center rounded-md border border-border font-sans text-xs">{i + 1}</kbd>{' '}
                <span className="min-w-0 flex-1 break-words">{choice}</span>
                {isRight && (
                  <>
                    <Check aria-hidden className="size-5 shrink-0 text-grass-text" strokeWidth={2} />
                    <span className="sr-only">верно</span>
                  </>
                )}
                {isWrong && (
                  <>
                    <X aria-hidden className="size-5 shrink-0" strokeWidth={2} />
                    <span className="sr-only">твой ответ</span>
                  </>
                )}
              </button>
            )
          })}
        </div>
      )}

      {/* Область живого региона есть всегда и до ответа пуста: тогда скринридер объявит появившийся результат. */}
      <div role="status" aria-live="polite" className={cn(result && 'flex flex-col gap-3 border-t border-border pt-4')}>
        {result && (
          <>
          <p className={cn('flex items-center gap-2 font-semibold', !result.correct && 'text-coral-text')}>
            {result.correct ? <Check aria-hidden className="size-5 text-grass-text" /> : <X aria-hidden className="size-5" />}
            {result.correct ? 'Верно' : 'Неверно'}
          </p>
          {!result.correct && (
            <p>
              {'Верный ответ: '}
              <span lang="en" className="font-semibold">
                {result.answer}
              </span>
            </p>
          )}
          <p className="text-muted-foreground">{result.translation}</p>
          {result.ratings.length > 0 && (
            <div data-affected className="text-sm text-muted-foreground">
              {'Затронуто: '}
              {result.ratings.map((r, i) => (
                <span key={r.topic}>
                  {i > 0 && ', '}
                  <span>{topicName(r.topic)}</span>
                  {' '}
                  <span className="font-semibold tabular-nums text-foreground">{Math.round(r.rating)}</span>
                </span>
              ))}
            </div>
          )}
          </>
        )}
      </div>

      {answer.isError && !unauthorized && (
        <div className="flex flex-wrap items-center gap-3">
          <p role="alert" className="flex-1 rounded-lg border border-coral bg-card p-3 text-sm font-semibold text-coral-text">
            {answer.error.message}
          </p>
          {canSkip && <Button onClick={onNext}>Дальше</Button>}
          {locked !== undefined && (
            <>
              <Button onClick={() => submit(locked)} disabled={answer.isPending}>
                Повторить
              </Button>
              <Button variant="secondary" onClick={onNext} disabled={loadingNext}>
                {loadingNext ? 'Подбираем…' : 'Пропустить'}
              </Button>
            </>
          )}
        </div>
      )}

      {result && (
        <div>
          <Button size="lg" onClick={onNext} disabled={loadingNext} aria-keyshortcuts="Space Enter">
            {loadingNext ? 'Подбираем…' : 'Дальше'}
          </Button>
        </div>
      )}
    </article>
  )
}
