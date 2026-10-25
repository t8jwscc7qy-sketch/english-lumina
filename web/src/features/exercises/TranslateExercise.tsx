import { useEffect, useId, useRef, useState } from 'react'
import { Check, X } from 'lucide-react'
import { Fox } from '@/components/brand/Fox'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { AnswerResult } from './exercises'
import { sparkOffsets, trackInput } from './matchFx'
import { diffOneEdit, lightNorm } from './translateDiff'

trackInput()

/** Не выше четырёх строк, дальше поле прокручивается. */
const MAX_ROWS = 4

type Props = {
  /** Русское предложение, которое нужно перевести. */
  prompt: string
  /** Ответ нельзя менять: идёт запрос, получен результат или сбой оставил ответ на повтор. */
  frozen: boolean
  pending: boolean
  /** Результат ответа; пока его нет, поле не окрашено. */
  result?: AnswerResult
  /** Анимации включены: ответ дан мышью или касанием и «уменьшить движение» не выбрано. */
  motion: boolean
  onSubmit: (given: string) => void
  /** Что ввёл пользователь (если ответ уже отправлен): поле показывает это же значение. */
  given?: string
}

/** Реплика лисёнка: рамка с хвостиком, внутри русская фраза. */
function Bubble({ children }: { children: string }) {
  return (
    <div className="relative min-w-0 flex-1">
      <p
        lang="ru"
        className="pixel-frame break-words bg-card px-4 py-3 text-xl leading-relaxed [--frame:var(--input)]"
      >
        {children}
      </p>
      <svg
        aria-hidden
        width="12"
        height="14"
        viewBox="0 0 12 14"
        shapeRendering="crispEdges"
        className="absolute -left-2.5 top-3 text-input"
      >
        <path d="M9 1 L1 7 L9 13 H12 V1 Z" className="fill-card" />
        <path d="M9 1 L1 7 L9 13" fill="none" stroke="currentColor" strokeWidth="2" />
      </svg>
    </div>
  )
}

