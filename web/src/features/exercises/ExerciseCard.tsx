import { Fox } from '@/components/brand/Fox'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Check, X } from 'lucide-react'
import { Navigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { meKey } from '@/features/auth/auth'
import { cn } from '@/lib/utils'
import { useAnswer, type AnswerResult, type Exercise } from './exercises'
import { topicName } from './topics'
import { AddToCards } from './AddToCards'
import { MatchExercise } from './MatchExercise'
import { lastInput, prefersReducedMotion } from './matchFx'
import { OrderExercise } from './OrderExercise'

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

/** Что с ошибкой после ответа: верный повтор закрывает её со второго раза, неверный ответ (новый или повторный) вернёт её завтра. */
function mistakeLine(correct: boolean, m: NonNullable<AnswerResult['mistake']>): string {
  if (!correct) return 'Вернётся завтра'
  return m.status === 'closed' ? 'Ошибка закрыта' : 'Ещё один верный ответ, и ошибка закрыта'
}

/** Сколько пар «сопоставления» верно. */
function matchRight(r: AnswerResult): number {
  return r.pairs?.filter((p) => p.correct).length ?? 0
}

type ExerciseCardProps = {
  exercise: Exercise
  onNext: () => void
  loadingNext: boolean
  /** Вызывается один раз, когда сервер принял ответ (до показа «Дальше»). */
  onAnswered?: (result: AnswerResult) => void
}

export function ExerciseCard({ exercise, onNext, loadingNext, onAnswered }: ExerciseCardProps) {
  const answer = useAnswer()
  const qc = useQueryClient()
  const [result, setResult] = useState<AnswerResult>()
  const [given, setGiven] = useState<string>()
  const [typed, setTyped] = useState('')
  const keyRef = useRef(newKey())
  const busyRef = useRef(false)
  const articleRef = useRef<HTMLElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const isOrder = exercise.kind === 'order'
  const isMatch = exercise.kind === 'match'
  // Лисёнок подпрыгивает только при пяти из пяти и только если ответ дан мышью и движение не отключено.
  const celebrate = useMemo(() => isMatch && !!result?.correct && lastInput() === 'pointer' && !prefersReducedMotion(), [isMatch, result])
  const manual = !isOrder && !isMatch && exercise.choices.length === 0

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
          onSuccess: (r) => {
            setResult(r)
            onAnswered?.(r)
          },
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
    [answer, exercise.id, result, locked, qc, given, onAnswered],
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
      {exercise.repeat && <p className="text-sm font-semibold text-muted-foreground">Повтор ошибки</p>}
      <p className="text-sm text-muted-foreground">{topicName(exercise.topic)}</p>

      {isOrder || isMatch ? (
        <p lang="ru" className="break-words text-xl leading-relaxed">
          {exercise.prompt}
        </p>
      ) : (
        <div className="flex flex-col gap-3">
          <p lang="en" className="break-words text-xl leading-relaxed">
            <Prompt text={exercise.prompt} answer={result?.answer} />
          </p>
          {exercise.kind === 'form' && exercise.hint && (
            <p lang="en" className="text-muted-foreground">
              <span aria-hidden>({exercise.hint})</span>
              <span className="sr-only" lang="ru">начальная форма: {exercise.hint}</span>
            </p>
          )}
          <p lang="ru" className="break-words text-muted-foreground">{exercise.translation}</p>
        </div>
      )}

      {isOrder && (
        <OrderExercise
          tiles={exercise.tiles ?? []}
          frozen={!!result || answer.isPending || locked !== undefined}
          pending={answer.isPending}
          correct={result?.correct}
          onSubmit={submit}
          restoreFocus={() => articleRef.current?.focus()}
        />
      )}

      {isMatch && (
        <MatchExercise
          words={exercise.words ?? []}
          options={exercise.options ?? []}
          frozen={!!result || answer.isPending || locked !== undefined}
          pending={answer.isPending}
          result={result}
          onSubmit={submit}
          restoreFocus={() => articleRef.current?.focus()}
        />
      )}

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

      {!manual && !isOrder && !isMatch && (
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
                  'pixel-frame pixel-press flex min-h-12 items-center gap-3 bg-card px-4 py-2 text-left font-semibold transition-colors',
                  'hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring disabled:cursor-default',
                  !result && '[--frame:var(--input)]',
                  isRight && '[--frame:var(--grass-text)] bg-muted',
                  isWrong && '[--frame:var(--coral)] bg-muted text-coral-text',
                  result && !isRight && !isWrong && '[--frame:var(--border)] opacity-60',
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
            <Fox
              scale={2}
              mood={result.correct || (isMatch && matchRight(result) >= 4) ? 'happy' : 'thinking'}
              bob={false}
              className={celebrate ? 'fox-jump' : undefined}
            />
            {result.correct ? <Check aria-hidden className="size-5 text-grass-text" /> : <X aria-hidden className="size-5" />}
            {isMatch ? (result.correct ? 'Все пары верны' : 'Есть ошибки в парах') : result.correct ? 'Верно' : 'Неверно'}
          </p>
          {!result.correct && !isMatch && (
            <p>
              {'Верный ответ: '}
              <span lang="en" className="font-semibold">
                {result.answer}
              </span>
            </p>
          )}
          {!isMatch && <p className="text-muted-foreground">{result.translation}</p>}
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
          {result.mistake && (
            <p data-mistake className="text-sm text-muted-foreground">
              {mistakeLine(result.correct, result.mistake)}
            </p>
          )}
          {result.word && <AddToCards word={result.word} />}
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
