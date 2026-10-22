import { expect, test, type Page } from '@playwright/test'

const ME = { login: 'firimon' }

async function mockApi(page: Page, opts: { loggedIn: boolean }) {
  let loggedIn = opts.loggedIn
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
}

test.describe('вход и оболочка', () => {
  test('неверный пароль, затем успешный вход ведёт на «Сегодня»', async ({ page }) => {
    await mockApi(page, { loggedIn: false })
    await page.goto('/')
    await expect(page).toHaveURL(/\/login$/)

    await page.getByLabel('Логин').fill('firimon')
    await page.getByLabel('Пароль').fill('wrong-password-1')
    await page.getByRole('button', { name: 'Войти' }).click()
    await expect(page.getByRole('alert')).toContainText('Неверный логин или пароль')

    await page.getByLabel('Пароль').fill('right-password-1')
    await page.getByRole('button', { name: 'Войти' }).click()
    await expect(page.getByRole('heading', { level: 1, name: 'Привет, firimon' })).toBeVisible()
  })

  test('выход возвращает на вход', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/')
    await page.getByRole('button', { name: 'Выйти' }).click()
    await expect(page).toHaveURL(/\/login$/)
  })

  test('навигация доступна с клавиатуры, фокус виден', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/')
    await page.getByRole('link', { name: 'Карточки' }).focus()
    await page.keyboard.press('Enter')
    await expect(page).toHaveURL(/\/cards$/)
    await expect(page.getByRole('heading', { name: 'Карточки' })).toBeVisible()
  })
})

// Снимки экранов для проверки вёрстки глазами: test-results/screens/*.png
const SIZES = [
  { name: '1440', width: 1440, height: 900 },
  { name: '1024', width: 1024, height: 768 },
  { name: '390', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const size of SIZES) {
    test(`снимки: ${theme} ${size.name}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: size.width, height: size.height },
        colorScheme: theme,
        reducedMotion: 'reduce',
      })
      const page = await context.newPage()

      await mockApi(page, { loggedIn: false })
      await page.goto('/login')
      await expect(page.getByRole('heading', { name: 'Вход' })).toBeVisible()
      await page.screenshot({ path: `test-results/screens/login-${theme}-${size.name}.png` })

      await page.unroute('**/api/v1/auth/**')
      await mockApi(page, { loggedIn: true })
      await page.goto('/')
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
      await page.screenshot({
        path: `test-results/screens/today-${theme}-${size.name}.png`,
        fullPage: true,
      })
      await context.close()
    })
  }
}
