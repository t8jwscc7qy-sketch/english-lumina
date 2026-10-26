import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('онбординг и настройки', () => {
  test('новый пользователь: вход → онбординг → «Сегодня» с выбранной целью', async ({ page }) => {
    const { puts } = await mockApi(page, { loggedIn: true, settings: null })
    await page.goto('/')
    await expect(page).toHaveURL(/\/onboarding$/)

    await page.getByRole('group', { name: 'Текущий уровень' }).getByRole('radio', { name: /^B1/ }).check({ force: true })
    await page.getByRole('button', { name: 'Дальше' }).click()
    await expect(page.getByRole('group', { name: 'Цель' }).getByRole('radio', { name: /^B2/ })).toBeChecked()
    await page.getByRole('button', { name: 'Дальше' }).click()

    await page.getByLabel(/своё число минут/i).fill('90')
    await expect(page.getByRole('status')).toContainText('6–8 месяцев')
    await page.getByRole('button', { name: 'Начать' }).click()

    await expect(page).toHaveURL(/\/path$/)
    await page.goto('/today')
    await expect(page.getByText(/Цель: B1 → B2/)).toBeVisible()
    await expect(page.getByText(/6–8 месяцев/)).toBeVisible()
    expect(puts).toHaveLength(1)
    expect(puts[0]).toMatchObject({ currentLevel: 'B1', targetLevel: 'B2', dailyMinutes: 90 })
  })

  test('с готовыми настройками онбординг недоступен', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/onboarding')
    await expect(page).toHaveURL(/\/path$/)
    await expect(page.getByText('Урок 1: Привет')).toBeVisible()
  })

  test('настройки: смена минут сохраняется и показывает уведомление', async ({ page }) => {
    const { puts } = await mockApi(page, { loggedIn: true })
    await page.goto('/settings')
    await expect(page.getByRole('heading', { level: 1, name: 'Настройки' })).toBeVisible()

    await page.getByLabel(/своё число минут/i).fill('30')
    await page.getByRole('button', { name: 'Сохранить' }).click()
    await expect(page.getByText('Настройки сохранены')).toBeVisible()
    expect(puts).toHaveLength(1)
    expect(puts[0].dailyMinutes).toBe(30)
  })

  test('сбой сервера при чтении настроек не отправляет в онбординг', async ({ page }) => {
    await mockApi(page, { loggedIn: true, settingsStatus: 500 })
    await page.goto('/')
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible()
    await expect(page).toHaveURL(/\/$/)
  })
})

const SIZES = [
  { name: '1440', width: 1440, height: 900 },
  { name: '1024', width: 1024, height: 768 },
  { name: '390', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const size of SIZES) {
    test(`снимки онбординга и настроек: ${theme} ${size.name}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: size.width, height: size.height },
        colorScheme: theme,
        reducedMotion: 'reduce',
      })
      const page = await context.newPage()
      const shot = (name: string) =>
        page.screenshot({ path: `test-results/screens/${name}-${theme}-${size.name}.png`, fullPage: true })

      await mockApi(page, { loggedIn: true, settings: null })
      await page.goto('/onboarding')
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
      await shot('onboarding-1')
      await page.getByRole('group', { name: 'Текущий уровень' }).getByRole('radio', { name: /^B1/ }).check({ force: true })
      await page.getByRole('button', { name: 'Дальше' }).click()
      await shot('onboarding-2')
      await page.getByRole('button', { name: 'Дальше' }).click()
      await page.getByLabel(/своё число минут/i).fill('90')
      await shot('onboarding-3')

      await page.unroute('**/api/v1/settings')
      await mockApi(page, { loggedIn: true })
      await page.goto('/settings')
      await expect(page.getByRole('heading', { level: 1, name: 'Настройки' })).toBeVisible()
      await shot('settings')
      await context.close()
    })
  }
}
