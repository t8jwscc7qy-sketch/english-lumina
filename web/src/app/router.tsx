import { createBrowserRouter, type RouteObject } from 'react-router'
import { LoginPage } from '@/features/auth/LoginPage'
import { ComingSoonPage, NotFoundPage } from '@/features/placeholder/ComingSoonPage'
import { OnboardingPage } from '@/features/settings/OnboardingPage'
import { SettingsPage } from '@/features/settings/SettingsPage'
import { SourcesPage } from '@/features/sources/SourcesPage'
import { WordsPage } from '@/features/words/WordsPage'
import { ExercisesPage } from '@/features/exercises/ExercisesPage'
import { MistakesPage } from '@/features/mistakes/MistakesPage'
import { CardsPage } from '@/features/cards/CardsPage'
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
      { path: 'cards', element: <CardsPage /> },
      { path: 'exercises', element: <ExercisesPage /> },
      { path: 'words', element: <WordsPage /> },
      { path: 'mistakes', element: <MistakesPage /> },
      { path: 'progress', element: soon('Прогресс', 'Здесь будет линия прогресса и прогноз.') },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'sources', element: <SourcesPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]

export const router = createBrowserRouter(routes)
