import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { ApiError, type ReadMode, type Understood } from '@/api/client'
import { FoxNote } from '@/components/brand/FoxNote'
import { Button } from '@/components/ui/button'
import { useNextText, useRateText } from './read'
import { wordAtChar } from './segments'
import { canSpeak, speak, stopSpeaking } from './speech'
import { TextView } from './TextView'
import { WordPanel } from './WordPanel'

const RATES = [0.7, 1, 1.2] as const
const percent = (v: number) => `${Math.round(v * 100)}%`

function Reader({ mode, onNext, loadingNext }: { mode: ReadMode; onNext: () => void; loadingNext: boolean }) {
  const next = useNextText(mode)
  const rate = useRateText()
  const text = next.data
  const [picked, setPicked] = useState<string | null>(null)
  const [revealed, setRevealed] = useState(mode === 'read')
  const [allRu, setAllRu] = useState(false)
  const [speed, setSpeed] = useState<number>(1)
  const [playing, setPlaying] = useState<{ sentence: number; word: number } | null>(null)
  const [talking, setTalking] = useState(false)
  const [answer, setAnswer] = useState<{ understood: Understood; goal: number } | null>(null)
  const rated = useRef(false)

  useEffect(() => () => stopSpeaking(), [])

  const hint = useMemo(() => text?.hints.find((h) => h.token === picked), [text, picked])

  if (!text) return null

  function play(from = 0) {
    if (!text) return
    setTalking(true)
    speak(
      text.sentences.map((s) => s.en),
      speed,
      {
        onSentence: (i) => setPlaying({ sentence: i, word: -1 }),
        onWord: (i, charIndex) => setPlaying({ sentence: i, word: wordAtChar(text.sentences[i].en, charIndex) }),
        onEnd: () => {
          setTalking(false)
          setPlaying(null)
        },
      },
      from,
    )
  }

  function stop() {
    stopSpeaking()
    setTalking(false)
    setPlaying(null)
  }

  function rateIt(understood: Understood) {
    if (!text || rated.current) return
    rated.current = true
    rate.mutate(
      { textId: text.id, understood, mode },
      { onSuccess: (r) => setAnswer({ understood, goal: r.goal }), onError: () => (rated.current = false) },
    )
  }

  return (
    <>
      <div role="status" className="flex flex-wrap items-baseline gap-x-4 gap-y-1 text-sm text-muted-foreground">
        <span className="tabular-nums">{`Понимание этого текста: ${percent(text.coverage)} (цель ${percent(text.goal)})`}</span>
        <span>{`Новые слова: ${text.targets.join(', ')}`}</span>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        {canSpeak() && (
          <>
            {talking ? (
              <Button variant="secondary" onClick={stop}>
                Остановить
              </Button>
            ) : (
              <Button variant="secondary" onClick={() => play(0)}>
                Прослушать
              </Button>
            )}
            <label className="flex items-center gap-2 text-sm">
              Скорость
              <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))} className="h-10 border border-input bg-card px-2">
                {RATES.map((r) => (
                  <option key={r} value={r}>{`${r}×`}</option>
                ))}
              </select>
            </label>
          </>
        )}
        {revealed && (
          <Button variant="secondary" aria-pressed={allRu} onClick={() => setAllRu((v) => !v)}>
            {allRu ? 'Скрыть переводы' : 'Все переводы'}
          </Button>
        )}
      </div>

      {revealed ? (
        <TextView sentences={text.sentences} hints={text.hints} selected={picked} onPick={(key) => setPicked(key)} playing={playing} showAllTranslations={allRu} />
      ) : (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <p className="text-muted-foreground">
            {canSpeak() ? 'Сначала послушай текст, потом открой его и проверь себя.' : 'В этом браузере нет синтеза речи: открой текст и читай.'}
          </p>
          <Button onClick={() => setRevealed(true)}>Показать текст</Button>
        </section>
      )}

      {revealed && picked && <WordPanel key={picked} token={picked} hint={hint} onClose={() => setPicked(null)} />}

      {revealed && (
        <details className="text-xs text-muted-foreground">
          <summary className="cursor-pointer">Источники предложений</summary>
          <ul className="mt-1">
            {text.sentences.map((s) => (
              <li key={s.tatoebaId}>
                <a href={`https://tatoeba.org/en/sentences/show/${s.tatoebaId}`} target="_blank" rel="noreferrer noopener" className="underline underline-offset-4">{`#${s.tatoebaId}`}</a>
                {` ${s.author}, Tatoeba, CC BY 2.0 FR`}
              </li>
            ))}
          </ul>
        </details>
      )}

      {revealed && (
        <section aria-label="Как понял" className="surface flex flex-col gap-3 p-4">
          {answer ? (
            <>
              <p role="status">{`Принято. Цель понимания теперь ${percent(answer.goal)}.`}</p>
              <div className="flex flex-wrap gap-3">
                <Button
                  disabled={loadingNext}
                  onClick={() => {
                    stop()
                    onNext()
                  }}
                >
                  Следующий текст
                </Button>
              </div>
            </>
          ) : (
            <>
              <h2 className="text-title">Как понял?</h2>
              <div className="flex flex-wrap gap-3">
                <Button variant="secondary" disabled={rate.isPending} onClick={() => rateIt('easy')}>
                  Почти всё
                </Button>
                <Button variant="secondary" disabled={rate.isPending} onClick={() => rateIt('ok')}>
                  Нормально
                </Button>
                <Button variant="secondary" disabled={rate.isPending} onClick={() => rateIt('hard')}>
                  Трудно
                </Button>
              </div>
            </>
          )}
          {rate.isError && (
            <p role="alert" className="text-sm text-coral-text">
              {rate.error.message}
            </p>
          )}
        </section>
      )}

      <p className="text-xs text-muted-foreground">Текст склеен из отдельных предложений Tatoeba, поэтому это набор связанных фраз, а не рассказ.</p>
    </>
  )
}

