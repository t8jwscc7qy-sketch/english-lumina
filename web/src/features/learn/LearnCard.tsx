import { Link } from 'react-router'
import { FoxNote } from '@/components/brand/FoxNote'
import { Button } from '@/components/ui/button'
import { useLearnStatus } from './learn'

/** Карточка «Знакомство» на «Сегодня»: сколько новых слов сегодня и почему столько. Нет данных или база знаний не собрана — карточки нет. */
export function LearnCard() {
  const status = useLearnStatus()
  const s = status.data
  if (!s || !s.kbReady) return null
  const can = s.remaining > 0
  return (
    <section aria-labelledby="learn-title" className="surface p-6">
      <FoxNote mood={can ? 'neutral' : 'happy'}>
        <h2 id="learn-title" className="text-title">Знакомство</h2>
        <p className="tabular-nums">{`Новых слов сегодня: ${s.newToday} из ${s.limit}`}</p>
        <p className="text-sm text-muted-foreground">{s.reason.text}</p>
        <div className="mt-2 flex flex-wrap gap-3">
          {can && (
            <Button asChild>
              <Link to="/learn">Начать знакомство</Link>
            </Button>
          )}
          <Button asChild variant="secondary">
            <Link to="/learn/placement">Быстрый тест</Link>
          </Button>
          <Button asChild variant="secondary">
            <Link to="/read">Читать</Link>
          </Button>
          <Button asChild variant="secondary">
            <Link to="/camp">{s.campOpen ? 'Привал' : `Привал (${s.campWords} из ${s.campMinWords})`}</Link>
          </Button>
        </div>
      </FoxNote>
    </section>
  )
}
