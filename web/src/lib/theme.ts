import { useEffect, useSyncExternalStore } from 'react'

export type Theme = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'verst-theme'

export function resolveTheme(theme: Theme, systemDark: boolean): ResolvedTheme {
  if (theme === 'system') return systemDark ? 'dark' : 'light'
  return theme
}

export function readStoredTheme(): Theme {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    return value === 'light' || value === 'dark' || value === 'system' ? value : 'system'
  } catch {
    return 'system' // хранилище недоступно (приватный режим) — не ломаем интерфейс
  }
}

export function storeTheme(theme: Theme) {
  try {
    localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // не страшно: тема просто не запомнится
  }
}

function systemPrefersDark(): boolean {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false
}

/** Ставит класс dark на <html>; тема «system» следует настройке системы. */
export function applyTheme(theme: Theme): ResolvedTheme {
  const resolved = resolveTheme(theme, systemPrefersDark())
  document.documentElement.classList.toggle('dark', resolved === 'dark')
  return resolved
}

// Выбранная тема живёт в одном месте: все компоненты видят одно значение.
let currentTheme: Theme = readStoredTheme()
const listeners = new Set<() => void>()

function subscribeTheme(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function subscribeSystem(listener: () => void) {
  const query = window.matchMedia?.('(prefers-color-scheme: dark)')
  query?.addEventListener('change', listener)
  return () => query?.removeEventListener('change', listener)
}

export function setTheme(next: Theme) {
  storeTheme(next)
  currentTheme = next
  applyTheme(next)
  listeners.forEach((l) => l())
}

export function useTheme() {
  const theme = useSyncExternalStore(subscribeTheme, () => currentTheme)
  const systemDark = useSyncExternalStore(subscribeSystem, systemPrefersDark)
  const resolved = resolveTheme(theme, systemDark)

  // Класс на <html> — внешняя система, с которой синхронизируемся после отрисовки.
  useEffect(() => {
    applyTheme(theme)
  }, [theme, systemDark])

  return { theme, resolved, setTheme }
}