export function ReadPage() {
  const [params, setParams] = useSearchParams()
  const mode: ReadMode = params.get('mode') === 'listen' ? 'listen' : 'read'
  const next = useNextText(mode)

  useEffect(() => {
    document.title = 'Читать — Verst'
  }, [])

  const noText = next.error instanceof ApiError && next.error.code === 'no_text'

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-wrap items-baseline justify-between gap-3">
        <h1 className="text-title">{mode === 'listen' ? 'Слушать' : 'Читать'}</h1>
        <div role="group" aria-label="Режим" className="flex gap-2">
          {(['read', 'listen'] as const).map((m) => (
            <button
              key={m}
              type="button"
              aria-pressed={mode === m}
              onClick={() => setParams(m === 'read' ? {} : { mode: m })}
              className={`pixel-frame min-h-10 px-3 text-sm font-semibold ${mode === m ? 'bg-primary text-primary-foreground' : 'bg-card'}`}
            >
              {m === 'read' ? 'Читать' : 'Слушать'}
            </button>
          ))}
        </div>
      </header>

      {next.isPending && <p role="status">Собираем текст…</p>}

      {noText && (
        <section className="surface flex flex-col items-start gap-3 p-6">
          <FoxNote mood="thinking">
            <h2 className="text-title">Пока не из чего собрать текст</h2>
            <p className="text-muted-foreground">{next.error?.message}</p>
          </FoxNote>
          <div className="flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/learn">Знакомство</Link>
            </Button>
            <Button asChild variant="secondary">
              <Link to="/learn/placement">Быстрый тест</Link>
            </Button>
          </div>
        </section>
      )}

      {next.isError && !noText && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">{next.error.message}</p>
          <Button onClick={() => next.refetch()}>Повторить</Button>
        </div>
      )}

      {next.data && (
        <Reader
          key={next.data.id}
          mode={mode}
          loadingNext={next.isFetching}
          onNext={() => void next.refetch()}
        />
      )}
    </div>
  )
}
