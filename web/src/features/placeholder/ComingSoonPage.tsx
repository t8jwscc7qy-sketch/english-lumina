import { useEffect } from 'react'
import { Mascot } from '@/components/brand/Wordmark'

/** Заглушка раздела, которого ещё нет: честно говорит «скоро», без выдуманного содержимого. */
export function ComingSoonPage({ title, text }: { title: string; text: string }) {
  useEffect(() => {
    document.title = `${title} — Verst`
  }, [title])

  return (
    <div className="grid min-h-dvh place-items-center p-6">
      <div className="surface flex max-w-md flex-col items-start gap-4 p-6">
        <Mascot scale={4} />
        <h1 className="text-title">{title}</h1>
        <p className="text-muted-foreground">{text}</p>
        <p className="rounded-full bg-secondary px-3 py-1 text-sm font-semibold">Скоро</p>
      </div>
    </div>
  )
}

export function NotFoundPage() {
  useEffect(() => {
    document.title = 'Не найдено — Verst'
  }, [])

  return (
    <div className="grid min-h-dvh place-items-center p-6">
      <div className="surface flex max-w-md flex-col items-start gap-4 p-6">
        <Mascot scale={4} />
        <h1 className="text-title">Здесь ничего нет</h1>
        <p className="text-muted-foreground">Такой страницы нет. Вернись в раздел «Сегодня».</p>
      </div>
    </div>
  )
}
