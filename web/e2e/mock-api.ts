import type { Page } from '@playwright/test'

export const ME = { login: 'firimon' }

export type Settings = {
  currentLevel: string
  targetLevel: string
  targetDate?: string | null
  dailyMinutes: number
  timezone: string
}

export const SETTINGS: Settings = {
  currentLevel: 'B1',
  targetLevel: 'B2',
  dailyMinutes: 15,
  timezone: 'Europe/Moscow',
}

/**
 * Подменяет API Go-сервера. `settings: null` — онбординг не пройден (GET /settings даёт 404);
 * PUT сохраняет присланное и дальше GET его возвращает. `puts` копит тела принятых PUT.
 */
export async function mockApi(
  page: Page,
  opts: { loggedIn: boolean; settings?: Settings | null; settingsStatus?: number },
) {
  let loggedIn = opts.loggedIn
  let settings: Settings | null = opts.settings === undefined ? SETTINGS : opts.settings
  const puts: Settings[] = []

  await page.route('**/api/v1/auth/**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/me')) {
      return loggedIn
        ? route.fulfill({ json: ME })
        : route.fulfill({
            status: 401,
            json: { error: { code: 'unauthorized', message: 'Нужно войти' } },
          })
    }
    if (url.pathname.endsWith('/login')) {
      const body = route.request().postDataJSON() as { password: string }
      if (body.password === 'right-password-1') {
        loggedIn = true
        return route.fulfill({ status: 204 })
      }
      return route.fulfill({
        status: 401,
        json: { error: { code: 'invalid_credentials', message: 'Неверный логин или пароль' } },
      })
    }
    if (url.pathname.endsWith('/logout')) {
      loggedIn = false
      return route.fulfill({ status: 204 })
    }
    return route.fulfill({ status: 404 })
  })

  await page.route('**/api/v1/settings', async (route) => {
    if (opts.settingsStatus) {
      return route.fulfill({
        status: opts.settingsStatus,
        json: { error: { code: 'internal', message: 'Внутренняя ошибка, попробуй ещё раз' } },
      })
    }
    if (route.request().method() === 'PUT') {
      settings = route.request().postDataJSON() as Settings
      puts.push(settings)
      return route.fulfill({ json: settings })
    }
    return settings
      ? route.fulfill({ json: settings })
      : route.fulfill({
          status: 404,
          json: { error: { code: 'onboarding_required', message: 'Сначала настрой цель и уровень' } },
        })
  })

  return { puts }
}
