import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

const UP = { direction: 'up', from: 'B1', to: 'B2', correct: 28, total: 30 } as const
const DOWN = { direction: 'down', from: 'B1', to: 'A2', correct: 10, total: 30 } as const

test.describe('предложение сменить уровень', () => {
  test('нет предложения — карточки на «Сегодня» нет', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/today')
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expect(page.getByRole('heading', { name: /Пора на уровень выше|закрепить/ })).toHaveCount(0)
  })

  test('«Пока нет» прячет карточку и пишет отказ', async ({ page }) => {
    const dismissed = { count: 0 }
    await mockApi(page, { loggedIn: true, level: { suggestion: UP, dismissed } })
    await page.goto('/today')
    await expect(page.getByRole('heading', { name: 'Пора на уровень выше?' })).toBeVisible()
    await page.getByRole('button', { name: 'Пока нет' }).click()
    await expect(page.getByRole('heading', { name: 'Пора на уровень выше?' })).toHaveCount(0)
    expect(dismissed.count).toBe(1)
  })

  test('«Перейти на B2» меняет уровень в настройках', async ({ page }) => {
    const { puts } = await mockApi(page, {
      loggedIn: true,
      settings: { currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 30, timezone: 'UTC' },
      level: { suggestion: UP },
    })
    await page.goto('/today')
    await page.getByRole('button', { name: 'Перейти на B2' }).click()
    await expect(page.getByRole('heading', { name: 'Пора на уровень выше?' })).toHaveCount(0)
    expect(puts.at(-1)).toMatchObject({ currentLevel: 'B2' })
    await expect(page.getByText(/Цель: B2 → B2/)).toBeVisible()
  })

  test('ошибка отказа: карточка остаётся с сообщением', async ({ page }) => {
    await mockApi(page, { loggedIn: true, level: { suggestion: DOWN, dismissFails: true } })
    await page.goto('/today')
    await page.getByRole('button', { name: 'Оставить B1' }).click()
    await expect(page.getByRole('alert')).toContainText('Внутренняя ошибка')
    await expect(page.getByRole('heading', { name: 'Может, закрепить A2?' })).toBeVisible()
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true, level: { suggestion: UP } })
        await page.goto('/today')
        await expect(page.getByRole('heading', { name: 'Пора на уровень выше?' })).toBeVisible()
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
        expect(overflow).toBeLessThanOrEqual(0)
        await page.screenshot({ path: `test-results/screens/level-up-${theme}-${size.name}.png`, fullPage: true })
        await context.close()
      })
    }
  }
})
