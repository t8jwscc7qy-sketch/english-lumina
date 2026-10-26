import { useEffect } from 'react'
import { Link } from 'react-router'
import { FoxNote } from '@/components/brand/FoxNote'
import { XpBar } from '@/components/pixel/XpBar'
import { Button } from '@/components/ui/button'
import { useLearnPreferences, useLearnStatus } from '@/features/learn/learn'

/** «Привал»: упражнения только по изученным словам. Пока слов мало, экран говорит, сколько осталось. */
export function CampPage() {
  const status = useLearnStatus()
  const prefs = useLearnPreferences()
  const s = status.data

  useEffect(() => {
    document.title = 'Привал — Verst'
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Привал</h1>
        <p className="text-muted-foreground">Упражнения только из слов, которые ты уже изучил.</p>
      </header>

      {status.isPending && <p role="status">Загружаем…</p>}
      {status.isError && (
        <div className="surface flex flex-col items-start gap-4 p-6">
          <p role="alert" className="text-muted-foreground">{status.error.message}</p>
          <Button onClick={() => status.refetch()}>Повторить</Button>
        </div>
      )}

      {s && (
        <section aria-labelledby="camp-title" className="surface flex flex-col gap-4 p-6">
          <FoxNote mood={s.campOpen ? 'happy' : 'thinking'}>
            <h2 id="camp-title" className="text-title">
              {s.campOpen ? 'Привал открыт' : `Привал откроется после ${s.campMinWords} слов`}
            </h2>
            <p className="tabular-nums">{`Изучено слов: ${s.campWords}${s.campOpen ? '' : ` из ${s.campMinWords}`}`}</p>
            {prefs.data && (
              <p className="text-sm text-muted-foreground">
                {`В предложениях известно около ${Math.round(prefs.data.campCoverage * 100)}% слов, в длинных допускается одно незнакомое (порог меняется в настройках).`}
              </p>
            )}
          </FoxNote>
          {!s.campOpen && <XpBar value={Math.min(s.campWords, s.campMinWords)} total={s.campMinWords} label="Слов до Привала" segments={10} />}
          <div className="flex flex-wrap gap-3">
            {s.campOpen ? (
              <Button asChild size="lg">
                <Link to="/exercises?mode=camp">Упражнения Привала</Link>
              </Button>
            ) : (
              <>
                <Button asChild>
                  <Link to="/learn">Знакомство</Link>
                </Button>
                <Button asChild variant="secondary">
                  <Link to="/learn/placement">Быстрый тест</Link>
                </Button>
              </>
            )}
          </div>
        </section>
      )}
    </div>
  )
}
