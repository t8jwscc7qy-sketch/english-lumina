import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('поиск слов', () => {
  test('из меню: ввод находит слова по началу и показывает уровни', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/')
    await page.getByRole('link', { name: 'Слова' }).click()
    await expect(page).toHaveURL(/\/words$/)
    await expect(page.getByRole('searchbox', { name: 'Слово' })).toBeFocused()

    await page.keyboard.type('RUN')
    await expect(page.getByText('Найдено: 3')).toBeVisible()
    const items = page.getByRole('listitem')
    await expect(items).toHaveCount(3)
    await expect(items.first()).toContainText('run')
    await expect(items.first().getByLabel('Уровень A1')).toBeVisible()
  })

  test('слово с двумя оценками: уровень слова, обе оценки и заметки', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/words')
    await page.getByRole('searchbox', { name: 'Слово' }).fill('compl')
    const item = page.getByRole('listitem')
    await expect(item.getByLabel('Уровень B2')).toBeVisible()
    await expect(item.getByText('CEFR-J B2 · Octanove C1')).toBeVisible()
    await expect(item.getByText('сложность; запутанность')).toBeVisible()
  })

  test('нет результатов и очистка поля', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/words')
    const box = page.getByRole('searchbox', { name: 'Слово' })
    await box.fill('zzz')
    await expect(page.getByText(/ничего не найдено/i)).toBeVisible()
    await box.fill('')
    await expect(page.getByText(/введи начало слова/i)).toBeVisible()
  })
})

const SIZES = [
  { name: '1440', width: 1440, height: 900 },
  { name: '1024', width: 1024, height: 768 },
  { name: '390', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const size of SIZES) {
    test(`снимок слов: ${theme} ${size.name}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: size.width, height: size.height },
        colorScheme: theme,
        reducedMotion: 'reduce',
      })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true })
      await page.goto('/words')
      await page.screenshot({ path: `test-results/screens/words-empty-${theme}-${size.name}.png`, fullPage: true })
      await page.getByRole('searchbox', { name: 'Слово' }).fill('c')
      await expect(page.getByText('Найдено: 2')).toBeVisible()
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      expect(overflow).toBeLessThanOrEqual(0)
      await page.screenshot({ path: `test-results/screens/words-${theme}-${size.name}.png`, fullPage: true })
      await context.close()
    })
  }
}
