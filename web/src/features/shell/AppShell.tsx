import { NavLink, Outlet, useNavigate } from 'react-router'
import { Post, Wordmark } from '@/components/brand/Wordmark'
import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { icons } from '@/components/pixel/icons'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useLogout, useMe } from '@/features/auth/auth'
import { ThemeToggle } from './ThemeToggle'

const NAV = [
  { to: '/', label: 'Сегодня', icon: icons.today, end: true },
  { to: '/cards', label: 'Карточки', icon: icons.cards },
  { to: '/exercises', label: 'Упражнения', icon: icons.exercises },
  { to: '/words', label: 'Слова', icon: icons.words },
  { to: '/mistakes', label: 'Ошибки', icon: icons.mistakes },
  { to: '/progress', label: 'Прогресс', icon: icons.progress },
] as const

/** Оболочка приложения: слева панель-указатель, справа рабочая область. На узких окнах панель сжимается до иконок. */
export function AppShell() {
  const me = useMe()
  const logout = useLogout()
  const navigate = useNavigate()

  async function onLogout() {
    try {
      await logout.mutateAsync()
    } catch {
      // сервер не ответил: локально мы всё равно вышли
    }
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex min-h-dvh">
      <a
        href="#content"
        className="pixel-box sr-only px-4 py-2 font-semibold focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50"
      >
        Перейти к содержимому
      </a>
      <aside className="w-20 shrink-0 bg-rail text-rail-foreground lg:w-60">
        <div className="sticky top-0 flex h-dvh flex-col gap-6 p-4">
          <div className="flex justify-center lg:justify-start">
            <Wordmark scale={3} className="hidden max-w-full lg:block" />
            <Post scale={3} className="lg:hidden" />
          </div>

          <nav aria-label="Разделы" className="flex flex-1 flex-col gap-2">
            {NAV.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={'end' in item ? item.end : false}
                className={({ isActive }) =>
                  cn(
                    'flex h-12 items-center gap-3 border-4 px-3 font-sans text-base font-semibold max-lg:justify-center max-lg:px-0',
                    isActive
                      ? 'border-line bg-sun text-ink shadow-pixel-sm'
                      : 'border-transparent text-rail-muted hover:border-rail-muted hover:text-rail-foreground',
                  )
                }
              >
                <PixelArtSvg art={item.icon} scale={3} />
                <span className="max-lg:sr-only">{item.label}</span>
              </NavLink>
            ))}
          </nav>

          <div className="flex flex-col gap-3 border-t-4 border-rail-muted/40 pt-4">
            <p className="truncate text-sm text-rail-muted max-lg:sr-only">
              Ты вошёл как{' '}
              <span className="font-semibold text-rail-foreground">{me.data?.login}</span>
            </p>
            <div className="flex items-center gap-2 max-lg:flex-col">
              <ThemeToggle className="text-rail-foreground hover:text-ink" />
              <Button
                variant="ghost"
                size="sm"
                onClick={onLogout}
                disabled={logout.isPending}
                className="text-rail-foreground hover:text-ink max-lg:size-10 max-lg:px-0"
                aria-label="Выйти"
              >
                <PixelArtSvg art={icons.exit} scale={3} />
                <span className="max-lg:sr-only">Выйти</span>
              </Button>
            </div>
          </div>
        </div>
      </aside>

      <main id="content" tabIndex={-1} className="min-w-0 flex-1 outline-none">
        <Outlet />
      </main>
    </div>
  )
}
