import { Fragment, useState } from 'react'
import { Check, Volume2, X } from 'lucide-react'
import type { Lesson, LessonDrill } from '@/api/client'
import { Button } from '@/components/ui/button'
import { canSpeak, sayText, speak, stopSpeaking } from '@/features/read/speech'
import { segment } from '@/features/read/segments'
import { cn } from '@/lib/utils'

type Sentence = Lesson['sentences'][number]
type Word = Lesson['words'][number]

/** Кнопка «послушать»: синтез речи браузера; без синтеза кнопки нет. */
export function ListenButton({ text, label }: { text: string; label: string }) {
  if (!canSpeak()) return null
  return (
    <Button
      type="button"
      variant="secondary"
      size="icon-sm"
      onClick={() => sayText(text)}
      aria-label={label}
      title={label}
    >
      <Volume2 className="size-4" />
    </Button>
  )
}

/** Английская фраза, в которой подсвечены формы слова (без HTML из данных: только текст и mark). */
function Marked({ text, forms }: { text: string; forms: Set<string> }) {
  return (
    <>
      {segment(text).map((s, i) =>
        s.word && forms.has(s.key) ? (
          <mark key={i} className="bg-sun/40 px-0.5 font-semibold text-foreground">
            {s.text}
          </mark>
        ) : (
          <Fragment key={i}>{s.text}</Fragment>
        ),
      )}
    </>
  )
}

function formsOf(w: Word): Set<string> {
  return new Set([w.lemma, ...w.forms].map((f) => f.toLowerCase().replace(/’/g, "'")))
}

/** Шаг «новое слово»: слово крупно, перевод, звук и фраза из сегодняшнего текста. */
export function WordStep({ word, index, total }: { word: Word; index: number; total: number }) {
  return (
    <article aria-label={`Новое слово ${word.lemma}`} className="flex flex-col gap-6">
      <p
        className="text-sm font-semibold text-muted-foreground"
        aria-live="polite"
      >{`Слово ${index + 1} из ${total}`}</p>
      <div className="flex flex-wrap items-center gap-4">
        <h2 lang="en" className="text-[44px] leading-[52px] font-bold break-words">
          {word.lemma}
        </h2>
        <ListenButton text={word.lemma} label={`Послушать: ${word.lemma}`} />
      </div>
      <p lang="ru" className="text-xl font-semibold">
        {word.ru}
      </p>
      {word.example && (
        <div className="surface flex flex-col gap-2 p-5">
          <p className="text-sm text-muted-foreground">Из сегодняшнего текста</p>
          <div className="flex items-start gap-3">
            <p lang="en" className="flex-1 text-xl">
              {word.example.speaker && (
                <span className="font-semibold">{`${word.example.speaker}: `}</span>
              )}
              <Marked text={word.example.en} forms={formsOf(word)} />
            </p>
            <ListenButton text={word.example.en} label="Послушать фразу" />
          </div>
          <p lang="ru" className="text-muted-foreground">
            {word.example.ru}
          </p>
        </div>
      )}
    </article>
  )
}

/** Шаг «текст»: диалог крупно; нажатие на слово показывает перевод, у каждой реплики звук и перевод. */
export function TextStep({ lesson }: { lesson: Lesson }) {
  const [picked, setPicked] = useState<{ line: number; key: string } | null>(null)
  const [shown, setShown] = useState<Set<number>>(new Set())
  const [playing, setPlaying] = useState<number | null>(null)
  const newForms = new Set(lesson.words.flatMap((w) => [...formsOf(w)]))

  function playAll() {
    if (playing !== null) {
      stopSpeaking()
      setPlaying(null)
      return
    }
    speak(
      lesson.sentences.map((s) => ({ text: s.en })),
      0.9,
      { onSentence: (i) => setPlaying(i), onEnd: () => setPlaying(null) },
    )
  }

  function toggle(i: number) {
    setShown((prev) => {
      const next = new Set(prev)
      if (next.has(i)) next.delete(i)
      else next.add(i)
      return next
    })
  }

  return (
    <article aria-label={`Текст: ${lesson.title}`} className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-title">{lesson.title}</h2>
        {canSpeak() && (
          <Button type="button" variant="secondary" size="sm" onClick={playAll}>
            <Volume2 className="size-4" />
            {playing !== null ? 'Остановить' : 'Послушать весь текст'}
          </Button>
        )}
      </div>
      <p className="text-sm text-muted-foreground">
        Нажми на слово, чтобы увидеть перевод. Новые слова подчёркнуты.
      </p>
      <ol className="flex flex-col gap-4">
        {lesson.sentences.map((s: Sentence, i) => {
          const pick = picked?.line === i ? picked.key : null
          const translation = pick ? (lesson.glossary[pick] ?? (isName(lesson, pick) ? 'имя' : null)) : null
          return (
            <li
              key={i}
              className={cn(
                'surface flex flex-col gap-2 p-4',
                playing === i && '[--frame:var(--sun)]',
              )}
            >
              <div className="flex items-start gap-3">
                <p lang="en" className="flex-1 text-[22px] leading-8">
                  {s.speaker && (
                    <span className="mr-1 font-semibold text-muted-foreground">{`${s.speaker}:`}</span>
                  )}
                  {segment(s.en).map((seg, j) =>
                    seg.word ? (
                      <button
                        key={j}
                        type="button"
                        aria-pressed={pick === seg.key}
                        onClick={() =>
                          setPicked(pick === seg.key ? null : { line: i, key: seg.key })
                        }
                        className={cn(
                          'cursor-pointer hover:bg-muted',
                          newForms.has(seg.key) &&
                            'underline decoration-sun decoration-[3px] underline-offset-4',
                          pick === seg.key && 'bg-sun/40',
                        )}
                      >
                        {seg.text}
                      </button>
                    ) : (
                      <Fragment key={j}>{seg.text}</Fragment>
                    ),
                  )}
                </p>
                <ListenButton text={s.en} label={`Послушать реплику ${i + 1}`} />
              </div>
              {pick && (
                <p aria-live="polite" className="text-base">
                  <span lang="en" className="font-semibold">
                    {pick}
                  </span>
                  {` — ${translation ?? 'перевода пока нет'}`}
                </p>
              )}
              {shown.has(i) ? (
                <p lang="ru" className="text-muted-foreground">
                  {s.ru}
                </p>
              ) : null}
              <button
                type="button"
                onClick={() => toggle(i)}
                className="w-fit cursor-pointer text-sm text-muted-foreground underline underline-offset-4 hover:text-foreground"
                aria-expanded={shown.has(i)}
              >
                {shown.has(i) ? 'Скрыть перевод' : 'Перевод'}
              </button>
            </li>
          )
        })}
      </ol>
    </article>
  )
}

