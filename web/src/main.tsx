import { QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MotionConfig } from 'motion/react'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router/dom'
import { ApiError } from '@/api/client'
import { Toaster } from '@/components/ui/sonner'
import { meKey } from '@/features/auth/auth'
import { applyTheme, readStoredTheme } from '@/lib/theme'
import { router } from '@/app/router'
import './index.css'

// Тема применяется до первой отрисовки React, чтобы не мигать.
applyTheme(readStoredTheme())

const queryClient: QueryClient = new QueryClient({
  queryCache: new QueryCache({
    // Сессия истекла посреди работы: любой запрос с 401 возвращает на экран входа, а не оставляет белый экран.
    onError: (error) => {
      if (error instanceof ApiError && error.code === 'unauthorized') {
        queryClient.setQueryData(meKey, null)
      }
    },
  }),
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {/* «Уменьшить движение» в системе отключает анимации перемещения. */}
      <MotionConfig reducedMotion="user">
        <RouterProvider router={router} />
        <Toaster />
      </MotionConfig>
    </QueryClientProvider>
  </StrictMode>,
)
