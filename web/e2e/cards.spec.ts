import { expect, test, type Page } from '@playwright/test'
import { mockApi } from './mock-api'

const RUN_FRONT = 'I like to _____ every morning before work.'
const sentence = (page: Page, text: string) => page.locator('p', { hasText: text }).filter({ hasText: /^I like|^My|^The/ }).first()

test.describe('карточки слов', () => {
  test('слово → «В карточки» → повторение с клавиатуры → «На сегодня всё»', async ({ page }) => {
    const { reviews } = await mockApi(page, { loggedIn: true })
    await page.goto('/words')
    await page.getByRole('searchbox', { name: 'Слово' }).fill('run')
    const first = page.getByRole('listitem').first()
    await first.getByRole('button', { name: 'В карточки' }).click()
    await expect(first.getByRole('button', { name: /в карточках/i })).toBeDisabled()

    await page.getByRole('link', { name: 'Карточки', exact: true }).click()
    await expect(page).toHaveURL(/\/cards$/)
    await expect(page.getByText('Осталось: 1')).toBeVisible()
    await expect(sentence(page, RUN_FRONT)).toHaveText(RUN_FRONT)
    await expect(page.getByText('Я люблю бегать по утрам перед работой.')).toBeVisible()
    await expect(page.locator('mark')).toHaveCount(0)

    await page.keyboard.press('3') // до ответа оценка не принимается
    expect(reviews).toHaveLength(0)

    await page.keyboard.press('Space')
    await expect(page.locator('mark')).toHaveText('run')
    await expect(page.getByRole('link', { name: '#1234567' })).toHaveAttribute('href', 'https://tatoeba.org/en/sentences/show/1234567')
    await expect(page.getByRole('button', { name: /^Хорошо.*10 мин/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /^Легко.*4 дн/ })).toBeVisible()

    await page.keyboard.press('3')
    await expect(page.getByText('На сегодня всё')).toBeVisible()
    expect(reviews).toHaveLength(1)
    expect(reviews[0]).toMatchObject({ rating: 3 })
  })

  test('двойное нажатие оценки отправляет одну оценку', async ({ page }) => {
    const { reviews } = await mockApi(page, { loggedIn: true, dueCards: ['run'] })
    await page.goto('/cards')
    await expect(page.getByRole('button', { name: /показать ответ/i })).toBeVisible()
    await page.keyboard.press('Enter')
    await expect(page.locator('mark')).toHaveText('run')
    await page.keyboard.press('3')
    await page.keyboard.press('3')
    await expect(page.getByText('На сегодня всё')).toBeVisible()
    expect(reviews).toHaveLength(1)
  })

  test('несколько карточек идут по очереди, счётчик убывает', async ({ page }) => {
    await mockApi(page, { loggedIn: true, dueCards: ['run', 'runner', 'runway'] })
    await page.goto('/cards')
    await expect(page.getByText('Осталось: 3')).toBeVisible()
    await expect(page.getByRole('button', { name: /показать ответ/i })).toBeVisible()
    for (const [left, front] of [
      [2, 'My brother is a good _____.'],
      [1, 'The plane landed on the _____.'],
    ] as const) {
      await page.keyboard.press('Space')
      await page.keyboard.press('4')
      await expect(page.getByText(`Осталось: ${left}`)).toBeVisible()
      await expect(page.locator('p', { hasText: front }).first()).toBeVisible()
      await expect(page.getByRole('button', { name: /показать ответ/i })).toBeVisible()
    }
  })

  test('слово без примеров не добавляется: объяснение у кнопки', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/words')
    await page.getByRole('searchbox', { name: 'Слово' }).fill('zebra')
    const item = page.getByRole('listitem').first()
    await item.getByRole('button', { name: 'В карточки' }).click()
    await expect(item.getByRole('alert')).toContainText('нет подходящих примеров')
    await expect(item.getByRole('button', { name: 'В карточки' })).toBeEnabled()
  })

  test('«Сегодня» показывает число карточек к повторению и ведёт в раздел', async ({ page }) => {
    await mockApi(page, { loggedIn: true, dueCards: ['run', 'runner', 'runway'] })
    await page.goto('/today')
    const tile = page.getByRole('link', { name: /карточки к повторению/i })
    await expect(tile).toBeVisible({ timeout: 15_000 })
    await expect(tile).toContainText('3')
    await tile.click()
    await expect(page).toHaveURL(/\/cards$/)
  })
})

const SIZES = [
  { name: '1440', width: 1440, height: 900 },
  { name: '1024', width: 1024, height: 768 },
  { name: '390', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const size of SIZES) {
    test(`снимки карточек: ${theme} ${size.name}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: size.width, height: size.height },
        colorScheme: theme,
        reducedMotion: 'reduce',
      })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true, dueCards: ['run', 'runner'] })
      const shot = async (name: string) => {
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
        expect(overflow, name).toBeLessThanOrEqual(0)
        await page.screenshot({ path: `test-results/screens/cards-${name}-${theme}-${size.name}.png`, fullPage: true })
      }
      await page.goto('/cards')
      await expect(page.getByRole('button', { name: /показать ответ/i })).toBeVisible()
      await shot('front')
      await page.keyboard.press('Space')
      await expect(page.locator('mark')).toBeVisible()
      await shot('back')
      await page.keyboard.press('3')
      await expect(page.locator('p', { hasText: 'My brother is a good _____.' }).first()).toBeVisible()
      await page.keyboard.press('Space')
      await expect(page.locator('mark')).toBeVisible()
      await page.keyboard.press('3')
      await expect(page.getByText('На сегодня всё')).toBeVisible()
      await shot('empty')
      await context.close()
    })
  }
}