function isName(lesson: Lesson, key: string) {
  return lesson.sentences.some((s) =>
    segment(s.en).some(
      (seg) =>
        seg.key === key && /^[A-Z]/.test(seg.text) && seg.text !== 'I' && !lesson.glossary[key],
    ),
  )
}

/** Варианты ответа плитками: цифры 1–4 выбирают, после проверки верный и выбранный неверный отмечены значком. */
export function Choices({
  options,
  selected,
  answer,
  checked,
  onSelect,
  lang,
}: {
  options: string[]
  selected: string | null
  answer: string
  checked: boolean
  onSelect: (o: string) => void
  lang?: string
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {options.map((o, i) => {
        const isAnswer = checked && o === answer
        const isWrong = checked && o === selected && o !== answer
        return (
          <button
            key={o}
            type="button"
            data-choice
            disabled={checked}
            aria-pressed={selected === o}
            aria-keyshortcuts={String(i + 1)}
            onClick={() => onSelect(o)}
            className={cn(
              'pixel-frame pixel-press flex min-h-14 cursor-pointer items-center gap-3 bg-card px-4 py-3 text-left text-lg [--frame:var(--input)] hover:bg-muted disabled:cursor-default',
              selected === o && !checked && 'bg-sun/30 [--frame:var(--pixel-ink)]',
              isAnswer && '[--frame:var(--grass-text)]',
              isWrong && 'text-coral-text [--frame:var(--coral)]',
              checked && !isAnswer && !isWrong && 'opacity-60',
            )}
          >
            <kbd
              aria-hidden
              className="grid size-6 place-items-center border border-input text-xs text-muted-foreground"
            >
              {i + 1}
            </kbd>
            <span lang={lang} className="flex-1">
              {o}
            </span>
            {isAnswer && <Check aria-label="верно" className="size-5 text-grass-text" />}
            {isWrong && <X aria-label="твой ответ" className="size-5" />}
          </button>
        )
      })}
    </div>
  )
}

/** Предложение с пропуском: до ответа плашка, после — выбранное или верное слово. */
export function GapSentence({ drill, fill }: { drill: LessonDrill; fill: string | null }) {
  const [before, after] = (drill.text ?? '').split('___')
  return (
    <p lang="en" className="text-[26px] leading-9">
      {before}
      {fill ? (
        <mark className="bg-sun/40 px-1 font-semibold text-foreground">{fill}</mark>
      ) : (
        <span
          role="img"
          aria-label="пропуск"
          className="inline-block min-w-20 border-b-4 border-input align-baseline"
        >
          &nbsp;
        </span>
      )}
      {after}
    </p>
  )
}

/** Сборка предложения из плиток: нажатие переносит плитку в ответ и обратно. */
export function OrderBoard({
  tiles,
  chosen,
  checked,
  onChange,
}: {
  tiles: string[]
  chosen: number[]
  checked: boolean
  onChange: (next: number[]) => void
}) {
  return (
    <div className="flex flex-col gap-5">
      <div
        aria-label="Твой ответ"
        className={cn(
          'flex min-h-16 flex-wrap items-center gap-2 border-b-2 border-input pb-3',
          chosen.length === 0 && 'text-muted-foreground',
        )}
      >
        {chosen.length === 0 && <span>Нажимай на слова ниже</span>}
        {chosen.map((t, i) => (
          <button
            key={`${t}-${i}`}
            type="button"
            lang="en"
            disabled={checked}
            onClick={() => onChange(chosen.filter((_, j) => j !== i))}
            aria-label={`убрать ${tiles[t]}`}
            className="pixel-frame pixel-press min-h-11 cursor-pointer bg-card px-3 text-lg [--frame:var(--input)] hover:bg-muted"
          >
            {tiles[t]}
          </button>
        ))}
      </div>
      <div className="flex flex-wrap gap-2">
        {tiles.map((t, i) =>
          chosen.includes(i) ? (
            <span
              key={i}
              aria-hidden
              className="min-h-11 border border-dashed border-input px-3 text-lg text-transparent"
            >
              {t}
            </span>
          ) : (
            <button
              key={i}
              type="button"
              lang="en"
              disabled={checked}
              onClick={() => onChange([...chosen, i])}
              className="pixel-frame pixel-press min-h-11 cursor-pointer bg-card px-3 text-lg [--frame:var(--input)] hover:bg-muted"
            >
              {t}
            </button>
          ),
        )}
      </div>
    </div>
  )
}
