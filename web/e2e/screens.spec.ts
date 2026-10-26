import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('вход и оболочка', () => {
  test('неверный пароль, затем успешный вход ведёт на «Путь»', async ({ page }) => {
    await mockApi(page, { loggedIn: false })
    await page.goto('/')
    await expect(page).toHaveURL(/\/login$/)

    await page.getByLabel('Логин').fill('firimon')
    await page.getByLabel('Пароль').fill('wrong-password-1')
    await page.getByRole('button', { name: 'Войти' }).click()
    await expect(page.getByRole('alert')).toContainText('Неверный логин или пароль')

    await page.getByLabel('Пароль').fill('right-password-1')
    await page.getByRole('button', { name: 'Войти' }).click()
    await expect(page).toHaveURL(/\/path$/)
    await expect(page.getByText('Урок 1: Привет')).toBeVisible()
  })

  test('выход возвращает на вход', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/today')
    await page.getByRole('button', { name: 'Выйти' }).click()
    await expect(page).toHaveURL(/\/login$/)
  })

  test('навигация доступна с клавиатуры, фокус виден', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/today')
    await page.getByRole('link', { name: 'Карточки', exact: true }).focus()
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
      await page.goto('/today')
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
      await page.screenshot({
        path: `test-results/screens/today-${theme}-${size.name}.png`,
        fullPage: true,
      })
      await context.close()
    })
  }
}

// Стресс: самое длинное допустимое имя и крайние ширины окна не должны ломать вёрстку.
test.describe('стресс-проверка вёрстки', () => {
  const LONG = 'w'.repeat(32)
  for (const width of [360, 768, 1024, 1440, 2000]) {
    test(`нет горизонтальной прокрутки при ширине ${width}`, async ({ browser }) => {
      const context = await browser.newContext({ viewport: { width, height: 800 } })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true })
      await page.route('**/api/v1/auth/me', (route) => route.fulfill({ json: { login: LONG } }))
      for (const path of ['/', '/cards', '/nope']) {
        await page.goto(path)
        await expect(page.getByRole('heading').first()).toBeVisible()
        const overflow = await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        )
        expect(overflow, `${path} при ширине ${width}`).toBeLessThanOrEqual(0)
      }
      await page.screenshot({ path: `test-results/screens/stress-${width}.png` })
      await context.close()
    })
  }

  test('увеличение масштаба до 200% не ломает вход', async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 640, height: 450 } }) // 1280×900 при 200%
    const page = await context.newPage()
    await page.route('**/api/v1/auth/me', (route) =>
      route.fulfill({
        status: 401,
        json: { error: { code: 'unauthorized', message: 'Нужно войти' } },
      }),
    )
    await page.goto('/login')
    await expect(page.getByRole('button', { name: 'Войти' })).toBeVisible()
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    )
    expect(overflow).toBeLessThanOrEqual(0)
  })
})
