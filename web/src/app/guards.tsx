import type { ReactNode } from 'react'
import { Navigate } from 'react-router'
import { Mascot } from '@/components/brand/Wordmark'
import { Button } from '@/components/ui/button'
import { useMe } from '@/features/auth/auth'
import { useSettings } from '@/features/settings/settings'

/** Пускает дальше только вошедшего; невошедшего отправляет на экран входа. */
export function RequireAuth({ children }: { children: ReactNode }) {
  const me = useMe()

  if (me.isPending) {
    return (
      <div role="status" className="grid min-h-dvh place-items-center">
        <div className="flex flex-col items-center gap-4">
          <Mascot scale={3} />
          <p className="font-semibold text-base">Проверяем вход…</p>
        </div>
      </div>
    )
  }

  if (me.isError) {
    return (
      <main className="grid min-h-dvh place-items-center p-6">
        <div className="surface flex max-w-md flex-col items-start gap-4 p-6">
          <Mascot scale={3} />
          <h1 className="text-title">Не получилось подключиться</h1>
          <p role="alert" className="text-muted-foreground">
            {me.error.message}
          </p>
          <Button onClick={() => me.refetch()}>Повторить</Button>
        </div>
      </main>
    )
  }

  if (me.data === null) return <Navigate to="/login" replace />
  return <>{children}</>
}

/** Экран входа для вошедшего не нужен: сразу ведём на главную. Сбой сервера форму не прячет. */
export function RedirectIfAuthed({ children }: { children: ReactNode }) {
  const me = useMe()
  if (me.data) return <Navigate to="/" replace />
  return <>{children}</>
}

/** Рабочие разделы открыты, только когда пройден онбординг. Сбой сервера в онбординг не ведёт: настройки могут быть. */
export function RequireOnboarded({ children }: { children: ReactNode }) {
  const settings = useSettings()

  if (settings.isPending) {
    return (
      <div role="status" className="grid min-h-dvh place-items-center">
        <div className="flex flex-col items-center gap-4">
          <Mascot scale={3} />
          <p className="font-semibold text-base">Проверяем настройки…</p>
        </div>
      </div>
    )
  }

  if (settings.isError) {
    return (
      <main className="grid min-h-dvh place-items-center p-6">
        <div className="surface flex max-w-md flex-col items-start gap-4 p-6">
          <Mascot scale={3} />
          <h1 className="text-title">Не получилось загрузить настройки</h1>
          <p role="alert" className="text-muted-foreground">
            {settings.error.message}
          </p>
          <Button onClick={() => settings.refetch()}>Повторить</Button>
        </div>
      </main>
    )
  }

  if (settings.data === null) return <Navigate to="/onboarding" replace />
  return <>{children}</>
}

/** Онбординг нужен только тому, у кого нет настроек. Сбой сервера форму не прячет: сохранение само сообщит об ошибке. */
export function RedirectIfOnboarded({ children }: { children: ReactNode }) {
  const settings = useSettings()
  if (settings.isPending) {
    return (
      <div role="status" className="grid min-h-dvh place-items-center">
        <p className="font-semibold text-base">Проверяем настройки…</p>
      </div>
    )
  }
  if (settings.data) return <Navigate to="/" replace />
  return <>{children}</>
}
