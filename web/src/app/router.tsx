import { createBrowserRouter, type RouteObject } from 'react-router'
import { LoginPage } from '@/features/auth/LoginPage'
import { ComingSoonPage, NotFoundPage } from '@/features/placeholder/ComingSoonPage'
import { OnboardingPage } from '@/features/settings/OnboardingPage'
import { SettingsPage } from '@/features/settings/SettingsPage'
import { SourcesPage } from '@/features/sources/SourcesPage'
import { AppShell } from '@/features/shell/AppShell'
import { TodayPage } from '@/features/today/TodayPage'
import { RedirectIfAuthed, RedirectIfOnboarded, RequireAuth, RequireOnboarded } from './guards'

const soon = (title: string, text: string) => <ComingSoonPage title={title} text={text} />

export const routes: RouteObject[] = [
  {
    path: '/login',
    element: (
      <RedirectIfAuthed>
        <LoginPage />
      </RedirectIfAuthed>
    ),
  },
  {
    path: '/onboarding',
    element: (
      <RequireAuth>
        <RedirectIfOnboarded>
          <OnboardingPage />
        </RedirectIfOnboarded>
      </RequireAuth>
    ),
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <RequireOnboarded>
          <AppShell />
        </RequireOnboarded>
      </RequireAuth>
    ),
    children: [
      { index: true, element: <TodayPage /> },
      { path: 'cards', element: soon('Карточки', 'Здесь будут слова для повторения.') },
      { path: 'exercises', element: soon('Упражнения', 'Здесь будут задания на слабые темы.') },
      { path: 'words', element: soon('Слова', 'Здесь будет поиск слов с примерами.') },
      { path: 'mistakes', element: soon('Ошибки', 'Здесь будет банк ошибок.') },
      { path: 'progress', element: soon('Прогресс', 'Здесь будет линия прогресса и прогноз.') },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'sources', element: <SourcesPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]

export const router = createBrowserRouter(routes)
