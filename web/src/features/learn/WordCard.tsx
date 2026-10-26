import { useRef, useState } from 'react'
import type { LearnWord } from '@/api/client'
import { Button } from '@/components/ui/button'
import { HighlightedText } from '@/features/words/HighlightedText'
import { TatoebaCredit } from '@/features/words/TatoebaCredit'
import { canPlay, DEFAULT_VOLUME, sayText, speak as playItems } from '@/features/read/speech'

/** Произнести слово синтезом речи браузера: запасной путь, если записи нет или она не играет. */
function speak(text: string) {
  sayText(text)
}

function canSpeak(): boolean {
  return typeof window !== 'undefined' && 'speechSynthesis' in window
}

function Listen({ word }: { word: LearnWord }) {
  const audio = useRef<HTMLAudioElement | null>(null)
  const [failed, setFailed] = useState(false)
  if (!word.audioUrl && !canSpeak()) return null

  function play() {
    if (!word.audioUrl || failed) return speak(word.lemma)
    audio.current ??= new Audio(word.audioUrl)
    audio.current.volume = DEFAULT_VOLUME
    audio.current.currentTime = 0
    audio.current.play().catch(() => {
      setFailed(true)
      speak(word.lemma)
    })
  }

  return (
    <Button type="button" variant="secondary" size="sm" onClick={play} aria-label={`Прослушать слово ${word.lemma}`}>
      Прослушать
    </Button>
  )
}

/** Одно слово блока знакомства: слово, произношение, переводы, формы и примеры. Тексты из базы выводятся только текстом. */
export function WordCard({ word }: { word: LearnWord }) {
  const meta = [word.pos, word.level, word.rank ? `№${word.rank} по частоте` : null].filter(Boolean).join(' · ')
  return (
    <article aria-label={`Слово ${word.lemma}`} className="surface flex flex-col gap-5 p-6">
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <h2 lang="en" className="text-display break-words font-semibold">
            {word.lemma}
          </h2>
          <Listen key={word.lexemeId} word={word} />
        </div>
        <p className="text-sm text-muted-foreground">
          {meta}
          {word.ipa && (
            <>
              {' · '}
              <span lang="en-fonipa" className="whitespace-nowrap">{`/${word.ipa}/`}</span>
            </>
          )}
        </p>
      </header>

      <p lang="ru" className="text-xl font-semibold break-words">
        {word.translations.join(', ')}
      </p>

      {(word.forms.length > 0 || word.synonyms.length > 0) && (
        <dl className="flex flex-col gap-1 text-sm text-muted-foreground">
          {word.forms.length > 0 && (
            <div className="flex gap-2">
              <dt>Формы:</dt>
              <dd lang="en" className="break-words">{word.forms.join(', ')}</dd>
            </div>
          )}
          {word.synonyms.length > 0 && (
            <div className="flex gap-2">
              <dt>Близкие по смыслу:</dt>
              <dd lang="en" className="break-words">{word.synonyms.join(', ')}</dd>
            </div>
          )}
        </dl>
      )}

      {word.examples.length > 0 && (
        <ul aria-label="Слово во фразах" className="flex flex-col gap-4">
          {word.examples.map((ex, i) => (
            <li key={ex.en.tatoebaId} className="flex flex-col gap-1 border-l-2 border-input pl-4">
              <p lang="en" className={i === 0 ? 'text-xl break-words' : 'break-words'}>
                <HighlightedText text={ex.en.text} range={ex.highlight} />
              </p>
              {canPlay([{ text: ex.en.text, audioUrl: ex.en.audio?.url }]) && (
                <button
                  type="button"
                  onClick={() => playItems([{ text: ex.en.text, audioUrl: ex.en.audio?.url }], 1)}
                  aria-label={`Прослушать фразу: ${ex.en.text}`}
                  className="w-fit text-sm underline underline-offset-4"
                >
                  {ex.en.audio ? '▶ Запись' : '▶ Голос'}
                </button>
              )}
              {ex.en.audio && <span className="text-xs text-muted-foreground">{`озвучка: ${ex.en.audio.author}, ${ex.en.audio.license}`}</span>}
              <p lang="ru" className="break-words text-muted-foreground">{ex.ru.text}</p>
              <TatoebaCredit en={ex.en} ru={ex.ru} />
            </li>
          ))}
        </ul>
      )}
    </article>
  )
}
