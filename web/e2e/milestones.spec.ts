import { expect, test, type Page } from '@playwright/test'
import { mockApi } from './mock-api'

const exercise = (page: Page) => page.getByLabel('Упражнение')

test.describe('вехи', () => {
  test('«Прогресс»: достигнутые с датой и ближайшие с полосой', async ({ page }) => {
    await mockApi(page, {
      loggedIn: true,
      milestones: { reached: ['streak-3', 'hours-1', 'first-card'] },
    })
    await page.goto('/progress')
    await expect(page.getByRole('heading', { name: 'Вехи' })).toBeVisible()
    await expect(page.getByText('Серия 3 дня · 19 октября')).toBeVisible()
    await expect(page.getByText('1 час занятий · 19 октября')).toBeVisible()
    const next = page.getByRole('list', { name: 'Ближайшие вехи' })
    await expect(next.getByText('Серия 7 дней: 2 из 7')).toBeVisible()
    await expect(next.getByText('Первое упражнение: ещё впереди')).toBeVisible()
    await expect(next.getByRole('progressbar', { name: 'Серия 7 дней' })).toBeVisible()
  })

  test('«Прогресс»: ничего не достигнуто — подсказка', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/progress')
    await expect(page.getByText('Первая веха ждёт тебя после занятий.')).toBeVisible()
  })

  test('«Прогресс»: ошибка загрузки вех — «Повторить», остальная страница работает', async ({
    page,
  }) => {
    await mockApi(page, { loggedIn: true, milestones: { fail: true } })
    await page.goto('/progress')
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Темп' })).toBeVisible()
  })

  test('итог дня: вспышка при новой вехе, несколько вех списком', async ({ page }) => {
    await mockApi(page, {
      loggedIn: true,
      milestones: { newToday: ['hours-5', 'streak-7', 'first-exercise'] },
    })
    await page.goto('/session')
    await expect(exercise(page)).toBeVisible()
    await page.getByRole('button', { name: 'Закончить' }).click()
    await expect(page.getByRole('heading', { name: 'Итог дня' })).toBeVisible()
    const flash = page.getByRole('status').filter({ hasText: 'Новая веха' })
    await expect(flash).toBeVisible()
    const items = flash.getByRole('listitem')
    await expect(items).toHaveText(['Серия 7 дней', '5 часов занятий', 'Первое упражнение'])
  })

  test('итог дня без новых вех: вспышки нет', async ({ page }) => {
    await mockApi(page, { loggedIn: true, milestones: { reached: ['hours-1'] } })
    await page.goto('/session')
    await expect(exercise(page)).toBeVisible()
    await page.getByRole('button', { name: 'Закончить' }).click()
    await expect(page.getByRole('heading', { name: 'Итог дня' })).toBeVisible()
    await expect(page.getByText('Новая веха')).toHaveCount(0)
  })

  test('при «уменьшить движение» вспышка видна сразу, без анимации', async ({ browser }) => {
    const context = await browser.newContext({ reducedMotion: 'reduce' })
    const page = await context.newPage()
    await mockApi(page, { loggedIn: true, milestones: { newToday: ['hours-1'] } })
    await page.goto('/session')
    await page.getByRole('button', { name: 'Закончить' }).click()
    const flash = page.getByRole('status').filter({ hasText: 'Новая веха' })
    await expect(flash).toBeVisible()
    await expect(flash).toHaveCSS('opacity', '1')
    await context.close()
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [
      { name: '1440', width: 1440, height: 900 },
      { name: '1024', width: 1024, height: 768 },
      { name: '390', width: 390, height: 844 },
    ]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({
          viewport: { width: size.width, height: size.height },
          colorScheme: theme,
        })
        const page = await context.newPage()
        const shot = async (name: string) => {
          const overflow = await page.evaluate(
            () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
          )
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({
            path: `test-results/screens/milestones-${name}-${theme}-${size.name}.png`,
            fullPage: true,
          })
        }
        await mockApi(page, {
          loggedIn: true,
          overview: { activeDays: 20, minutesPerDay: 30 },
          milestones: {
            reached: [
              'streak-3',
              'streak-7',
              'hours-1',
              'hours-5',
              'answers-100',
              'first-card',
              'first-exercise',
            ],
          },
        })
        await page.goto('/progress')
        await expect(page.getByRole('heading', { name: 'Вехи' })).toBeVisible()
        await shot('progress')
        await page.unrouteAll({ behavior: 'ignoreErrors' })
        await mockApi(page, {
          loggedIn: true,
          milestones: { reached: ['streak-3', 'hours-1'], newToday: ['hours-5', 'streak-7'] },
        })
        await page.goto('/session')
        await expect(exercise(page)).toBeVisible()
        await page.getByRole('button', { name: 'Закончить' }).click()
        const flash = page.getByRole('status').filter({ hasText: 'Новая веха' })
        await expect(flash).toBeVisible()
        await page.waitForTimeout(800) // вспышка завершилась
        await shot('flash')
        await context.close()
      })
    }
  }
})
