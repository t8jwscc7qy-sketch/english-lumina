import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('источники и лицензии', () => {
  test('из меню открывается страница с атрибуцией источника', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/today')
    await page.getByRole('link', { name: 'Источники', exact: true }).click()
    await expect(page).toHaveURL(/\/sources$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Источники и лицензии' })).toBeVisible()
    await expect(page.getByText('The CEFR-J Wordlist Version 1.5. Compiled by Yukio Tono, Tokyo University of Foreign Studies.')).toBeVisible()
    await expect(page.getByRole('link', { name: /raw\.githubusercontent\.com/ })).toHaveAttribute('target', '_blank')
  })

  test('без загруженных данных подсказывает команду', async ({ page }) => {
    await mockApi(page, { loggedIn: true, sources: [] })
    await page.goto('/sources')
    await expect(page.getByText(/go run \.\/cmd\/import cefrj/)).toBeVisible()
  })
})

const SIZES = [
  { name: '1440', width: 1440, height: 900 },
  { name: '1024', width: 1024, height: 768 },
  { name: '390', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const size of SIZES) {
    test(`снимок источников: ${theme} ${size.name}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: size.width, height: size.height },
        colorScheme: theme,
        reducedMotion: 'reduce',
      })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true })
      await page.goto('/sources')
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      expect(overflow).toBeLessThanOrEqual(0)
      await page.screenshot({ path: `test-results/screens/sources-${theme}-${size.name}.png`, fullPage: true })
      await context.close()
    })
  }
}