/** «Перевод»: русская фраза в реплике лисёнка и поле для английского перевода. */
export function TranslateExercise({
  prompt,
  frozen,
  pending,
  result,
  motion,
  onSubmit,
  given,
}: Props) {
  const id = useId()
  const [text, setText] = useState(given ?? '')
  const ref = useRef<HTMLTextAreaElement>(null)
  const trimmed = text.trim()

  // Фокус в поле сразу: ответ печатают, не нажимая на него.
  useEffect(() => {
    if (!frozen) ref.current?.focus()
  }, [frozen])

  // Поле растёт вместе с текстом до MAX_ROWS строк.
  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    const line = parseFloat(getComputedStyle(el).lineHeight) || 24
    const pad = parseFloat(getComputedStyle(el).paddingTop) * 2 || 0
    el.style.height = `${Math.min(el.scrollHeight, line * MAX_ROWS + pad)}px`
  }, [text])

  function submit() {
    if (!trimmed || frozen || pending) return
    onSubmit(trimmed)
  }

  const answered = result !== undefined
  const right = result?.correct === true
  const sparks = motion && right ? sparkOffsets(trimmed.length, 4) : []

  return (
    <div data-motion={motion ? 'on' : 'off'} className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground">Переведи на английский</p>
        <div className="flex items-start gap-4">
          <Fox
            scale={2}
            mood={!answered && trimmed ? 'thinking' : 'neutral'}
            bob={false}
            className="mt-1 shrink-0"
          />
          <Bubble>{prompt}</Bubble>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <label htmlFor={id} className="text-sm font-semibold">
          Твой перевод
        </label>
        <div className={cn('relative', motion && answered && !right && 'm-wrong')}>
          <textarea
            id={id}
            ref={ref}
            rows={2}
            value={text}
            readOnly={frozen}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault()
                submit()
              } else if (e.key === 'Escape' && !frozen) {
                setText('')
              }
            }}
            maxLength={200}
            lang="en"
            spellCheck={false}
            autoCapitalize="off"
            autoCorrect="off"
            autoComplete="off"
            placeholder="Напиши по-английски"
            aria-keyshortcuts="Enter Escape"
            className={cn(
              'pixel-frame block w-full resize-none bg-card px-4 py-3 text-lg leading-snug placeholder:text-muted-foreground read-only:cursor-default',
              'focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring',
              !answered && '[--frame:var(--input)]',
              answered && right && '[--frame:var(--grass-text)]',
              answered && !right && '[--frame:var(--coral)]',
              answered && 'pr-12',
            )}
          />
          {answered && (
            <span aria-hidden className="pointer-events-none absolute right-4 top-3.5">
              {right ? (
                <Check className="size-5 text-grass-text" strokeWidth={2} />
              ) : (
                <X className="size-5 text-coral-text" strokeWidth={2} />
              )}
              {sparks.map((s, i) => (
                <span
                  key={i}
                  className="m-spark absolute left-2 top-2 size-1 bg-grass-text"
                  style={{ '--dx': `${s.dx}px`, '--dy': `${s.dy}px` } as React.CSSProperties}
                />
              ))}
            </span>
          )}
          {motion && right && (
            <span
              aria-hidden
              className="m-flash pointer-events-none absolute inset-0 bg-grass-text"
            />
          )}
        </div>
        {!frozen && (
          <p
            aria-hidden
            className="hidden text-xs text-muted-foreground [@media(hover:hover)]:block"
          >
            <kbd className="rounded-md border border-border px-1.5 font-sans">Enter</kbd> проверить
            · <kbd className="rounded-md border border-border px-1.5 font-sans">Esc</kbd> очистить ·{' '}
            <kbd className="rounded-md border border-border px-1.5 font-sans">Shift+Enter</kbd>{' '}
            новая строка
          </p>
        )}
      </div>

      {!frozen || pending ? (
        <div>
          <Button
            type="button"
            disabled={!trimmed || pending || frozen}
            onClick={submit}
            aria-keyshortcuts="Enter"
          >
            Проверить
          </Button>
        </div>
      ) : null}
    </div>
  )
}

/** Разбор после ответа: опечатка с подсвеченной буквой и другие допустимые переводы. Всё выводится как текст. */
export function TranslateDetails({
  given,
  result,
  motion,
}: {
  given: string
  result: AnswerResult
  motion: boolean
}) {
  const shown = lightNorm(given)
  const edit = result.nearMiss ? diffOneEdit(shown, lightNorm(result.nearMiss)) : null
  const chars = [...shown]
  return (
    <>
      {result.nearMiss && (
        <>
          <p>
            {'Твой ответ: '}
            <span lang="en" className="font-semibold">
              {edit ? (
                <>
                  {chars.slice(0, edit.start).join('')}
                  <mark
                    className={cn(
                      'bg-transparent text-inherit underline decoration-sun decoration-2 underline-offset-4',
                      edit.missing !== undefined && 'decoration-dashed',
                      motion && 't-blink',
                    )}
                  >
                    {edit.missing ?? chars.slice(edit.start, edit.end).join('')}
                  </mark>
                  {edit.missing !== undefined && (
                    <span className="sr-only" lang="ru">
                      {' '}
                      (пропущена буква)
                    </span>
                  )}
                  {chars.slice(edit.end).join('')}
                </>
              ) : (
                shown
              )}
            </span>
          </p>
          <p>
            {'Правильно: '}
            <span lang="en" className="font-semibold">
              {result.nearMiss}
            </span>
          </p>
        </>
      )}
      {result.accepted && result.accepted.length > 0 && (
        <div>
          <p className="text-muted-foreground">Так тоже можно:</p>
          <ul className="mt-1 flex flex-col gap-1">
            {result.accepted.map((a) => (
              <li key={a} lang="en" className="font-semibold">
                {a}
              </li>
            ))}
          </ul>
        </div>
      )}
    </>
  )
}
