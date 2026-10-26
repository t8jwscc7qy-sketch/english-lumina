import { createBrowserRouter, type RouteObject } from 'react-router'
import { LoginPage } from '@/features/auth/LoginPage'
import { NotFoundPage } from '@/features/placeholder/ComingSoonPage'
import { OnboardingPage } from '@/features/settings/OnboardingPage'
import { SettingsPage } from '@/features/settings/SettingsPage'
import { SourcesPage } from '@/features/sources/SourcesPage'
import { WordsPage } from '@/features/words/WordsPage'
import { ProgressPage } from '@/features/progress/ProgressPage'
import { SessionPage } from '@/features/session/SessionPage'
import { ExercisesPage } from '@/features/exercises/ExercisesPage'
import { MistakesPage } from '@/features/mistakes/MistakesPage'
import { ReadPage } from '@/features/read/ReadPage'
import { SheetPage } from '@/features/sheets/SheetPage'
import { SheetsPage } from '@/features/sheets/SheetsPage'
import { CampPage } from '@/features/camp/CampPage'
import { LearnPage } from '@/features/learn/LearnPage'
import { PlacementPage } from '@/features/learn/PlacementPage'
import { CardsPage } from '@/features/cards/CardsPage'
import { AppShell } from '@/features/shell/AppShell'
import { TodayPage } from '@/features/today/TodayPage'
import { RedirectIfAuthed, RedirectIfOnboarded, RequireAuth, RequireOnboarded } from './guards'

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
      { path: 'learn', element: <LearnPage /> },
      { path: 'learn/placement', element: <PlacementPage /> },
      { path: 'read', element: <ReadPage /> },
      { path: 'sheets', element: <SheetsPage /> },
      { path: 'sheets/:id', element: <SheetPage /> },
      { path: 'camp', element: <CampPage /> },
      { path: 'exercises', element: <ExercisesPage /> },
      { path: 'session', element: <SessionPage /> },
      { path: 'words', element: <WordsPage /> },
      { path: 'mistakes', element: <MistakesPage /> },
      { path: 'progress', element: <ProgressPage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'sources', element: <SourcesPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]

export const router = createBrowserRouter(routes)
