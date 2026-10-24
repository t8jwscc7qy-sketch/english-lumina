import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { Wordmark } from '@/components/brand/Wordmark'
import { SettingsForm } from './SettingsForm'

/** Первый запуск: три шага настройки, потом «Сегодня». Оболочки с меню здесь нет — меню пока не нужно. */
export function OnboardingPage() {
  const navigate = useNavigate()

  useEffect(() => {
    document.title = 'Настройка — Verst'
  }, [])

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-4xl flex-col gap-8 p-6 xl:py-10">
      <Wordmark scale={4} className="h-auto max-w-full self-start" />
      <div className="flex flex-col gap-1">
        <h1 className="text-title">Настроим твой путь</h1>
        <p className="text-muted-foreground">Три коротких шага. Всё можно будет поменять в настройках.</p>
      </div>
      <SettingsForm initial={null} mode="onboarding" onSaved={() => navigate('/', { replace: true })} />
    </main>
  )
}
