import { useEffect } from 'react'
import { SettingsForm } from './SettingsForm'
import { useSettings } from './settings'

export function SettingsPage() {
  const settings = useSettings()

  useEffect(() => {
    document.title = 'Настройки — Verst'
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-8 p-6 xl:p-8">
      <header className="flex flex-col gap-1">
        <h1 className="text-title">Настройки</h1>
        <p className="text-muted-foreground">Уровень, цель и темп. Прогноз пересчитывается сразу.</p>
      </header>
      {settings.data ? (
        <SettingsForm initial={settings.data} mode="settings" onSaved={() => {}} />
      ) : (
        <p role="status">Загружаем настройки…</p>
      )}
    </div>
  )
}
