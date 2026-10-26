import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { ApiError, type LearnBlock } from '@/api/client'
import { FoxNote } from '@/components/brand/FoxNote'
import { Button } from '@/components/ui/button'
import { newKey, useIntroduce, useLearnBlock } from './learn'
import { WordCard } from './WordCard'

function Empty({ title, text }: { title: string; text: string }) {
  return (
    <section className="surface flex flex-col items-start gap-3 p-6">
      <FoxNote mood="happy">
        <h2 className="text-title">{title}</h2>
        <p className="text-muted-foreground">{text}</p>
      </FoxNote>
      <div className="flex flex-wrap gap-3">
        <Button asChild variant="secondary">
          <Link to="/cards">К карточкам</Link>
        </Button>
        <Button asChild variant="secondary">
          <Link to="/learn/placement">Быстрый тест</Link>
        </Button>
      </div>
    </section>
  )
}

function Steps({ block }: { block: LearnBlock }) {
  const navigate = useNavigate()
  const introduce = useIntroduce()
  const [index, setIndex] = useState(0)
  const [key] = useState(() => newKey())
  const last = index === block.words.length - 1
  const word = block.words[index]

  function finish() {
    if (introduce.isPending) return
    introduce.mutate({ ids: block.words.map((w) => w.lexemeId), key }, { onSuccess: () => navigate('/cards') })
  }

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.ctrlKey || e.metaKey || e.altKey) return
      if (e.target instanceof HTMLElement && (e.target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target.tagName))) return
      if (e.key === 'ArrowRight') setIndex((i) => Math.min(i + 1, block.words.length - 1))
      if (e.key === 'ArrowLeft') setIndex((i) => Math.max(i - 1, 0))
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [block.words.length])

  return (
    <>
      <p aria-live="polite" className="text-sm text-muted-foreground tabular-nums">
        {`Слово ${index + 1} из ${block.words.length}`}
      </p>
      <WordCard key={word.lexemeId} word={word} />
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="secondary" disabled={index === 0} onClick={() => setIndex(index - 1)} aria-keyshortcuts="ArrowLeft">
          Назад
        </Button>
        {last ? (
          <Button size="lg" disabled={introduce.isPending} onClick={finish}>
            {introduce.isPending ? 'Сохраняем…' : 'Начать повторять'}
          </Button>
        ) : (
          <Button size="lg" onClick={() => setIndex(index + 1)} aria-keyshortcuts="ArrowRight">
            Дальше
          </Button>
        )}
      </div>
      <p className="text-sm text-muted-foreground">{block.reason.text}</p>
      {introduce.isError && (
        <p role="alert" className="rounded-lg border border-coral bg-card p-3 text-sm font-semibold text-coral-text">
          {introduce.error.message}
        </p>
      )}
    </>
  )
}

export function LearnPage() {
  const block = useLearnBlock()

  useEffect(() => {
    document.title = 'Знакомство — Verst'
  }, [])

  const noWords = block.error instanceof ApiError && block.error.code === 'no_words'

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Знакомство</h1>
        <p className="text-muted-foreground">Сначала посмотри слова, потом они придут в карточки.</p>
      </header>

      {block.isPending && <p role="status">Подбираем слова…</p>}

      {noWords && <Empty title="Слов для знакомства пока нет" text="Когда появятся новые слова твоего уровня, они будут здесь." />}

      {block.isError && !noWords && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">{block.error.message}</p>
          <Button onClick={() => block.refetch()}>Повторить</Button>
        </div>
      )}

      {block.data && block.data.words.length === 0 && <Empty title="На сегодня всё" text={block.data.reason.text} />}

      {block.data && block.data.words.length > 0 && <Steps block={block.data} />}
    </div>
  )
}
