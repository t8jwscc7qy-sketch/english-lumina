import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('Привал', () => {
  test('закрыт: счётчик, подсказки и закрытые упражнения', async ({ page }) => {
    await mockApi(page, { loggedIn: true, learn: { campWords: 12 } })
    await page.goto('/camp')
    await expect(page.getByRole('heading', { name: 'Привал откроется после 50 слов' })).toBeVisible()
    await expect(page.getByText('Изучено слов: 12 из 50')).toBeVisible()
    await page.goto('/exercises?mode=camp')
    await expect(page.getByRole('heading', { name: 'Привал пока закрыт' })).toBeVisible()
    await page.getByRole('link', { name: 'К Привалу' }).click()
    await expect(page).toHaveURL(/\/camp$/)
  })

  test('открыт: ссылка ведёт на упражнения режима и просит mode=camp', async ({ page }) => {
    const campRequests: number[] = []
    await mockApi(page, { loggedIn: true, learn: { campWords: 60, campRequests } })
    await page.goto('/camp')
    await expect(page.getByRole('heading', { name: 'Привал открыт' })).toBeVisible()
    await page.screenshot({ path: 'test-results/screens/camp-open.png' })
    await page.getByRole('link', { name: 'Упражнения Привала' }).click()
    await expect(page.getByRole('heading', { name: 'Упражнения Привала' })).toBeVisible()
    await expect(page.getByText('Только предложения из слов')).toBeVisible()
    expect(campRequests.length).toBeGreaterThan(0)
  })

  test('пункт меню «Привал» и ссылка на «Сегодня»', async ({ page }) => {
    await mockApi(page, { loggedIn: true, learn: { campWords: 12 } })
    await page.goto('/')
    await expect(page.getByRole('link', { name: 'Привал (12 из 50)' })).toBeVisible()
    await page.getByRole('navigation', { name: 'Разделы' }).getByRole('link', { name: 'Привал' }).click()
    await expect(page).toHaveURL(/\/camp$/)
  })

  for (const size of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    test(`вид ${size.width}px без горизонтальной прокрутки`, async ({ page }) => {
      await page.setViewportSize(size)
      await mockApi(page, { loggedIn: true, learn: { campWords: 12 } })
      await page.goto('/camp')
      await expect(page.getByRole('heading', { name: 'Привал откроется после 50 слов' })).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false)
      await page.screenshot({ path: `test-results/screens/camp-closed-${size.width}.png` })
    })
  }
})
