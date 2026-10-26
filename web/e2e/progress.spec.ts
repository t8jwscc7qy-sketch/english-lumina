import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('страница «Прогресс»', () => {
  test('из меню: пока занятий нет — «Собираю данные», счётчик и пустой график', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/today')
    await page.getByRole('link', { name: 'Прогресс', exact: true }).click()
    await expect(page).toHaveURL(/\/progress$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Прогресс' })).toBeVisible()
    await expect(page.getByText('Собираю данные', { exact: true })).toBeVisible()
    await expect(page.getByText('0 из 14 суток')).toBeVisible()
    await expect(page.getByText('Начни сессию, и здесь появится линия')).toBeVisible()
    await expect(page.getByText('Пока нет данных — ответь на несколько упражнений').or(page.getByRole('list', { name: 'Карта тем' }))).toBeVisible()
  })

  test('5 суток занятий: счётчик «5 из 14 суток», даты нет', async ({ page }) => {
    await mockApi(page, { loggedIn: true, overview: { activeDays: 5, minutesPerDay: 20 } })
    await page.goto('/progress')
    await expect(page.getByText('5 из 14 суток')).toBeVisible()
    await expect(page.getByText(/до b2 примерно/i)).toHaveCount(0)
    await expect(page.getByText('Темп покажу после 14 суток занятий')).toBeVisible()
    await expect(page.getByTestId('hours-line')).toBeVisible()
  })

  test('20 суток занятий: срок, даты, «что изменить», график и таблица по дням', async ({ page }) => {
    await mockApi(page, { loggedIn: true, overview: { activeDays: 20, minutesPerDay: 30 } })
    await page.goto('/progress')
    await expect(page.getByText(/До B2 примерно/)).toBeVisible()
    await expect(page.getByText(/^к \d{1,2} [а-я]+( \d{4})? – \d{1,2} [а-я]+( \d{4})?$/)).toBeVisible()
    await expect(page.getByText(/Что изменить: \+10 минут в день сократят срок примерно на \d+ /)).toBeVisible()
    await expect(page.getByText('Средний темп за 14 дней: 30 мин в день')).toBeVisible()
    await expect(page.getByTestId('hours-line')).toBeVisible()
    await expect(page.getByRole('progressbar', { name: 'Набрано часов' })).toBeVisible()
    await page.getByText('Данные по дням').click()
    await expect(page.getByRole('row')).toHaveCount(91)
  })

  test('наведение на график показывает подсказку', async ({ page }) => {
    await mockApi(page, { loggedIn: true, overview: { activeDays: 20, minutesPerDay: 30 } })
    await page.goto('/progress')
    const surface = page.getByTestId('hours-surface')
    const box = (await surface.boundingBox())!
    await page.mouse.move(box.x + box.width * 0.95, box.y + box.height / 2)
    await expect(page.getByRole('tooltip', { includeHidden: true })).toContainText('всего')
    await page.mouse.move(0, 0)
    await expect(page.getByRole('tooltip', { includeHidden: true })).toHaveCount(0)
  })

  test('ошибка загрузки: сообщение и «Повторить»', async ({ page }) => {
    await mockApi(page, { loggedIn: true, overview: { fail: true } })
    await page.goto('/progress')
    await expect(page.getByRole('alert')).toContainText('Внутренняя ошибка')
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible()
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/progress-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await mockApi(page, { loggedIn: true })
        await page.goto('/progress')
        await expect(page.getByText('Собираю данные', { exact: true })).toBeVisible()
        await shot('empty')
        await page.unrouteAll({ behavior: 'ignoreErrors' })
        await mockApi(page, { loggedIn: true, overview: { activeDays: 20, minutesPerDay: 30 } })
        await page.goto('/progress')
        await expect(page.getByText(/До B2 примерно/)).toBeVisible()
        await shot('estimate')
        await page.unrouteAll({ behavior: 'ignoreErrors' })
        await mockApi(page, { loggedIn: true, overview: { activeDays: 6, minutesPerDay: 15 } })
        await page.goto('/progress')
        await expect(page.getByText('6 из 14 суток')).toBeVisible()
        await shot('collecting')
        await context.close()
      })
    }
  }
})
